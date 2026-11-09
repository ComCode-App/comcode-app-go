package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	profilesmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/password"
)

var (
	ErrInvalidPresentation = errors.New("invalid presentation config")
	ErrInvalidTarget       = errors.New("invalid smart link target")
	ErrInvalidPasscode     = errors.New("invalid passcode")
)

// Smart link passcodes: short PINs are allowed, wrong attempts are rate limited by the handler.
const (
	passcodeMinLen = 4
	passcodeMaxLen = 64
)

// SmartLinkTargets are the target types a customer smart link may use (TEC-556).
var SmartLinkTargets = []string{
	model.TargetTypeProfile, model.TargetTypeDirectURL, model.TargetTypeVCardDownload, model.TargetTypeSocialHub,
}

// presentationInput is the owner-editable part of a smart link's presentation_config.
// security.passcode is write-only: it is stored as security.passcode_hash and never returned.
type presentationInput struct {
	Theme          string   `json:"theme"`
	LayoutMode     string   `json:"layout_mode"`
	VisibleModules []string `json:"visible_modules"`
	PrimaryAction  *struct {
		Type string `json:"type"`
	} `json:"primary_action"`
	LeadCapture *struct {
		Enabled bool `json:"enabled"`
	} `json:"lead_capture"`
	Security *struct {
		// PasscodeEnabled false removes the passcode; true without a passcode keeps the current one.
		PasscodeEnabled *bool  `json:"passcode_enabled"`
		Passcode        string `json:"passcode"`
	} `json:"security"`
}

// presentationError wraps a validation message so handlers can answer 400 with it.
func presentationError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidPresentation, fmt.Sprintf(format, args...))
}

// normalizePresentation validates a smart link presentation_config and returns what is stored.
// existing is the stored config of the link being edited (nil on create); its passcode hash is
// kept unless the input sets a new passcode or turns it off.
func normalizePresentation(in, existing map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, presentationError("unreadable")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p presentationInput
	if err := dec.Decode(&p); err != nil {
		return nil, presentationError("%s", strings.TrimPrefix(err.Error(), "json: "))
	}

	out := map[string]any{}
	if p.Theme != "" {
		if !slices.Contains(profilesmodel.PublicThemes, p.Theme) {
			return nil, presentationError("unknown theme %q", p.Theme)
		}
		out["theme"] = p.Theme
	}
	if p.LayoutMode != "" {
		if !slices.Contains(profilesmodel.PublicLayoutModes, p.LayoutMode) {
			return nil, presentationError("unknown layout_mode %q", p.LayoutMode)
		}
		out["layout_mode"] = p.LayoutMode
	}
	if p.VisibleModules != nil {
		modules := []string{}
		for _, m := range profilesmodel.PublicModules {
			if slices.Contains(p.VisibleModules, m) {
				modules = append(modules, m)
			}
		}
		for _, m := range p.VisibleModules {
			if !slices.Contains(profilesmodel.PublicModules, m) {
				return nil, presentationError("unknown module %q", m)
			}
		}
		out["visible_modules"] = modules
	}
	if p.PrimaryAction != nil && p.PrimaryAction.Type != "" {
		if !slices.Contains(model.PrimaryActions, p.PrimaryAction.Type) {
			return nil, presentationError("unknown primary_action %q", p.PrimaryAction.Type)
		}
		out["primary_action"] = map[string]any{"type": p.PrimaryAction.Type}
	}
	if p.LeadCapture != nil {
		out["lead_capture"] = map[string]any{"enabled": p.LeadCapture.Enabled}
	}

	hash := model.PasscodeHash(existing)
	if p.Security != nil {
		switch {
		case p.Security.Passcode != "":
			if p.Security.PasscodeEnabled != nil && !*p.Security.PasscodeEnabled {
				return nil, presentationError("passcode given while passcode_enabled is false")
			}
			if n := utf8.RuneCountInString(p.Security.Passcode); n < passcodeMinLen || n > passcodeMaxLen {
				return nil, presentationError("passcode must be %d-%d characters", passcodeMinLen, passcodeMaxLen)
			}
			if hash, err = password.HashSecret(p.Security.Passcode); err != nil {
				return nil, err
			}
		case p.Security.PasscodeEnabled == nil:
		case !*p.Security.PasscodeEnabled:
			hash = ""
		case hash == "":
			return nil, presentationError("passcode is required")
		}
	}
	if hash != "" {
		out["security"] = map[string]any{model.PasscodeHashKey: hash}
	}
	return out, nil
}

// requirePremiumTheme needs design.premium when a link picks a premium theme (TEC-947). Keeping
// the theme a link already has is allowed: after a downgrade the profile page renders the basic
// one, and an upgrade brings it back.
func (s *Service) requirePremiumTheme(ctx context.Context, accountID int64, cfg, existing map[string]any) error {
	theme, _ := cfg["theme"].(string)
	if _, premium := profilesmodel.PremiumThemes[theme]; !premium {
		return nil
	}
	if current, _ := existing["theme"].(string); current == theme {
		return nil
	}
	return entitlement.RequireFeature(ctx, s.features, accountID, entitlement.FeatureDesignPremium)
}

// ownerPresentation is presentation_config as owners see it: the passcode hash becomes
// security.passcode_enabled.
func ownerPresentation(cfg map[string]any) map[string]any {
	if cfg == nil {
		return nil
	}
	out := maps.Clone(cfg)
	delete(out, "security")
	if model.PasscodeHash(cfg) != "" {
		out["security"] = map[string]any{"passcode_enabled": true}
	}
	return out
}

// publicPresentation is presentation_config as visitors see it: no security settings.
func publicPresentation(cfg map[string]any) map[string]any {
	if cfg == nil {
		return nil
	}
	out := maps.Clone(cfg)
	delete(out, "security")
	return out
}

// ownerView copies a route for an owner response without the stored passcode hash.
func ownerView(route *model.RouteDetail) *model.RouteDetail {
	if route == nil {
		return nil
	}
	out := *route
	out.PresentationConfig = ownerPresentation(route.PresentationConfig)
	return &out
}

// validateTarget checks a smart link's target type and the fields it needs.
func validateTarget(targetType string, hasProfile bool, directURL string) error {
	if !slices.Contains(SmartLinkTargets, targetType) {
		return fmt.Errorf("%w: unsupported target_type %q", ErrInvalidTarget, targetType)
	}
	switch targetType {
	case model.TargetTypeDirectURL:
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(directURL)), "https://") {
			return fmt.Errorf("%w: direct_url must be an https address", ErrInvalidTarget)
		}
	default:
		if !hasProfile {
			return fmt.Errorf("%w: %s needs a profile", ErrInvalidTarget, targetType)
		}
	}
	return nil
}

// UnlockSmartLink checks a visitor's passcode for /s/{slug} and returns the resolution with its
// target. The scan was already counted when the passcode screen resolved the link.
func (s *Service) UnlockSmartLink(ctx context.Context, slug, passcode string) (*model.RouteResolution, error) {
	path := s.NormalizePath("s/" + slug)
	route, err := s.lookupPath(ctx, path)
	if err != nil {
		return nil, err
	}
	if route == nil || route.RouteType != model.RouteTypeCustomSmartLink || route.Status != model.RouteStatusActive {
		return nil, ErrRouteNotFound
	}
	hash := model.PasscodeHash(route.PresentationConfig)
	if hash == "" {
		return nil, ErrRouteNotFound
	}
	ok, err := password.Verify(hash, passcode)
	if err != nil || !ok {
		return nil, ErrInvalidPasscode
	}
	return s.buildResolution(route, true), nil
}

// SmartLinkPresentation returns how an active smart link presents the profile it targets, for
// the public profile page opened through it (?via=s/{slug}). Nil when the path is not such a link.
func (s *Service) SmartLinkPresentation(ctx context.Context, via string) (*model.LinkPresentation, error) {
	path := s.NormalizePath(via)
	if !strings.HasPrefix(path, "s/") {
		return nil, nil
	}
	route, err := s.lookupPath(ctx, path)
	if err != nil || route == nil {
		return nil, err
	}
	if route.RouteType != model.RouteTypeCustomSmartLink || route.Status != model.RouteStatusActive || route.ProfileUUID == nil {
		return nil, nil
	}
	if route.TargetType != model.TargetTypeProfile && route.TargetType != model.TargetTypeSocialHub {
		return nil, nil
	}
	out := &model.LinkPresentation{ProfileUUID: *route.ProfileUUID, SocialHub: route.TargetType == model.TargetTypeSocialHub}
	cfg := route.PresentationConfig
	out.Theme, _ = cfg["theme"].(string)
	out.LayoutMode, _ = cfg["layout_mode"].(string)
	switch mods := cfg["visible_modules"].(type) {
	case []string: // as normalizePresentation stores it, before a JSON round trip
		out.VisibleModules = mods
	case []any:
		out.VisibleModules = []string{}
		for _, m := range mods {
			if s, ok := m.(string); ok {
				out.VisibleModules = append(out.VisibleModules, s)
			}
		}
	}
	if pa, ok := cfg["primary_action"].(map[string]any); ok {
		out.PrimaryAction, _ = pa["type"].(string)
	}
	return out, nil
}
