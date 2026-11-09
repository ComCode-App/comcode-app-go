package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
)

// Contact exchange (TEC-567): a visitor of a public profile page shares their own details back
// with the profile's owner. An exchange is a lead of kind "exchange"; it lands in the same inbox.

var (
	// ErrSenderProfileNotFound means the shared profile is not in the caller's active account.
	ErrSenderProfileNotFound = errors.New("profile not found")
	// ErrSenderProfileForbidden means a plain member tried to share someone else's profile.
	ErrSenderProfileForbidden = errors.New("profile cannot be shared by this account role")
	// ErrSelfExchange means the visitor's active account owns the profile page.
	ErrSelfExchange = errors.New("cannot exchange contact details with your own account")
	// ErrImpersonating means a platform admin acting as the user tried to share their data.
	ErrImpersonating = errors.New("contact exchange is not available while impersonating")
)

// viaRE is what cards and smart links append as ?via= (frontend lib/via.ts).
var viaRE = regexp.MustCompile(`^[cs]/[A-Za-z0-9._-]{1,120}$`)

// ValidateExchange normalises a visitor's exchange: a name, at least one of email / phone,
// optional company and title, and an explicit consent on a known exchange consent text.
func ValidateExchange(in model.ExchangeInput) (model.ExchangeInput, error) {
	out := model.ExchangeInput{
		Path:           normalizePath(in.Path),
		Via:            normalizeVia(in.Via),
		Name:           collapseSpaces(in.Name),
		Email:          strings.ToLower(strings.TrimSpace(in.Email)),
		Phone:          strings.TrimSpace(in.Phone),
		Company:        collapseSpaces(in.Company),
		JobTitle:       collapseSpaces(in.JobTitle),
		Locale:         normalizeLocale(in.Locale),
		Consent:        in.Consent,
		ConsentVersion: strings.TrimSpace(in.ConsentVersion),
	}
	var errs []FieldError
	add := func(field, code string) { errs = append(errs, FieldError{Field: field, Code: code}) }

	if out.Path == "" {
		add("path", "required")
	}
	switch {
	case out.Name == "":
		add("name", "required")
	case utf8.RuneCountInString(out.Name) > model.MaxName:
		add("name", "too_long")
	case hasControl(out.Name):
		add("name", "invalid")
	}
	if out.Email == "" && out.Phone == "" {
		add("email", "email_or_phone_required")
	}
	if out.Email != "" && !validEmail(out.Email) {
		add("email", "invalid")
	}
	if out.Phone != "" && !validPhone(out.Phone) {
		add("phone", "invalid")
	}
	for _, f := range []struct {
		field, value string
		max          int
	}{{"company", out.Company, model.MaxCompany}, {"job_title", out.JobTitle, model.MaxTitle}} {
		switch {
		case utf8.RuneCountInString(f.value) > f.max:
			add(f.field, "too_long")
		case hasControl(f.value):
			add(f.field, "invalid")
		}
	}
	if !out.Consent {
		add("consent", "required")
	}
	if !slices.Contains(model.ExchangeConsentVersions, out.ConsentVersion) {
		add("consent_version", "invalid")
	}
	if len(errs) > 0 {
		return out, &ValidationError{Fields: errs}
	}
	return out, nil
}

// Exchange stores the details a visitor typed in for the owner of the profile page.
func (s *Service) Exchange(ctx context.Context, in model.ExchangeInput, visitor model.Visitor) (model.SubmitResult, error) {
	in, err := ValidateExchange(in)
	if err != nil {
		return model.SubmitResult{}, err
	}
	return s.storeExchange(ctx, in, nil, in.Email, visitor)
}

// ShareProfile stores one of the caller's own profiles as an exchange with the owner of the
// profile page. The details are a snapshot of that profile. A plain member may share only a
// profile they own; owners and admins may share any profile of the account.
func (s *Service) ShareProfile(ctx context.Context, p authctx.Principal, in model.ShareProfileInput, visitor model.Visitor) (model.SubmitResult, error) {
	if p.ImpersonatorUserID != nil {
		return model.SubmitResult{}, ErrImpersonating
	}
	sender, err := s.repo.SenderProfile(ctx, p.Account.ID, in.ProfileUUID)
	if err != nil {
		return model.SubmitResult{}, err
	}
	if sender == nil {
		return model.SubmitResult{}, ErrSenderProfileNotFound
	}
	ownsProfile := sender.UserID != nil && *sender.UserID == p.UserInternal
	if !ownsProfile && !p.Account.CanWrite() {
		return model.SubmitResult{}, ErrSenderProfileForbidden
	}
	ex, err := ValidateExchange(model.ExchangeInput{
		Path: in.Path, Via: in.Via,
		Name:    sender.FirstName + " " + sender.LastName,
		Email:   sender.Email,
		Phone:   sender.Phone,
		Company: clip(sender.Company, model.MaxCompany), JobTitle: clip(sender.Title, model.MaxTitle),
		Locale: in.Locale, Consent: in.Consent, ConsentVersion: in.ConsentVersion,
	})
	if err != nil {
		return model.SubmitResult{}, err
	}
	// The copy goes to the signed-in visitor, not to the shared profile's contact address (an
	// admin may share a colleague's profile).
	return s.storeExchange(ctx, ex, &senderRef{accountID: p.Account.ID, userID: p.UserInternal, profileID: sender.ID}, p.Email, visitor)
}

type senderRef struct {
	accountID int64
	userID    int64
	profileID int64
}

func (s *Service) storeExchange(ctx context.Context, in model.ExchangeInput, sender *senderRef, copyTo string, visitor model.Visitor) (model.SubmitResult, error) {
	route, err := s.repo.CaptureRoute(ctx, in.Path)
	if err != nil {
		return model.SubmitResult{}, err
	}
	if !exchangeEnabled(route) {
		return model.SubmitResult{}, ErrFormUnavailable
	}
	if sender != nil && sender.accountID == route.AccountID {
		return model.SubmitResult{}, ErrSelfExchange
	}
	locked, err := s.record(ctx, route.AccountID, model.KindExchange)
	if err != nil {
		return model.SubmitResult{}, err
	}
	lead := repository.NewLead{
		AccountID: route.AccountID, RouteID: route.ID, ProfileID: route.ProfileID,
		SourceType: model.SourceProfile, RoutePath: in.Path, Kind: model.KindExchange,
		Name: in.Name, Email: in.Email, Phone: in.Phone, Company: in.Company, JobTitle: in.JobTitle,
		Locale: in.Locale, ConsentAt: s.now().UTC(), ConsentVersion: in.ConsentVersion,
		IP: visitor.IP, UserAgent: truncate(visitor.UserAgent, 512), Locked: locked,
	}
	s.attributeVia(ctx, &lead, in.Via, route)
	if sender != nil {
		lead.SenderUserID, lead.SenderProfileID = &sender.userID, &sender.profileID
	}
	res, err := s.repo.Create(ctx, lead)
	if err != nil {
		return model.SubmitResult{}, err
	}
	s.notify(ctx, route.AccountID, res.UUID, "leads.exchange_received", in.Name, lead.RoutePath, locked)
	s.emitLeadCreated(ctx, res, lead)
	s.sendVisitorCopy(ctx, res.UUID, in, route.ProfileID, copyTo)
	return res, nil
}

// attributeVia moves a lead of a profile route to the card or smart link the visitor came
// through, when that route is live and opens the same profile. The card's own form switch does
// not matter: cards always append ?via= (TEC-940).
func (s *Service) attributeVia(ctx context.Context, lead *repository.NewLead, via string, profileRoute *repository.CaptureRoute) {
	r := s.viaRoute(ctx, via, profileRoute)
	if r == nil {
		return
	}
	lead.RouteID, lead.CardID, lead.RoutePath = r.ID, r.CardID, via
	lead.SourceType = model.SourceSmartLink
	if r.RouteType == "card_route" {
		lead.SourceType = model.SourceCard
	}
}

// viaRoute returns the card or smart link the visitor came through when it is live and opens the
// same profile of the same account; nil otherwise (the exchange is then the profile's own).
func (s *Service) viaRoute(ctx context.Context, via string, profileRoute *repository.CaptureRoute) *repository.CaptureRoute {
	if via == "" || profileRoute.RouteType != "canonical_profile" || profileRoute.ProfileID == nil {
		return nil
	}
	route, err := s.repo.CaptureRoute(ctx, via)
	if err != nil {
		s.log.Warn("exchange_via_lookup_failed", "error", err)
		return nil
	}
	if route == nil || route.Status != "active" || route.AccountID != profileRoute.AccountID ||
		route.ProfileID == nil || *route.ProfileID != *profileRoute.ProfileID {
		return nil
	}
	switch route.RouteType {
	case "card_route":
		if route.CardID == nil || !route.CardIsActive || route.CardIsLocked {
			return nil
		}
		return route
	case "custom_smart_link":
		return route
	default:
		return nil
	}
}

type exchangeConfig struct {
	ContactExchange struct {
		Enabled bool `json:"enabled"`
	} `json:"contact_exchange"`
}

// exchangeEnabled reports whether a profile page offers contact exchange: the route is a live
// canonical profile route with presentation_config.contact_exchange.enabled.
func exchangeEnabled(route *repository.CaptureRoute) bool {
	if route == nil || route.Status != "active" || route.RouteType != "canonical_profile" {
		return false
	}
	var cfg exchangeConfig
	if len(route.PresentationConfig) > 0 {
		_ = json.Unmarshal(route.PresentationConfig, &cfg)
	}
	return cfg.ContactExchange.Enabled
}

// clip shortens a profile field to the column limit (profile fields allow more than leads).
func clip(s string, n int) string {
	s = collapseSpaces(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}
