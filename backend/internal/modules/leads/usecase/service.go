// Package usecase implements lead capture (TEC-552): the visitor form behind public profiles,
// cards and smart links, contact exchange on profile pages (TEC-567), and the account's lead inbox.
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/repository"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

var (
	// ErrNotFound means the lead is not in the active account.
	ErrNotFound = errors.New("lead not found")
	// ErrForbidden means the account role cannot change leads (owner/admin only).
	ErrForbidden = errors.New("account role cannot modify leads")
	// ErrFormUnavailable means the path is unknown or its lead form is off. Visitors get 404.
	ErrFormUnavailable = errors.New("lead form is not available")
)

// FieldError is one invalid field of a visitor submission.
type FieldError struct {
	Field string
	Code  string
}

// ValidationError lists every invalid field of a submission.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, f.Field+": "+f.Code)
	}
	return "invalid lead: " + strings.Join(parts, ", ")
}

// Notifier enqueues notifications (notifications.Service).
type Notifier interface {
	Enqueue(ctx context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error)
}

// Service is the leads use case.
// Webhooks fans a domain event out to the account's webhook endpoints (TEC-603).
type Webhooks interface {
	Dispatch(ctx context.Context, accountID int64, eventName string, data map[string]any)
}

type Service struct {
	repo     repository.Repository
	notifier Notifier
	quotas   Quotas
	webhooks Webhooks
	log      *slog.Logger
	now      func() time.Time
	// Visitor copy email of an exchange (TEC-940); off until WithVisitorCopy.
	copyKey     []byte
	frontendURL string
}

// New creates the service. notifier may be nil (no owner notification).
func New(repo repository.Repository, notifier Notifier, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, notifier: notifier, log: log, now: time.Now}
}

// Validate normalises a visitor submission: a name, at least one of email / phone, an optional
// message, and an explicit KVKK consent on a known consent text version.
func Validate(in model.SubmitInput) (model.SubmitInput, error) {
	out := model.SubmitInput{
		Path:           normalizePath(in.Path),
		Via:            normalizeVia(in.Via),
		Name:           collapseSpaces(in.Name),
		Email:          strings.ToLower(strings.TrimSpace(in.Email)),
		Phone:          strings.TrimSpace(in.Phone),
		Message:        strings.TrimSpace(in.Message),
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
	if utf8.RuneCountInString(out.Message) > model.MaxMessage {
		add("message", "too_long")
	}
	if !out.Consent {
		add("consent", "required")
	}
	if !slices.Contains(model.ConsentVersions, out.ConsentVersion) {
		add("consent_version", "invalid")
	}
	if len(errs) > 0 {
		return out, &ValidationError{Fields: errs}
	}
	return out, nil
}

// FormConfig tells a visitor page whether the lead form is on for a public path. A plan whose
// monthly quota for the form (or the exchange) is 0 turns it off.
func (s *Service) FormConfig(ctx context.Context, path string) (model.FormConfig, error) {
	path = normalizePath(path)
	out := model.FormConfig{
		Path: path, ConsentVersion: model.ConsentVersion, ExchangeConsentVersion: model.ExchangeConsentVersion,
	}
	if path == "" {
		return out, nil
	}
	route, err := s.repo.CaptureRoute(ctx, path)
	if err != nil {
		return out, err
	}
	if source, ok := captureSource(route); ok {
		if out.Enabled, err = s.quotaOn(ctx, route.AccountID, model.KindForm); err != nil {
			return out, err
		}
		if out.Enabled {
			out.SourceType = source
			out.ProfileUUID = route.ProfileUUID
		}
	}
	if exchangeEnabled(route) {
		if out.ExchangeEnabled, err = s.quotaOn(ctx, route.AccountID, model.KindExchange); err != nil {
			return out, err
		}
	}
	if out.ExchangeEnabled {
		out.ProfileUUID = route.ProfileUUID
	}
	return out, nil
}

// Submit stores a visitor's lead for the account that owns the route and notifies its owners
// and admins. The route must be live with its lead form on. Over the monthly quota the lead is
// stored locked (TEC-941).
func (s *Service) Submit(ctx context.Context, in model.SubmitInput, visitor model.Visitor) (model.SubmitResult, error) {
	in, err := Validate(in)
	if err != nil {
		return model.SubmitResult{}, err
	}
	route, err := s.repo.CaptureRoute(ctx, in.Path)
	if err != nil {
		return model.SubmitResult{}, err
	}
	source, ok := captureSource(route)
	if !ok {
		return model.SubmitResult{}, ErrFormUnavailable
	}
	locked, err := s.record(ctx, route.AccountID, model.KindForm)
	if err != nil {
		return model.SubmitResult{}, err
	}
	lead := repository.NewLead{
		AccountID: route.AccountID, RouteID: route.ID, ProfileID: route.ProfileID, CardID: route.CardID,
		SourceType: source, RoutePath: in.Path, Kind: model.KindForm,
		Name: in.Name, Email: in.Email, Phone: in.Phone, Message: in.Message, Locale: in.Locale,
		ConsentAt: s.now().UTC(), ConsentVersion: in.ConsentVersion,
		IP: visitor.IP, UserAgent: truncate(visitor.UserAgent, 512), Locked: locked,
	}
	if source == model.SourceProfile {
		// The profile's own form, opened through a card or smart link whose form is off (TEC-940).
		s.attributeVia(ctx, &lead, in.Via, route)
	}
	res, err := s.repo.Create(ctx, lead)
	if err != nil {
		return model.SubmitResult{}, err
	}
	s.notify(ctx, route.AccountID, res.UUID, "leads.received", in.Name, lead.RoutePath, locked)
	s.emitLeadCreated(ctx, res, lead)
	return res, nil
}

// SetWebhooks enables lead.created webhooks (TEC-603).
func (s *Service) SetWebhooks(w Webhooks) *Service {
	s.webhooks = w
	return s
}

// emitLeadCreated sends lead.created. A locked lead (over quota, TEC-941) carries no contact
// details: the webhook must not leak what the panel masks.
func (s *Service) emitLeadCreated(ctx context.Context, res model.SubmitResult, lead repository.NewLead) {
	if s.webhooks == nil {
		return
	}
	data := map[string]any{
		"lead_uuid": res.UUID.String(), "kind": lead.Kind, "source_type": lead.SourceType,
		"route_path": "/" + lead.RoutePath, "locale": lead.Locale, "locked": lead.Locked,
		"created_at": res.CreatedAt.UTC().Format(time.RFC3339),
	}
	if !lead.Locked {
		data["name"], data["email"], data["phone"] = lead.Name, lead.Email, lead.Phone
		data["message"], data["company"], data["job_title"] = lead.Message, lead.Company, lead.JobTitle
	}
	s.webhooks.Dispatch(ctx, lead.AccountID, "lead.created", data)
}

// notify tells the account's owners and admins about a new lead. It is fail-soft: the lead is
// already stored. A locked lead's name is masked here too.
func (s *Service) notify(ctx context.Context, accountID int64, leadUUID uuid.UUID, template, name, path string, locked bool) {
	if s.notifier == nil {
		return
	}
	if locked {
		name = model.MaskName(name)
	}
	ids, err := s.repo.NotifyUserIDs(ctx, accountID)
	if err != nil {
		s.log.Warn("lead_notify_recipients_failed", "error", err)
		return
	}
	action := "/leads?lead=" + leadUUID.String()
	for _, id := range ids {
		uid := id
		if _, err := s.notifier.Enqueue(ctx, notifmodel.EnqueueInput{
			UserID:       &uid,
			Channels:     []string{notifmodel.ChannelInapp, notifmodel.ChannelRealtime},
			TemplateCode: template,
			TemplateVars: map[string]string{"name": name, "source": "/" + path},
			ActionURL:    &action,
			Payload:      map[string]any{"lead_uuid": leadUUID.String(), "locked": locked},
			SourceEvent:  template,
		}); err != nil {
			s.log.Warn("lead_notify_failed", "error", err)
		}
	}
}

// List returns the leads the caller may read in the active account, newest first: all of them
// for owners and admins, otherwise those of the caller's own profile or card (TEC-940). Locked
// leads come masked (TEC-941).
func (s *Service) List(ctx context.Context, p authctx.Principal, f model.ListFilter) ([]model.Lead, int64, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	s.unlock(ctx, p.Account.ID)
	items, total, err := s.repo.List(ctx, scopeOf(p), f)
	for i := range items {
		if items[i].Locked {
			items[i].Mask()
		}
	}
	return items, total, err
}

// Get returns one lead the caller may read; ErrNotFound otherwise (no existence oracle).
func (s *Service) Get(ctx context.Context, p authctx.Principal, id uuid.UUID) (*model.Lead, error) {
	s.unlock(ctx, p.Account.ID)
	l, err := s.repo.Get(ctx, scopeOf(p), id)
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, ErrNotFound
	}
	if l.Locked {
		l.Mask()
	}
	return l, nil
}

// MarkRead marks a lead as read (idempotent). Needs an owner/admin role.
func (s *Service) MarkRead(ctx context.Context, p authctx.Principal, id uuid.UUID) (*model.Lead, error) {
	if !p.Account.CanWrite() {
		return nil, ErrForbidden
	}
	ok, err := s.repo.MarkRead(ctx, p.Account.ID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return s.Get(ctx, p, id)
}

// Delete removes a lead for good (it is personal data). Needs an owner/admin role.
func (s *Service) Delete(ctx context.Context, p authctx.Principal, id uuid.UUID) error {
	if !p.Account.CanWrite() {
		return ErrForbidden
	}
	ok, err := s.repo.Delete(ctx, p.Account.ID, id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func scopeOf(p authctx.Principal) repository.Scope {
	return repository.Scope{AccountID: p.Account.ID, ViewerUserID: p.UserInternal}
}

type leadCaptureConfig struct {
	LeadCapture struct {
		Enabled bool `json:"enabled"`
	} `json:"lead_capture"`
}

// captureSource reports the lead source of a route when its form is on: the route is active,
// a card route's card is active and unlocked, and lead capture is enabled in the route's
// presentation_config (cards: also the card's own lead_capture_enabled switch).
func captureSource(route *repository.CaptureRoute) (string, bool) {
	if route == nil || route.Status != "active" {
		return "", false
	}
	var cfg leadCaptureConfig
	if len(route.PresentationConfig) > 0 {
		_ = json.Unmarshal(route.PresentationConfig, &cfg)
	}
	enabled := cfg.LeadCapture.Enabled
	switch route.RouteType {
	case "canonical_profile":
		return model.SourceProfile, enabled
	case "custom_smart_link":
		return model.SourceSmartLink, enabled
	case "card_route":
		if route.CardID == nil || !route.CardIsActive || route.CardIsLocked {
			return "", false
		}
		return model.SourceCard, enabled || route.CardLeadCapture
	default:
		return "", false
	}
}

func normalizePath(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if len(p) > 128 {
		return ""
	}
	return p
}

// normalizeVia keeps a well-formed card or smart link path; attribution is best effort, so a
// mangled via is dropped instead of failing the submission.
func normalizeVia(v string) string {
	if v = normalizePath(v); viaRE.MatchString(v) {
		return v
	}
	return ""
}

func normalizeLocale(l string) string {
	switch l = strings.ToLower(strings.TrimSpace(l)); l {
	case "tr", "en":
		return l
	default:
		return ""
	}
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func hasControl(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) >= 0
}

func validEmail(s string) bool {
	if len(s) > model.MaxEmail {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	at := strings.LastIndexByte(s, '@')
	return at > 0 && strings.Contains(s[at+1:], ".")
}

// validPhone accepts digits with optional leading +, spaces, dots, dashes and parentheses, and
// 7 to 15 digits (E.164 maximum).
func validPhone(s string) bool {
	if len(s) > model.MaxPhone {
		return false
	}
	digits := 0
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' && i == 0:
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return digits >= 7 && digits <= 15
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) && len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s
}
