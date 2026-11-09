package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/google/uuid"
)

type smartLinkFixture struct {
	repo     *mockRouteRepo
	svc      *usecase.Service
	profile  uuid.UUID
	features *stubFeatures
}

// stubFeatures is the bool-feature checker (design.premium) for tests.
type stubFeatures struct{ premium bool }

func (s *stubFeatures) CheckBool(_ context.Context, _ int64, _ bool, key string) (bool, error) {
	return key == entitlement.FeatureDesignPremium && s.premium, nil
}

func newSmartLinkFixture(t *testing.T) smartLinkFixture {
	t.Helper()
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})
	features := &stubFeatures{premium: true}
	svc.SetFeatures(features)
	profile := uuid.New()
	repo.profiles[profile] = 42
	return smartLinkFixture{repo: repo, svc: svc, profile: profile, features: features}
}

// create makes a profile-targeted smart link; the mock repo does not join the profile, so the
// test fills what the real repository reads from it.
func (f smartLinkFixture) create(t *testing.T, slug, target string, cfg map[string]any) *model.RouteDetail {
	t.Helper()
	created, err := f.svc.CreateCustomSmartLink(context.Background(), ownerAccount(1), 1, model.CreateSmartLinkInput{
		Slug: slug, TargetType: target, ProfileUUID: &f.profile, PresentationConfig: cfg,
	})
	if err != nil {
		t.Fatalf("create %s: %v", slug, err)
	}
	stored := f.repo.routes[created.Path]
	stored.ProfileUUID = &f.profile
	stored.ProfileUsername = "ada"
	return created
}

func TestSmartLinkPresentationValidation(t *testing.T) {
	f := newSmartLinkFixture(t)
	ctx := context.Background()

	bad := []map[string]any{
		{"theme": "neon"},
		{"layout_mode": "wide"},
		{"visible_modules": []any{"bio", "secrets"}},
		{"primary_action": map[string]any{"type": "sms"}},
		{"custom_css": "body{}"},
		{"security": map[string]any{"passcode": "12"}},
		{"security": map[string]any{"passcode_enabled": true}},
		{"security": map[string]any{"passcode_enabled": false, "passcode": "1234"}},
		{"lead_capture": map[string]any{"enabled": "yes"}},
	}
	for i, cfg := range bad {
		_, err := f.svc.CreateCustomSmartLink(ctx, ownerAccount(1), 1, model.CreateSmartLinkInput{
			Slug: "bad-" + string(rune('a'+i)), TargetType: model.TargetTypeProfile, ProfileUUID: &f.profile, PresentationConfig: cfg,
		})
		if !errors.Is(err, usecase.ErrInvalidPresentation) {
			t.Errorf("%v: expected ErrInvalidPresentation, got %v", cfg, err)
		}
	}

	created := f.create(t, "event", model.TargetTypeProfile, map[string]any{
		"theme":           "glass",
		"visible_modules": []any{"social_links", "bio"},
		"primary_action":  map[string]any{"type": "whatsapp"},
		"lead_capture":    map[string]any{"enabled": true},
		"security":        map[string]any{"passcode_enabled": true, "passcode": "2468"},
	})
	cfg := created.PresentationConfig
	if cfg["theme"] != "glass" || !model.LeadCaptureOn(cfg) {
		t.Fatalf("config not kept: %v", cfg)
	}
	if mods, _ := cfg["visible_modules"].([]string); strings.Join(mods, ",") != "bio,social_links" {
		t.Fatalf("modules should be deduplicated in display order, got %v", cfg["visible_modules"])
	}
	sec, _ := cfg["security"].(map[string]any)
	if sec["passcode_enabled"] != true || sec["passcode"] != nil || sec[model.PasscodeHashKey] != nil {
		t.Fatalf("owner view must say passcode_enabled only, got %v", sec)
	}
	stored := model.PasscodeHash(f.repo.routes["s/event"].PresentationConfig)
	if stored == "" || strings.Contains(stored, "2468") {
		t.Fatalf("passcode must be stored hashed, got %q", stored)
	}

	// Editing other settings without resending the passcode keeps it.
	updated, err := f.svc.Update(ctx, created.UUID, ownerAccount(1), model.UpdateRouteInput{
		PresentationConfig: map[string]any{"theme": "dark_minimal", "security": map[string]any{"passcode_enabled": true}},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if model.PasscodeHash(f.repo.routes["s/event"].PresentationConfig) != stored {
		t.Fatal("passcode hash should be kept")
	}
	if updated.PresentationConfig["theme"] != "dark_minimal" || model.LeadCaptureOn(updated.PresentationConfig) {
		t.Fatalf("update replaces the other settings: %v", updated.PresentationConfig)
	}

	// Turning it off removes it.
	if _, err := f.svc.Update(ctx, created.UUID, ownerAccount(1), model.UpdateRouteInput{
		PresentationConfig: map[string]any{"security": map[string]any{"passcode_enabled": false}},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if model.PasscodeHash(f.repo.routes["s/event"].PresentationConfig) != "" {
		t.Fatal("passcode should be removed")
	}

	got, err := f.svc.Get(ctx, created.UUID, ownerAccount(1))
	if err != nil || got.PresentationConfig["security"] != nil {
		t.Fatalf("get: %v %v", got, err)
	}
}

func TestSmartLinkTargetValidation(t *testing.T) {
	f := newSmartLinkFixture(t)
	ctx := context.Background()
	cases := []model.CreateSmartLinkInput{
		{Slug: "a1", TargetType: model.TargetTypeCustomModules, ProfileUUID: &f.profile},
		{Slug: "a2", TargetType: model.TargetTypeSocialHub},
		{Slug: "a3", TargetType: model.TargetTypeDirectURL, DirectURL: "javascript:alert(1)"},
		{Slug: "a4", TargetType: model.TargetTypeDirectURL, DirectURL: "http://plain.example"},
	}
	for _, in := range cases {
		if _, err := f.svc.CreateCustomSmartLink(ctx, ownerAccount(1), 1, in); !errors.Is(err, usecase.ErrInvalidTarget) {
			t.Errorf("%+v: expected ErrInvalidTarget, got %v", in, err)
		}
	}

	link := f.create(t, "hub", model.TargetTypeSocialHub, nil)
	if link.TargetType != model.TargetTypeSocialHub {
		t.Fatalf("target = %s", link.TargetType)
	}
	// Switching to a direct URL without an address is refused.
	direct := model.TargetTypeDirectURL
	if _, err := f.svc.Update(ctx, link.UUID, ownerAccount(1), model.UpdateRouteInput{TargetType: &direct}); !errors.Is(err, usecase.ErrInvalidTarget) {
		t.Fatalf("expected ErrInvalidTarget, got %v", err)
	}

	res, err := f.svc.Resolve(ctx, "s/hub")
	if err != nil || res.TargetURL != "https://app.com/ada" {
		t.Fatalf("social hub resolves to the profile page: %+v %v", res, err)
	}
	lp, err := f.svc.SmartLinkPresentation(ctx, "s/hub")
	if err != nil || lp == nil || !lp.SocialHub || lp.ProfileUUID != f.profile {
		t.Fatalf("social hub presentation: %+v %v", lp, err)
	}
}

func TestSmartLinkPresentationForProfilePage(t *testing.T) {
	f := newSmartLinkFixture(t)
	ctx := context.Background()
	f.create(t, "promo", model.TargetTypeProfile, map[string]any{
		"theme": "bold_gradient", "layout_mode": "compact",
		"visible_modules": []any{"contacts"}, "primary_action": map[string]any{"type": "call"},
	})
	lp, err := f.svc.SmartLinkPresentation(ctx, "s/promo")
	if err != nil || lp == nil {
		t.Fatalf("presentation: %v", err)
	}
	if lp.Theme != "bold_gradient" || lp.LayoutMode != "compact" || lp.PrimaryAction != "call" ||
		strings.Join(lp.VisibleModules, ",") != "contacts" || lp.SocialHub {
		t.Fatalf("presentation = %+v", lp)
	}
	for _, via := range []string{"c/promo", "s/missing", ""} {
		if lp, _ := f.svc.SmartLinkPresentation(ctx, via); lp != nil {
			t.Errorf("%q should not present anything", via)
		}
	}
	f.repo.routes["s/promo"].Status = model.RouteStatusLocked
	if lp, _ := f.svc.SmartLinkPresentation(ctx, "s/promo"); lp != nil {
		t.Error("a locked link should not restyle the profile")
	}
}

type stubPasscodeLimiter struct {
	blocked  bool
	failures []string
}

func (s *stubPasscodeLimiter) SmartLinkPasscodeAllowed(_ context.Context, _, _ string) (bool, time.Duration) {
	if s.blocked {
		return false, time.Minute
	}
	return true, 0
}

func (s *stubPasscodeLimiter) RecordSmartLinkPasscodeFailure(_ context.Context, ip, path string) {
	s.failures = append(s.failures, ip+"|"+path)
}

func TestSmartLinkPasscode(t *testing.T) {
	f := newSmartLinkFixture(t)
	ctx := context.Background()
	f.create(t, "vip", model.TargetTypeProfile, map[string]any{
		"security": map[string]any{"passcode": "s3cret"},
	})

	// The public resolution asks for the passcode and leaks neither the target nor the hash.
	res, err := f.svc.Resolve(ctx, "s/vip")
	if err != nil {
		t.Fatal(err)
	}
	if !res.RequiresPasscode || res.TargetURL != "" || res.ProfileUsername != "" || res.ProfileUUID != nil {
		t.Fatalf("locked resolution leaks the target: %+v", res)
	}
	if raw, _ := json.Marshal(res); strings.Contains(string(raw), "passcode_hash") || strings.Contains(string(raw), "argon") {
		t.Fatalf("resolution leaks the passcode hash: %s", raw)
	}

	if _, err := f.svc.UnlockSmartLink(ctx, "vip", "wrong"); !errors.Is(err, usecase.ErrInvalidPasscode) {
		t.Fatalf("wrong passcode: %v", err)
	}
	unlocked, err := f.svc.UnlockSmartLink(ctx, "VIP", "s3cret")
	if err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if unlocked.RequiresPasscode || unlocked.TargetURL != "https://app.com/ada" {
		t.Fatalf("unlocked resolution = %+v", unlocked)
	}

	// Links without a passcode, and locked links, cannot be "unlocked".
	f.create(t, "open", model.TargetTypeProfile, nil)
	if _, err := f.svc.UnlockSmartLink(ctx, "open", "anything"); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("open link: %v", err)
	}
	f.repo.routes["s/vip"].Status = model.RouteStatusLocked
	if _, err := f.svc.UnlockSmartLink(ctx, "vip", "s3cret"); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("locked link: %v", err)
	}
}

func TestUnlockSmartLinkHandler(t *testing.T) {
	f := newSmartLinkFixture(t)
	f.create(t, "vip", model.TargetTypeProfile, map[string]any{
		"security": map[string]any{"passcode": "s3cret"},
	})
	limiter := &stubPasscodeLimiter{}
	h := handler.New(f.svc)
	h.SetPasscodeLimiter(limiter)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/s/{slug}/unlock", h.UnlockSmartLink)
	do := func(slug, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/s/"+slug+"/unlock", strings.NewReader(body))
		req.RemoteAddr = "203.0.113.9:4000"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	if rec := do("vip", `{"passcode":"nope"}`); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "INVALID_PASSCODE") {
		t.Fatalf("wrong passcode: %d %s", rec.Code, rec.Body.String())
	}
	if len(limiter.failures) != 1 || !strings.HasSuffix(limiter.failures[0], "|s/vip") {
		t.Fatalf("wrong passcode should be counted, got %v", limiter.failures)
	}
	if rec := do("vip", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing passcode: %d", rec.Code)
	}
	rec := do("vip", `{"passcode":"s3cret"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "https://app.com/ada") {
		t.Fatalf("unlock: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unlocked target must not be cached")
	}
	if len(limiter.failures) != 1 {
		t.Fatal("a correct passcode is not a failure")
	}
	if rec := do("missing", `{"passcode":"s3cret"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown link: %d", rec.Code)
	}

	// Over the limit even the right passcode is refused, so guessing cannot continue.
	limiter.blocked = true
	rec = do("vip", `{"passcode":"s3cret"}`)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("rate limited: %d %v", rec.Code, rec.Header())
	}
}

// Premium themes need design.premium when a link picks one (TEC-947); a link keeps the theme it
// already has after a downgrade.
func TestSmartLinkPremiumTheme(t *testing.T) {
	f := newSmartLinkFixture(t)
	ctx := context.Background()
	f.features.premium = false

	_, err := f.svc.CreateCustomSmartLink(ctx, ownerAccount(1), 1, model.CreateSmartLinkInput{
		Slug: "fancy", TargetType: model.TargetTypeProfile, ProfileUUID: &f.profile,
		PresentationConfig: map[string]any{"theme": "glass"},
	})
	if !errors.Is(err, entitlement.ErrPlanLimitExceeded) {
		t.Fatalf("premium theme without design.premium: %v", err)
	}
	basic := f.create(t, "basic", model.TargetTypeProfile, map[string]any{"theme": "dark_minimal"})
	if _, err := f.svc.Update(ctx, basic.UUID, ownerAccount(1), model.UpdateRouteInput{
		PresentationConfig: map[string]any{"theme": "elegant_serif"},
	}); !errors.Is(err, entitlement.ErrPlanLimitExceeded) {
		t.Fatalf("switching to a premium theme without design.premium: %v", err)
	}

	f.features.premium = true
	fancy := f.create(t, "fancy", model.TargetTypeProfile, map[string]any{"theme": "glass"})
	f.features.premium = false
	if _, err := f.svc.Update(ctx, fancy.UUID, ownerAccount(1), model.UpdateRouteInput{
		PresentationConfig: map[string]any{"theme": "glass", "lead_capture": map[string]any{"enabled": true}},
	}); err != nil {
		t.Fatalf("keeping the link's premium theme after a downgrade: %v", err)
	}
}
