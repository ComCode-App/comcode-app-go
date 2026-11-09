package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"strings"
	"sync"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/usecase"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	profilesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

// memMedia records objects; URLs use the stream proxy path like the API without a public base.
type memMedia struct {
	mu      sync.Mutex
	objects map[string]string
}

func (m *memMedia) Upload(_ context.Context, f storage.File, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[path] = f.ContentType
	return nil
}

func (m *memMedia) Delete(_ context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, path)
	return nil
}

func (m *memMedia) PublicURL(path string) string { return "/api/v1/storage/stream/" + path }

type allowQuotas struct{}

func (allowQuotas) CheckQuota(context.Context, int64, bool, string) error { return nil }

var pngBytes = func() []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	return buf.Bytes()
}()

var pdfBytes = []byte("%PDF-1.4\n1 0 obj << >> endobj\ntrailer << >>\n%%EOF\n")

type env struct {
	t     *testing.T
	d     *pgtest.DB
	ctx   context.Context
	media *memMedia
	svc   *usecase.Service
}

func newEnv(t *testing.T) *env {
	d := pgtest.Open(t)
	media := &memMedia{objects: map[string]string{}}
	svc := usecase.New(d.Queries, media)
	svc.SetFeatures(entitlement.New(d.Queries))
	return &env{t: t, d: d, ctx: context.Background(), media: media, svc: svc}
}

func (e *env) company(name string) (db.Account, pgtest.User) {
	e.t.Helper()
	owner := e.d.CreateUser(e.t)
	a, err := e.d.Queries.CreateCompanyAccount(e.ctx, db.CreateCompanyAccountParams{Name: name, Slug: "brand-" + pgtest.Unique()})
	if err != nil {
		e.t.Fatal(err)
	}
	e.member(a, owner, db.AccountRoleOwner)
	return a, owner
}

func (e *env) member(a db.Account, u pgtest.User, role db.AccountRole) {
	e.t.Helper()
	if err := e.d.Queries.AddAccountMember(e.ctx, db.AddAccountMemberParams{AccountID: a.ID, UserID: u.ID, Role: role}); err != nil {
		e.t.Fatal(err)
	}
}

func principal(a db.Account, u pgtest.User, role string) authctx.Principal {
	return authctx.Principal{
		UserID: u.UUID, UserInternal: u.ID,
		Permissions: []string{
			rbac.PermAppProfilesRead, rbac.PermAppProfilesWrite,
			rbac.PermAppCompanyBrandingRead, rbac.PermAppCompanyBrandingWrite,
		},
		Account: authctx.Account{ID: a.ID, UUID: a.Uuid, Type: string(a.Type), Role: role},
	}
}

func (e *env) upload(p authctx.Principal, kind string, body []byte) (*model.Media, error) {
	return e.svc.Upload(e.ctx, p, model.UploadInput{Kind: kind, Name: kind + " file", Filename: "f", Size: int64(len(body))}, bytes.NewReader(body))
}

func TestIntegrationLibraryScopingAndRoles(t *testing.T) {
	e := newEnv(t)
	acme, acmeOwner := e.company("Acme")
	other, otherOwner := e.company("Other")
	adminUser, memberUser := e.d.CreateUser(t), e.d.CreateUser(t)
	e.member(acme, adminUser, db.AccountRoleAdmin)
	e.member(acme, memberUser, db.AccountRoleMember)
	owner := principal(acme, acmeOwner, authctx.AccountRoleOwner)
	admin := principal(acme, adminUser, authctx.AccountRoleAdmin)
	member := principal(acme, memberUser, authctx.AccountRoleMember)
	foreign := principal(other, otherOwner, authctx.AccountRoleOwner)

	// Writes need owner/admin; members read.
	if _, err := e.upload(member, model.KindLogo, pngBytes); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("member upload: %v", err)
	}
	logo, err := e.upload(admin, model.KindLogo, pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(logo.URL, "/api/v1/storage/stream/accounts/"+acme.Uuid.String()+"/media/") || logo.ContentType != "image/png" {
		t.Fatalf("uploaded file: %+v", logo)
	}
	doc, err := e.upload(owner, model.KindDocument, pdfBytes)
	if err != nil || doc.ContentType != "application/pdf" {
		t.Fatalf("document: %+v %v", doc, err)
	}
	if items, total, err := e.svc.List(e.ctx, member, model.Filter{}); err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("member list: %d %v", total, err)
	}
	if items, _, _ := e.svc.List(e.ctx, member, model.Filter{Kind: model.KindDocument}); len(items) != 1 || items[0].UUID != doc.UUID {
		t.Fatalf("kind filter: %+v", items)
	}
	if err := e.svc.Delete(e.ctx, member, doc.UUID); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("member delete: %v", err)
	}
	if _, err := e.svc.UpdateBranding(e.ctx, member, model.UpdateBrandingInput{Locked: true}); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("member branding write: %v", err)
	}
	if _, err := e.svc.GetBranding(e.ctx, member); err != nil {
		t.Fatalf("member branding read: %v", err)
	}
	noPerm := owner
	noPerm.Permissions = []string{rbac.PermAppProfilesRead}
	if _, _, err := e.svc.List(e.ctx, noPerm, model.Filter{}); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("list without permission: %v", err)
	}
	// Personal accounts have no library.
	solo := e.d.CreateUser(t)
	personal := authctx.Principal{UserID: solo.UUID, UserInternal: solo.ID, Permissions: owner.Permissions,
		Account: authctx.Account{ID: solo.AccountID, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner}}
	if _, _, err := e.svc.List(e.ctx, personal, model.Filter{}); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("personal list: %v", err)
	}

	// Content is sniffed: the kind decides what is accepted.
	if _, err := e.upload(owner, model.KindDocument, pngBytes); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("png as document: %v", err)
	}
	if _, err := e.upload(owner, model.KindCover, pdfBytes); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("pdf as cover: %v", err)
	}
	if _, err := e.upload(owner, model.KindImage, []byte("<svg xmlns='http://www.w3.org/2000/svg'/>")); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("svg accepted: %v", err)
	}
	// Logos may be SVG, but only static vector content (TEC-1097).
	svgLogo, err := e.upload(owner, model.KindLogo, []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#123"/></svg>`))
	if err != nil {
		t.Fatalf("clean svg logo: %v", err)
	}
	if svgLogo.ContentType != "image/svg+xml" || !strings.HasSuffix(svgLogo.URL, ".svg") {
		t.Fatalf("svg logo stored as %q at %q", svgLogo.ContentType, svgLogo.URL)
	}
	if _, err := e.upload(owner, model.KindLogo, []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`)); !errors.Is(err, upload.ErrSVGUnsafe) {
		t.Fatalf("unsafe svg logo: %v", err)
	}
	if _, err := e.upload(owner, model.KindLogo, []byte("<html><script>alert(1)</script></html>")); !errors.Is(err, upload.ErrUnsupportedType) {
		t.Fatalf("html logo: %v", err)
	}

	// Another company sees and touches nothing of Acme's.
	if items, total, _ := e.svc.List(e.ctx, foreign, model.Filter{}); total != 0 || len(items) != 0 {
		t.Fatalf("foreign list: %d", total)
	}
	if err := e.svc.Delete(e.ctx, foreign, logo.UUID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	if _, err := e.svc.UpdateBranding(e.ctx, foreign, model.UpdateBrandingInput{LogoMediaUUID: &logo.UUID}); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("foreign logo pick: %v", err)
	}

	// Branding: validation, then the logo is in use and cannot be deleted.
	for _, bad := range []model.UpdateBrandingInput{
		{Theme: "neon"}, {LayoutMode: "grid"}, {PrimaryColor: "red"}, {PrimaryColor: "#12345"},
		{LogoMediaUUID: &doc.UUID},
	} {
		if _, err := e.svc.UpdateBranding(e.ctx, owner, bad); !errors.Is(err, model.ErrInvalid) {
			t.Errorf("invalid branding accepted %+v: %v", bad, err)
		}
	}
	b, err := e.svc.UpdateBranding(e.ctx, admin, model.UpdateBrandingInput{
		Theme: "dark_minimal", LayoutMode: "compact", PrimaryColor: "#AA3300", LogoMediaUUID: &logo.UUID, Locked: true,
	})
	if err != nil || b.Logo == nil || b.Logo.URL != logo.URL || b.PrimaryColor != "#aa3300" || !b.Locked || b.UpdatedAt == nil {
		t.Fatalf("branding: %+v %v", b, err)
	}
	if fb, _ := e.svc.GetBranding(e.ctx, foreign); fb.Logo != nil || fb.Theme != "" || fb.UpdatedAt != nil {
		t.Fatalf("branding leaked across accounts: %+v", fb)
	}
	var inUse *model.InUseError
	if err := e.svc.Delete(e.ctx, owner, logo.UUID); !errors.As(err, &inUse) || inUse.Usage[0].Type != model.UsageBrandingLogo {
		t.Fatalf("delete in-use logo: %v", err)
	}
	if _, err := e.svc.UpdateBranding(e.ctx, owner, model.UpdateBrandingInput{}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Delete(e.ctx, owner, logo.UUID); err != nil {
		t.Fatalf("delete unused logo: %v", err)
	}
	if _, ok := e.media.objects[strings.TrimPrefix(logo.URL, "/api/v1/storage/stream/")]; ok {
		t.Fatal("object left in storage")
	}
}

// Employee profiles render the company look on the public page: as the default while unlocked,
// forced while locked. Profiles of other accounts are never affected.
func TestIntegrationPublicPageInheritance(t *testing.T) {
	e := newEnv(t)
	acme, acmeOwner := e.company("Acme")
	owner := principal(acme, acmeOwner, authctx.AccountRoleOwner)
	repo := profilesrepo.NewPostgres(e.d.Pool, e.d.Queries)
	profiles := profilesusecase.NewService(repo, nil)
	profiles.SetQuotas(allowQuotas{})
	profiles.SetPublicPages(repo)
	profiles.SetAccountThemes(e.svc)
	// The themes below are premium (TEC-947): the company is on a paid plan until the end.
	profiles.SetFeatures(entitlement.New(e.d.Queries))
	if _, err := e.d.Pool.Exec(e.ctx, `
		INSERT INTO subscriptions (account_id, plan_id, status, source, starts_at, next_billing_at)
		SELECT $1, id, 'active', 'manual', now(), now() + interval '30 days' FROM plans WHERE slug = 'pro-monthly'`,
		acme.ID); err != nil {
		t.Fatal(err)
	}

	create := func(p authctx.Principal, first string) (string, *pmodel.ProfileDetail) {
		t.Helper()
		prof, err := profiles.Create(e.ctx, p, pmodel.UpsertProfileInput{PrimaryLocale: "tr", Translations: []pmodel.UpsertTranslationInput{
			{Locale: "tr", FirstName: first, LastName: "Test" + pgtest.Unique(), IsDefault: true},
		}})
		if err != nil {
			t.Fatal(err)
		}
		page, err := profiles.GetPublicPage(e.ctx, p, prof.UUID)
		if err != nil {
			t.Fatal(err)
		}
		return page.Username, prof
	}
	username, prof := create(owner, "Ada")

	// No branding yet: the profile's plain look.
	public, err := profiles.PublicProfile(e.ctx, username)
	if err != nil || public.Theme != "" || public.CompanyLogoURL != "" {
		t.Fatalf("no branding: %+v %v", public, err)
	}

	logo, err := e.upload(owner, model.KindLogo, pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	cover, err := e.upload(owner, model.KindCover, pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdateBranding(e.ctx, owner, model.UpdateBrandingInput{
		Theme: "elegant_serif", LayoutMode: "minimal", PrimaryColor: "#0055ff",
		LogoMediaUUID: &logo.UUID, CoverMediaUUID: &cover.UUID,
	}); err != nil {
		t.Fatal(err)
	}

	// Unlocked: the company look fills what the profile left unset ...
	public, _ = profiles.PublicProfile(e.ctx, username)
	if public.Theme != "elegant_serif" || public.LayoutMode != "minimal" || public.CoverURL != cover.URL ||
		public.CompanyLogoURL != logo.URL || public.PrimaryColor != "#0055ff" {
		t.Fatalf("unlocked default: %+v", public)
	}
	// ... and the profile's own choices win.
	theme, layout := "glass", "compact"
	if _, err := profiles.UpdatePublicPage(e.ctx, owner, prof.UUID, pmodel.UpdatePublicPageInput{Theme: &theme, LayoutMode: &layout}); err != nil {
		t.Fatal(err)
	}
	public, _ = profiles.PublicProfile(e.ctx, username)
	if public.Theme != "glass" || public.LayoutMode != "compact" {
		t.Fatalf("unlocked own choice: %+v", public)
	}

	// Locked: the company look is forced.
	if _, err := e.svc.UpdateBranding(e.ctx, owner, model.UpdateBrandingInput{
		Theme: "elegant_serif", LayoutMode: "minimal", PrimaryColor: "#0055ff",
		LogoMediaUUID: &logo.UUID, CoverMediaUUID: &cover.UUID, Locked: true,
	}); err != nil {
		t.Fatal(err)
	}
	public, _ = profiles.PublicProfile(e.ctx, username)
	if public.Theme != "elegant_serif" || public.LayoutMode != "minimal" || public.CoverURL != cover.URL {
		t.Fatalf("locked: %+v", public)
	}
	page, _ := profiles.GetPublicPage(e.ctx, owner, prof.UUID)
	if !page.BrandingLocked || page.Theme != "glass" || page.InheritedTheme != "elegant_serif" {
		t.Fatalf("locked settings: %+v", page)
	}
	// Without design.premium the locked premium company theme renders as its basic stand-in,
	// and comes back with the plan (TEC-947).
	if _, err := e.d.Pool.Exec(e.ctx, `UPDATE subscriptions SET status = 'canceled', ends_at = now() - interval '1 day' WHERE account_id = $1`, acme.ID); err != nil {
		t.Fatal(err)
	}
	public, _ = profiles.PublicProfile(e.ctx, username)
	if public.Theme != pmodel.PremiumThemes["elegant_serif"] || public.LayoutMode != "minimal" {
		t.Fatalf("locked premium theme on the free plan: %+v", public)
	}
	// The kept company theme can still be re-saved; a new premium pick is refused.
	keep := model.UpdateBrandingInput{Theme: "elegant_serif", LayoutMode: "minimal", PrimaryColor: "#0055ff", Locked: true}
	if _, err := e.svc.UpdateBranding(e.ctx, owner, keep); err != nil {
		t.Fatalf("re-saving the kept theme: %v", err)
	}
	keep.Theme = "glass"
	if _, err := e.svc.UpdateBranding(e.ctx, owner, keep); !errors.Is(err, entitlement.ErrPlanLimitExceeded) {
		t.Fatalf("new premium company theme on the free plan: %v", err)
	}

	// The library lists a profile that uses a file directly (avatar set to the logo).
	in := pmodel.UpsertProfileInput{PrimaryLocale: "tr", AvatarURL: logo.URL, Translations: []pmodel.UpsertTranslationInput{
		{Locale: "tr", FirstName: "Ada", LastName: "Test", IsDefault: true},
	}}
	if _, err := profiles.Update(e.ctx, owner, prof.UUID, in); err != nil {
		t.Fatalf("avatar from the library: %v", err)
	}
	items, _, _ := e.svc.List(e.ctx, owner, model.Filter{Kind: model.KindLogo})
	var avatarUse bool
	for _, u := range items[0].Usage {
		avatarUse = avatarUse || (u.Type == model.UsageProfileAvatar && *u.ProfileUUID == prof.UUID)
	}
	if !avatarUse {
		t.Fatalf("profile usage missing: %+v", items[0].Usage)
	}

	// Another company cannot point a profile at Acme's library, and its pages stay unbranded.
	other, otherOwner := e.company("Other")
	foreign := principal(other, otherOwner, authctx.AccountRoleOwner)
	otherName, otherProf := create(foreign, "Alan")
	in.AvatarURL = logo.URL
	if _, err := profiles.Update(e.ctx, foreign, otherProf.UUID, in); !errors.Is(err, profilesusecase.ErrInvalidRequest) {
		t.Fatalf("foreign library URL accepted: %v", err)
	}
	if public, _ := profiles.PublicProfile(e.ctx, otherName); public.Theme != "" || public.CompanyLogoURL != "" || public.PrimaryColor != "" {
		t.Fatalf("branding leaked to another company: %+v", public)
	}
}
