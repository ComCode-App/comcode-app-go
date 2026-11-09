package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"slices"

	bgmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	routesmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/publicpath"
	"github.com/google/uuid"
)

var (
	// ErrUsernameTaken means another profile, card or smart link holds the path.
	ErrUsernameTaken = errors.New("username is already taken")
	// ErrPublicPagesUnavailable means the service was wired without a PublicPageStore.
	ErrPublicPagesUnavailable = errors.New("public pages are not configured")
)

// SetPublicPages wires username / public page persistence.
func (s *Service) SetPublicPages(store repository.PublicPageStore) { s.pages = store }

// SetRouteCache lets profile and username changes drop resolver cache entries (TEC-557).
func (s *Service) SetRouteCache(c *routecache.Cache) { s.routeCache = c }

// SetActivity records username changes in the activity log (TEC-558).
func (s *Service) SetActivity(r *activity.Recorder) { s.activity = r }

// BackgroundCatalog resolves Background Studio designs (backgrounds module).
type BackgroundCatalog interface {
	// Selectable reports whether the account may pick the design, and whether it is premium.
	Selectable(ctx context.Context, accountID int64, id uuid.UUID) (usable, premium bool, err error)
	PublicBackground(ctx context.Context, accountID int64, id uuid.UUID) (*bgmodel.PublicBackground, error)
}

// SetBackgrounds wires background selection for public pages.
func (s *Service) SetBackgrounds(c BackgroundCatalog) { s.backgrounds = c }

// LinkPresentationSource reads how a smart link presents the profile it targets (TEC-556).
type LinkPresentationSource interface {
	SmartLinkPresentation(ctx context.Context, via string) (*routesmodel.LinkPresentation, error)
}

// SetLinkPresentations lets a profile opened through a smart link (?via=s/{slug}) use the
// link's theme, modules and primary action.
func (s *Service) SetLinkPresentations(src LinkPresentationSource) { s.linkPresentations = src }

// linkPresentation is the presentation of the smart link `via` when it targets profileUUID.
// Lookup failures fall back to the profile's own page.
func (s *Service) linkPresentation(ctx context.Context, via string, profileUUID uuid.UUID) *routesmodel.LinkPresentation {
	if via == "" || s.linkPresentations == nil {
		return nil
	}
	lp, err := s.linkPresentations.SmartLinkPresentation(ctx, via)
	if err != nil || lp == nil || lp.ProfileUUID != profileUUID {
		return nil
	}
	return lp
}

// SetFeatures wires the design.premium plan check (TEC-947). Without it premium themes and
// backgrounds cannot be picked and public pages render the basic ones (fail-closed).
func (s *Service) SetFeatures(f entitlement.Features) { s.features = f }

// premiumAllowed reports whether public pages of the account may render premium visuals. A
// failed lookup renders the basic ones rather than breaking the page.
func (s *Service) premiumAllowed(ctx context.Context, accountID int64) bool {
	if s.features == nil {
		return false
	}
	ok, err := s.features.CheckBool(ctx, accountID, false, entitlement.FeatureDesignPremium)
	return err == nil && ok
}

// AccountThemeSource resolves the company look of an account (account_branding, TEC-946,
// feeding TEC-601 theme inheritance). It returns nil when the account has none (personal
// accounts never do).
type AccountThemeSource interface {
	AccountBranding(ctx context.Context, accountID int64) (*model.AccountBranding, error)
}

// SetAccountThemes wires company theme inheritance for public pages.
func (s *Service) SetAccountThemes(src AccountThemeSource) { s.accountThemes = src }

// accountBranding is the company look of an account with unknown theme and layout values
// dropped; the zero value when unset or not wired. A broken or stale branding row must not
// break the page.
func (s *Service) accountBranding(ctx context.Context, accountID int64) model.AccountBranding {
	if s.accountThemes == nil {
		return model.AccountBranding{}
	}
	b, err := s.accountThemes.AccountBranding(ctx, accountID)
	if err != nil || b == nil {
		return model.AccountBranding{}
	}
	out := *b
	if !slices.Contains(model.PublicThemes, out.Theme) {
		out.Theme = ""
	}
	if !slices.Contains(model.PublicLayoutModes, out.LayoutMode) {
		out.LayoutMode = ""
	}
	return out
}

// look is what a public page renders for theme, layout and cover.
type look struct {
	theme, layoutMode, coverURL string
}

// resolveLook merges the profile's own choices with the company branding. Unlocked, every field
// the profile set wins and the company fills the rest (its default). Locked, every field the
// company set wins and the profile's own value is only the fallback for fields it left unset.
func resolveLook(cfg presentationConfig, coverURL string, b model.AccountBranding) look {
	pick := func(own, company string) string {
		if b.Locked {
			own, company = company, own
		}
		if own != "" {
			return own
		}
		return company
	}
	return look{
		theme:      pick(cfg.Theme, b.Theme),
		layoutMode: pick(cfg.LayoutMode, b.LayoutMode),
		coverURL:   pick(coverURL, b.CoverURL),
	}
}

type presentationConfig struct {
	VisibleModules []string   `json:"visible_modules"`
	BackgroundUUID *uuid.UUID `json:"background_uuid,omitempty"`
	LeadCapture    struct {
		Enabled bool `json:"enabled"`
	} `json:"lead_capture"`
	ContactExchange struct {
		Enabled bool `json:"enabled"`
	} `json:"contact_exchange"`
	Theme      string `json:"theme,omitempty"`
	LayoutMode string `json:"layout_mode,omitempty"`
}

func decodeConfig(raw []byte) presentationConfig {
	var cfg presentationConfig
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &cfg)
	}
	if cfg.VisibleModules == nil {
		cfg.VisibleModules = slices.Clone(model.PublicModules)
	}
	// Values written before the allow-list (or removed from it) read as unset.
	if !slices.Contains(model.PublicThemes, cfg.Theme) {
		cfg.Theme = ""
	}
	if !slices.Contains(model.PublicLayoutModes, cfg.LayoutMode) {
		cfg.LayoutMode = ""
	}
	return cfg
}

// configPatch is the set of presentation_config keys one update rewrites. Nil / unset fields
// keep the stored value; every other key in the document is preserved.
type configPatch struct {
	modules     []string
	background  model.OptionalUUID
	leadCapture *bool
	exchange    *bool
	theme       *string // "" removes the key (inherit)
	layoutMode  *string // "" removes the key (theme default)
}

func mergeConfig(raw []byte, patch configPatch) ([]byte, error) {
	doc := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &doc)
	}
	doc["visible_modules"] = patch.modules
	setEnabled := func(key string, v *bool) {
		if v == nil {
			return
		}
		section, _ := doc[key].(map[string]any)
		if section == nil {
			section = map[string]any{}
		}
		section["enabled"] = *v
		doc[key] = section
	}
	setEnabled("lead_capture", patch.leadCapture)
	setEnabled("contact_exchange", patch.exchange)
	if patch.background.Set {
		if patch.background.Value == nil {
			delete(doc, "background_uuid")
		} else {
			doc["background_uuid"] = patch.background.Value.String()
		}
	}
	setOrDelete := func(key string, v *string) {
		if v == nil {
			return
		}
		if *v == "" {
			delete(doc, key)
		} else {
			doc[key] = *v
		}
	}
	setOrDelete("theme", patch.theme)
	setOrDelete("layout_mode", patch.layoutMode)
	return json.Marshal(doc)
}

// checkAllowed accepts "" (clear) or a value from the allow-list.
func checkAllowed(field string, v *string, allowed []string) error {
	if v == nil || *v == "" || slices.Contains(allowed, *v) {
		return nil
	}
	return fmt.Errorf("%w: unknown %s %q", ErrInvalidRequest, field, *v)
}

func normalizeModules(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, m := range model.PublicModules { // keep canonical order, drop duplicates
		if slices.Contains(in, m) {
			out = append(out, m)
		}
	}
	for _, m := range in {
		if !slices.Contains(model.PublicModules, m) {
			return nil, fmt.Errorf("%w: unknown module %q", ErrInvalidRequest, m)
		}
	}
	return out, nil
}

// GetPublicPage returns the public page settings of a profile in the active account, assigning
// a username first if the profile has none yet.
func (s *Service) GetPublicPage(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) (*model.PublicPageSettings, error) {
	if s.pages == nil {
		return nil, ErrPublicPagesUnavailable
	}
	detail, err := s.get(ctx, p, profileUUID)
	if err != nil {
		return nil, err
	}
	profileID, _, err := s.pages.ProfileRef(ctx, p.Account.ID, profileUUID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	route, err := s.pages.LiveRoute(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if route == nil {
		if err := s.ensureUsername(ctx, p.Account.ID, detail); err != nil {
			return nil, err
		}
		if route, err = s.pages.LiveRoute(ctx, profileID); err != nil || route == nil {
			return nil, fmt.Errorf("public page: username not assigned: %w", err)
		}
	}
	banks, err := s.pages.PublicBankAccounts(ctx, profileID)
	if err != nil {
		return nil, err
	}
	bankUUIDs := make([]uuid.UUID, 0, len(banks))
	for _, b := range banks {
		bankUUIDs = append(bankUUIDs, b.Uuid)
	}
	cfg := decodeConfig(route.PresentationConfig)
	brand := s.accountBranding(ctx, p.Account.ID)
	return &model.PublicPageSettings{
		Username:       route.Path,
		Enabled:        route.Status == "active",
		VisibleModules: cfg.VisibleModules,
		BankAccounts:   bankUUIDs,
		BackgroundUUID: cfg.BackgroundUUID,
		// Shows the visitor lead form (TEC-552).
		LeadCaptureEnabled: cfg.LeadCapture.Enabled,
		// Lets visitors share their own details back (TEC-567).
		ContactExchangeEnabled: cfg.ContactExchange.Enabled,
		Theme:                  cfg.Theme,
		LayoutMode:             cfg.LayoutMode,
		InheritedTheme:         brand.Theme,
		InheritedLayoutMode:    brand.LayoutMode,
		BrandingLocked:         brand.Locked,
	}, nil
}

// UpdatePublicPage changes the username, visibility, modules or bank account picks.
// Writes need an owner/admin role in the active account.
func (s *Service) UpdatePublicPage(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID, in model.UpdatePublicPageInput) (*model.PublicPageSettings, error) {
	if !p.Account.CanWrite() {
		return nil, ErrForbidden
	}
	current, err := s.GetPublicPage(ctx, p, profileUUID) // also assigns a username if missing
	if err != nil {
		return nil, err
	}
	profileID, ownerUserID, err := s.pages.ProfileRef(ctx, p.Account.ID, profileUUID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	route, err := s.pages.LiveRoute(ctx, profileID)
	if err != nil {
		return nil, err
	}

	apply := repository.ApplyPublicPage{
		ProfileID:   profileID,
		AccountID:   p.Account.ID,
		OwnerUserID: ownerUserID,
		Current:     route,
		Status:      route.Status,
		ChangedBy:   p.UserInternal,
	}
	modules := current.VisibleModules
	if in.VisibleModules != nil {
		if modules, err = normalizeModules(*in.VisibleModules); err != nil {
			return nil, err
		}
	}
	// Premium visuals need design.premium (TEC-947). Only a new pick is checked: the form sends
	// every field on save, and a choice kept from a paid period must not block other changes.
	needPremium := false
	if in.Background.Set && in.Background.Value != nil {
		if s.backgrounds == nil {
			return nil, fmt.Errorf("%w: backgrounds are not available", ErrInvalidRequest)
		}
		ok, premium, err := s.backgrounds.Selectable(ctx, p.Account.ID, *in.Background.Value)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%w: background %s is not available to this account", ErrInvalidRequest, *in.Background.Value)
		}
		changed := current.BackgroundUUID == nil || *current.BackgroundUUID != *in.Background.Value
		needPremium = needPremium || (premium && changed)
	}
	if err := checkAllowed("theme", in.Theme, model.PublicThemes); err != nil {
		return nil, err
	}
	if in.Theme != nil && *in.Theme != current.Theme {
		_, premium := model.PremiumThemes[*in.Theme]
		needPremium = needPremium || premium
	}
	if needPremium {
		if err := entitlement.RequireFeature(ctx, s.features, p.Account.ID, entitlement.FeatureDesignPremium); err != nil {
			return nil, err
		}
	}
	if err := checkAllowed("layout_mode", in.LayoutMode, model.PublicLayoutModes); err != nil {
		return nil, err
	}
	if apply.Config, err = mergeConfig(route.PresentationConfig, configPatch{
		modules: modules, background: in.Background, leadCapture: in.LeadCapture,
		exchange: in.ContactExchange, theme: in.Theme, layoutMode: in.LayoutMode,
	}); err != nil {
		return nil, err
	}
	if in.Enabled != nil {
		apply.Status = "locked"
		if *in.Enabled {
			apply.Status = "active"
		}
	}
	if in.Username != nil {
		username := publicpath.NormalizeUsername(*in.Username)
		if username != route.Path {
			if err := publicpath.ValidateUsername(username); err != nil {
				return nil, fmt.Errorf("%w: %s", ErrInvalidRequest, err.Error())
			}
			owner, err := s.pages.PathOwner(ctx, username)
			if err != nil {
				return nil, err
			}
			if owner != nil {
				// A retired username of this same profile can be taken back; anything else is taken.
				if owner.RouteType != "canonical_profile" || owner.ProfileID != profileID || owner.Status != "retired" {
					return nil, ErrUsernameTaken
				}
				apply.ReviveID = owner.RouteID
			}
			apply.NewPath = username
		}
	}
	if in.BankAccounts != nil {
		ids, err := s.pages.BankAccountIDs(ctx, p.Account.ID, *in.BankAccounts)
		if err != nil {
			return nil, err
		}
		ordered := make([]int64, 0, len(*in.BankAccounts))
		for _, u := range *in.BankAccounts {
			id, ok := ids[u]
			if !ok {
				return nil, fmt.Errorf("%w: bank account %s is not in this account", ErrInvalidRequest, u)
			}
			if !slices.Contains(ordered, id) {
				ordered = append(ordered, id)
			}
		}
		apply.BankIDs = &ordered
	}
	if err := s.pages.Apply(ctx, apply); err != nil {
		return nil, err
	}
	s.routeCache.InvalidateProfile(ctx, profileUUID)
	if apply.NewPath != "" {
		s.indexProfile(ctx, profileUUID) // the username is a search keyword
	}
	if apply.NewPath != "" && apply.Current != nil {
		s.activity.Record(ctx, &p.UserInternal, "profile.username_changed", "profiles", &profileUUID, map[string]any{
			"before": map[string]any{"username": apply.Current.Path},
			"after":  map[string]any{"username": apply.NewPath},
		}, nil)
	}
	return s.GetPublicPage(ctx, p, profileUUID)
}

// ensureUsername gives a profile its first username, derived from its name. Best effort on
// create: a failure leaves the profile without a public page until GetPublicPage retries.
func (s *Service) ensureUsername(ctx context.Context, accountID int64, detail *model.ProfileDetail) error {
	if s.pages == nil {
		return ErrPublicPagesUnavailable
	}
	profileID, ownerUserID, err := s.pages.ProfileRef(ctx, accountID, detail.UUID)
	if err != nil {
		return mapRepoErr(err)
	}
	if route, err := s.pages.LiveRoute(ctx, profileID); err != nil || route != nil {
		return err
	}
	base := usernameBase(detail)
	for i := 1; i <= 50; i++ {
		candidate := base
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		if publicpath.ValidateUsername(candidate) != nil {
			continue
		}
		owner, err := s.pages.PathOwner(ctx, candidate)
		if err != nil {
			return err
		}
		if owner != nil {
			continue
		}
		cfg, _ := mergeConfig(nil, configPatch{modules: model.PublicModules})
		return s.pages.Apply(ctx, repository.ApplyPublicPage{
			ProfileID: profileID, AccountID: accountID, OwnerUserID: ownerUserID,
			NewPath: candidate, Status: "active", Config: cfg,
		})
	}
	return fmt.Errorf("public page: no free username for %q", base)
}

func usernameBase(detail *model.ProfileDetail) string {
	tr := pickTranslation(detail, detail.PrimaryLocale)
	var base string
	if tr != nil {
		base = publicpath.SlugifyName(tr.FirstName, tr.LastName)
		if len(base) < publicpath.MinUsername {
			base = publicpath.SlugifyName(tr.CompanyName)
		}
	}
	if len(base) < publicpath.MinUsername {
		base = "p-" + detail.UUID.String()[:8]
	}
	return base
}

func pickTranslation(detail *model.ProfileDetail, locale string) *model.ProfileTranslationDTO {
	for i := range detail.Translations {
		if detail.Translations[i].Locale == locale {
			return &detail.Translations[i]
		}
	}
	for i := range detail.Translations {
		if detail.Translations[i].IsDefault {
			return &detail.Translations[i]
		}
	}
	if len(detail.Translations) > 0 {
		return &detail.Translations[0]
	}
	return nil
}

// publicRoute resolves a username for visitors. Locked pages and suspended accounts are not
// found; a renamed username returns the current one in RedirectTo.
func (s *Service) publicRoute(ctx context.Context, username string) (*repository.PublicRoute, *model.ProfileDetail, string, error) {
	if s.pages == nil {
		return nil, nil, "", ErrPublicPagesUnavailable
	}
	route, err := s.pages.RouteByPath(ctx, publicpath.NormalizeUsername(username))
	if err != nil {
		return nil, nil, "", mapRepoErr(err)
	}
	switch route.Status {
	case "active":
	case "retired":
		live, err := s.pages.LiveRoute(ctx, route.ProfileID)
		if err != nil {
			return nil, nil, "", err
		}
		if live == nil || live.Status != "active" {
			return nil, nil, "", ErrNotFound
		}
		return route, nil, live.Path, nil
	default:
		return nil, nil, "", ErrNotFound
	}
	detail, err := s.repo.GetByUUID(ctx, route.AccountID, route.ProfileUUID)
	if err != nil {
		return nil, nil, "", mapRepoErr(err)
	}
	return route, detail, "", nil
}

// PublicProfile returns the visitor view of /{username}.
func (s *Service) PublicProfile(ctx context.Context, username string) (*model.PublicProfile, error) {
	return s.PublicProfileVia(ctx, username, "")
}

// PublicProfileVia is PublicProfile opened through a card or smart link (`c/{code}`, `s/{slug}`).
// A smart link targeting this profile may pick the theme, layout and primary action and hide
// modules; it never shows a module the owner hid.
func (s *Service) PublicProfileVia(ctx context.Context, username, via string) (*model.PublicProfile, error) {
	route, detail, redirect, err := s.publicRoute(ctx, username)
	if err != nil {
		return nil, err
	}
	if redirect != "" {
		return &model.PublicProfile{Username: route.Path, RedirectTo: redirect}, nil
	}
	cfg := decodeConfig(route.PresentationConfig)
	modules := cfg.VisibleModules
	// A smart link's theme and layout count as the profile's own: locked company branding and
	// the premium fallback still apply after it (TEC-556).
	primaryAction := ""
	if lp := s.linkPresentation(ctx, via, detail.UUID); lp != nil {
		keep := lp.VisibleModules
		if lp.SocialHub {
			keep = []string{model.ModuleSocialLinks}
		}
		if keep != nil {
			modules = slices.DeleteFunc(slices.Clone(modules), func(m string) bool { return !slices.Contains(keep, m) })
		}
		if lp.Theme != "" {
			cfg.Theme = lp.Theme
		}
		if lp.LayoutMode != "" {
			cfg.LayoutMode = lp.LayoutMode
		}
		primaryAction = lp.PrimaryAction
	}
	show := func(m string) bool { return slices.Contains(modules, m) }
	// Employee profiles take the company look (TEC-601, TEC-946): forced when the company
	// locked its branding, otherwise as the default for what the profile left unset.
	brand := s.accountBranding(ctx, route.AccountID)
	resolved := resolveLook(cfg, detail.CoverURL, brand)

	out := &model.PublicProfile{
		Username:       route.Path,
		ProfileUUID:    detail.UUID,
		PrimaryLocale:  detail.PrimaryLocale,
		AvatarURL:      detail.AvatarURL,
		CoverURL:       resolved.coverURL,
		VisibleModules: modules,
		Translations:   []model.PublicTranslation{},
		Contacts:       []model.PublicContact{},
		Addresses:      []model.PublicAddress{},
		SocialLinks:    []model.PublicSocialLink{},
		BankAccounts:   []model.PublicBankAccount{},
		Theme:          resolved.theme,
		LayoutMode:     resolved.layoutMode,
		PrimaryColor:   brand.PrimaryColor,
		CompanyLogoURL: brand.LogoURL,
		PrimaryAction:  primaryAction,
	}
	if s.verifications != nil {
		// Fail-soft: a lookup error hides the badge rather than breaking the page.
		if ok, err := s.verifications.Verified(ctx, route.ProfileID); err == nil {
			out.Verified = ok
		}
	}
	for _, t := range detail.Translations {
		pt := model.PublicTranslation{
			Locale: t.Locale, Prefix: t.Prefix, FirstName: t.FirstName, LastName: t.LastName,
			MiddleName: t.MiddleName, Suffix: t.Suffix, Nickname: t.Nickname, CompanyName: t.CompanyName,
			Department: t.Department, Title: t.Title, Role: t.Role, IsDefault: t.IsDefault,
		}
		if show(model.ModuleBio) {
			pt.Bio = t.Bio
		}
		out.Translations = append(out.Translations, pt)
	}
	if show(model.ModuleContacts) {
		for _, c := range detail.ContactDetails {
			out.Contacts = append(out.Contacts, model.PublicContact{Category: c.Category, Type: c.Type, Label: c.Label, Value: c.Value, IsPrimary: c.IsPrimary})
		}
	}
	if show(model.ModuleAddresses) {
		for _, a := range detail.Addresses {
			out.Addresses = append(out.Addresses, model.PublicAddress{
				Type: a.Type, Label: a.Label, Street: a.Street, Extended: a.Extended,
				City: a.City, Region: a.Region, PostalCode: a.PostalCode, Country: a.Country,
			})
		}
	}
	if show(model.ModuleSocialLinks) {
		for _, l := range detail.SocialLinks {
			if l.IsActive {
				out.SocialLinks = append(out.SocialLinks, model.PublicSocialLink{Platform: l.Platform, Label: l.Label, URL: l.URL, IconURL: l.IconURL})
			}
		}
	}
	var background *bgmodel.PublicBackground
	if cfg.BackgroundUUID != nil && s.backgrounds != nil {
		// A deleted design simply falls back to the default background.
		if bg, err := s.backgrounds.PublicBackground(ctx, route.AccountID, *cfg.BackgroundUUID); err == nil {
			background = bg
		}
	}
	// After a downgrade, premium visuals render as the basic ones (TEC-947). The stored choice
	// is untouched, so an upgrade brings it back.
	_, premiumTheme := model.PremiumThemes[out.Theme]
	if (premiumTheme || (background != nil && background.Premium)) && !s.premiumAllowed(ctx, route.AccountID) {
		if premiumTheme {
			out.Theme = model.PremiumThemes[out.Theme]
		}
		if background != nil && background.Premium {
			background = nil // the theme's own backdrop
		}
	}
	if background != nil {
		out.Background = background.Document
		out.BackgroundThumbnailURL = background.ThumbnailURL
	}
	if show(model.ModuleBankAccounts) {
		banks, err := s.pages.PublicBankAccounts(ctx, route.ProfileID)
		if err != nil {
			return nil, err
		}
		for _, b := range banks {
			out.BankAccounts = append(out.BankAccounts, model.PublicBankAccount{
				BankName: b.BankName, BankIconURL: b.BankIconUrl, BankBrandColor: b.BankBrandColor,
				AccountHolder: b.AccountHolder, IBAN: b.Iban, Currency: b.Currency, Title: b.Title,
			})
		}
	}
	return out, nil
}

// PublicVCard renders the visitor vCard of /{username}. It never contains private fields
// (note, birthday, gender) nor modules the owner hid. Returns the redirect username when renamed.
func (s *Service) PublicVCard(ctx context.Context, username, locale string) (content, filename, redirect string, err error) {
	route, detail, redirect, err := s.publicRoute(ctx, username)
	if err != nil || redirect != "" {
		return "", "", redirect, err
	}
	modules := decodeConfig(route.PresentationConfig).VisibleModules
	safe := *detail
	safe.Birthday = nil
	safe.Gender = ""
	safe.Translations = make([]model.ProfileTranslationDTO, len(detail.Translations))
	showBio := slices.Contains(modules, model.ModuleBio)
	for i, t := range detail.Translations {
		t.Note = ""
		if !showBio {
			t.Bio = "" // the vCard NOTE line carries the bio
		}
		safe.Translations[i] = t
	}
	if !slices.Contains(modules, model.ModuleContacts) {
		safe.ContactDetails = nil
	}
	if !slices.Contains(modules, model.ModuleAddresses) {
		safe.Addresses = nil
	}
	if !slices.Contains(modules, model.ModuleSocialLinks) {
		safe.SocialLinks = nil
	}
	if locale == "" {
		locale = safe.PrimaryLocale
	}
	content, filename = s.buildContactVCard(ctx, &safe, locale)
	return content, filename, "", nil
}

func mapRepoErr(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
