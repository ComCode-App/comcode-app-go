package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	bgmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	routesmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/google/uuid"
)

type memRoute struct {
	repository.CanonicalRoute
	profileID int64
	routeType string
}

// memPages is an in-memory PublicPageStore over mockRepo's profiles.
type memPages struct {
	repo    *mockRepo
	ids     map[uuid.UUID]int64
	routes  []*memRoute
	banks   map[uuid.UUID]int64 // bank account uuid -> id (all in account 1)
	picks   map[int64][]int64
	applied int
}

func newMemPages(repo *mockRepo) *memPages {
	return &memPages{repo: repo, ids: map[uuid.UUID]int64{}, banks: map[uuid.UUID]int64{}, picks: map[int64][]int64{}}
}

func (m *memPages) idOf(u uuid.UUID) int64 {
	if id, ok := m.ids[u]; ok {
		return id
	}
	id := int64(len(m.ids) + 1)
	m.ids[u] = id
	return id
}

func (m *memPages) ProfileRef(_ context.Context, accountID int64, u uuid.UUID) (int64, int64, error) {
	if _, err := m.repo.find(accountID, u); err != nil {
		return 0, 0, err
	}
	return m.idOf(u), 10, nil
}

func (m *memPages) LiveRoute(_ context.Context, profileID int64) (*repository.CanonicalRoute, error) {
	for _, r := range m.routes {
		if r.profileID == profileID && r.routeType == "canonical_profile" && (r.Status == "active" || r.Status == "locked") {
			c := r.CanonicalRoute
			return &c, nil
		}
	}
	return nil, nil
}

func (m *memPages) PathOwner(_ context.Context, path string) (*repository.PathOwner, error) {
	for _, r := range m.routes {
		if r.Path == path {
			return &repository.PathOwner{RouteType: r.routeType, Status: r.Status, ProfileID: r.profileID, RouteID: r.ID}, nil
		}
	}
	return nil, nil
}

func (m *memPages) RouteByPath(_ context.Context, path string) (*repository.PublicRoute, error) {
	for _, r := range m.routes {
		if r.Path == path && r.routeType == "canonical_profile" {
			for u, id := range m.ids {
				if id == r.profileID {
					return &repository.PublicRoute{CanonicalRoute: r.CanonicalRoute, ProfileID: id, ProfileUUID: u, AccountID: m.repo.accounts[u]}, nil
				}
			}
		}
	}
	return nil, repository.ErrNotFound
}

func (m *memPages) BankAccountIDs(_ context.Context, accountID int64, uuids []uuid.UUID) (map[uuid.UUID]int64, error) {
	out := map[uuid.UUID]int64{}
	for _, u := range uuids {
		if id, ok := m.banks[u]; ok && accountID == 1 {
			out[u] = id
		}
	}
	return out, nil
}

func (m *memPages) PublicBankAccounts(_ context.Context, profileID int64) ([]db.ListProfileBankAccountsPublicRow, error) {
	var out []db.ListProfileBankAccountsPublicRow
	for _, id := range m.picks[profileID] {
		for u, bid := range m.banks {
			if bid == id {
				out = append(out, db.ListProfileBankAccountsPublicRow{Uuid: u, Iban: "TR00", BankName: "Bank"})
			}
		}
	}
	return out, nil
}

func (m *memPages) Apply(_ context.Context, in repository.ApplyPublicPage) error {
	m.applied++
	switch {
	case in.NewPath != "":
		if in.Current != nil {
			m.byID(in.Current.ID).Status = "retired"
		}
		if in.ReviveID != 0 {
			r := m.byID(in.ReviveID)
			r.Status, r.PresentationConfig = in.Status, in.Config
			break
		}
		m.routes = append(m.routes, &memRoute{
			CanonicalRoute: repository.CanonicalRoute{ID: int64(len(m.routes) + 1), Path: in.NewPath, Status: in.Status, PresentationConfig: in.Config},
			profileID:      in.ProfileID, routeType: "canonical_profile",
		})
	case in.Current != nil:
		r := m.byID(in.Current.ID)
		r.Status, r.PresentationConfig = in.Status, in.Config
	}
	if in.BankIDs != nil {
		m.picks[in.ProfileID] = *in.BankIDs
	}
	return nil
}

func (m *memPages) byID(id int64) *memRoute {
	for _, r := range m.routes {
		if r.ID == id {
			return r
		}
	}
	return nil
}

func newPublicPageService(t *testing.T) (*Service, *memPages, *mockRepo) {
	t.Helper()
	repo := newMockRepo()
	pages := newMemPages(repo)
	svc := NewService(repo, nil)
	svc.SetQuotas(&stubQuotas{})
	svc.SetPublicPages(pages)
	premium := true
	svc.SetFeatures(stubFeatures{premium: &premium})
	return svc, pages, repo
}

func profileInput(first, last string) model.UpsertProfileInput {
	return model.UpsertProfileInput{
		PrimaryLocale: "tr",
		Translations:  []model.UpsertTranslationInput{{Locale: "tr", FirstName: first, LastName: last, IsDefault: true}},
	}
}

func TestCreateAssignsUniqueUsername(t *testing.T) {
	svc, pages, _ := newPublicPageService(t)
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	// A smart link already holds the obvious username.
	pages.routes = append(pages.routes, &memRoute{CanonicalRoute: repository.CanonicalRoute{ID: 99, Path: "furkan-meclis", Status: "active"}, routeType: "custom_smart_link"})

	a, err := svc.Create(ctx, owner, profileInput("Furkan", "Meçlis"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Create(ctx, owner, profileInput("Furkan", "Meclis"))
	if err != nil {
		t.Fatal(err)
	}
	if a.Username != "furkan-meclis-2" || b.Username != "furkan-meclis-3" {
		t.Fatalf("usernames %q, %q", a.Username, b.Username)
	}
	// A name that slugifies to nothing still gets a username.
	c, _ := svc.Create(ctx, owner, profileInput("李", "王"))
	if !strings.HasPrefix(c.Username, "p-") {
		t.Fatalf("fallback username %q", c.Username)
	}
}

func TestUpdatePublicPageRenameRetiresAndRedirects(t *testing.T) {
	svc, pages, _ := newPublicPageService(t)
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))

	newName := "ada"
	page, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{Username: &newName})
	if err != nil || page.Username != "ada" {
		t.Fatalf("rename: %+v %v", page, err)
	}
	// The old username redirects to the new one.
	old, err := svc.PublicProfile(ctx, "ada-lovelace")
	if err != nil || old.RedirectTo != "ada" {
		t.Fatalf("old username: %+v %v", old, err)
	}
	// Taking the old username back revives it instead of creating a duplicate.
	back := "ada-lovelace"
	if _, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{Username: &back}); err != nil {
		t.Fatalf("revive: %v", err)
	}
	if n := len(pages.routes); n != 2 {
		t.Fatalf("expected 2 routes after revive, got %d", n)
	}
}

func TestUpdatePublicPageRejectsTakenReservedAndInvalid(t *testing.T) {
	svc, _, _ := newPublicPageService(t)
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	a, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))
	b, _ := svc.Create(ctx, owner, profileInput("Alan", "Turing"))

	taken := a.Username
	if _, err := svc.UpdatePublicPage(ctx, owner, b.UUID, model.UpdatePublicPageInput{Username: &taken}); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("taken: %v", err)
	}
	for _, bad := range []string{"login", "ab", "Ada Lovelace", "-x-"} {
		u := bad
		if _, err := svc.UpdatePublicPage(ctx, owner, b.UUID, model.UpdatePublicPageInput{Username: &u}); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	mods := []string{"bio", "secrets"}
	if _, err := svc.UpdatePublicPage(ctx, owner, b.UUID, model.UpdatePublicPageInput{VisibleModules: &mods}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("unknown module: %v", err)
	}
	member := principalIn(1, authctx.AccountRoleMember)
	if _, err := svc.UpdatePublicPage(ctx, member, b.UUID, model.UpdatePublicPageInput{}); !errors.Is(err, ErrForbidden) {
		t.Errorf("member: %v", err)
	}
	other := principalIn(2, authctx.AccountRoleOwner)
	if _, err := svc.GetPublicPage(ctx, other, b.UUID); !errors.Is(err, ErrNotFound) {
		t.Errorf("other account: %v", err)
	}
}

func TestPublicProfileHonoursVisibilityAndBanks(t *testing.T) {
	svc, pages, repo := newPublicPageService(t)
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))
	d := repo.profiles[prof.UUID]
	d.Translations[0].Bio, d.Translations[0].Note = "Mathematician", "private note"
	d.ContactDetails = []model.ContactDetailDTO{{Category: "phone", Value: "+90"}}
	d.SocialLinks = []model.SocialLinkDTO{{Platform: "x", URL: "https://x.com/ada", IsActive: true}, {Platform: "old", IsActive: false}}
	bank := uuid.New()
	pages.banks[bank] = 7

	public, err := svc.PublicProfile(ctx, prof.Username)
	if err != nil {
		t.Fatal(err)
	}
	if public.Translations[0].Bio != "Mathematician" || len(public.Contacts) != 1 || len(public.SocialLinks) != 1 || len(public.BankAccounts) != 0 {
		t.Fatalf("default view: %+v", public)
	}

	mods := []string{"social_links", "bank_accounts"}
	banks := []uuid.UUID{bank}
	if _, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{VisibleModules: &mods, BankAccounts: &banks}); err != nil {
		t.Fatal(err)
	}
	public, _ = svc.PublicProfile(ctx, prof.Username)
	if public.Translations[0].Bio != "" || len(public.Contacts) != 0 || len(public.BankAccounts) != 1 {
		t.Fatalf("restricted view: %+v", public)
	}
	vcf, _, _, err := svc.PublicVCard(ctx, prof.Username, "")
	if err != nil || strings.Contains(vcf, "private note") || strings.Contains(vcf, "+90") || strings.Contains(vcf, "Mathematician") {
		t.Fatalf("vcard leaks hidden data (err %v):\n%s", err, vcf)
	}

	// A bank account from another account cannot be picked.
	foreign := []uuid.UUID{uuid.New()}
	if _, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{BankAccounts: &foreign}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("foreign bank: %v", err)
	}

	// Disabling the page hides it from visitors.
	off := false
	if _, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublicProfile(ctx, prof.Username); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disabled page: %v", err)
	}
}

type stubBackgrounds struct{ docs map[uuid.UUID][]byte }

func (s stubBackgrounds) Selectable(_ context.Context, accountID int64, id uuid.UUID) (bool, bool, error) {
	doc, ok := s.docs[id]
	return ok && accountID == 1, bgmodel.IsPremium(doc), nil
}

func (s stubBackgrounds) PublicBackground(_ context.Context, _ int64, id uuid.UUID) (*bgmodel.PublicBackground, error) {
	doc, ok := s.docs[id]
	if !ok {
		return nil, nil
	}
	return &bgmodel.PublicBackground{Document: doc, Premium: bgmodel.IsPremium(doc), ThumbnailURL: "/media/" + id.String() + ".jpg"}, nil
}

// stubFeatures is the account plan's design.premium value.
type stubFeatures struct{ premium *bool }

func (s stubFeatures) CheckBool(_ context.Context, _ int64, isSuperAdmin bool, key string) (bool, error) {
	if key != entitlement.FeatureDesignPremium {
		return false, nil
	}
	return isSuperAdmin || *s.premium, nil
}

func TestPublicPageBackground(t *testing.T) {
	svc, _, _ := newPublicPageService(t)
	bg := uuid.New()
	svc.SetBackgrounds(stubBackgrounds{docs: map[uuid.UUID][]byte{bg: []byte(`{"kind":"solid","params":{"color":"#000"}}`)}})
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))

	page, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{Background: model.OptionalUUID{Set: true, Value: &bg}})
	if err != nil || page.BackgroundUUID == nil || *page.BackgroundUUID != bg {
		t.Fatalf("set background: %+v %v", page, err)
	}
	public, _ := svc.PublicProfile(ctx, prof.Username)
	if !strings.Contains(string(public.Background), `"solid"`) {
		t.Fatalf("public background missing: %s", public.Background)
	}
	// Leaving the field out keeps it; null clears it.
	page, _ = svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{})
	if page.BackgroundUUID == nil {
		t.Fatal("background dropped by an unrelated update")
	}
	page, _ = svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{Background: model.OptionalUUID{Set: true}})
	if page.BackgroundUUID != nil {
		t.Fatal("null did not clear the background")
	}
	foreign := uuid.New()
	if _, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{Background: model.OptionalUUID{Set: true, Value: &foreign}}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unusable background: %v", err)
	}
}

func TestExistingUsernameSurvivesNewReservedWord(t *testing.T) {
	svc, pages, _ := newPublicPageService(t)
	owner := principalIn(1, authctx.AccountRoleOwner)
	profile, err := svc.Create(t.Context(), owner, profileInput("Existing", "Owner"))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a username assigned before checkout was added to the reserved list.
	pages.routes[0].Path = "checkout"
	username := "checkout"
	if _, err := svc.UpdatePublicPage(t.Context(), owner, profile.UUID, model.UpdatePublicPageInput{Username: &username}); err != nil {
		t.Fatalf("retaining an existing reserved username: %v", err)
	}
	public, err := svc.PublicProfile(t.Context(), username)
	if err != nil || public.Username != username {
		t.Fatalf("reading existing reserved username: %+v %v", public, err)
	}
	other, err := svc.Create(t.Context(), owner, profileInput("Another", "Owner"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdatePublicPage(t.Context(), owner, other.UUID, model.UpdatePublicPageInput{Username: &username}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("new rename to reserved username must fail: %v", err)
	}
}

type stubAccountThemes map[int64]string

func (s stubAccountThemes) AccountBranding(_ context.Context, accountID int64) (*model.AccountBranding, error) {
	if s[accountID] == "" {
		return nil, nil
	}
	return &model.AccountBranding{Theme: s[accountID]}, nil
}

type stubBranding map[int64]*model.AccountBranding

func (s stubBranding) AccountBranding(_ context.Context, accountID int64) (*model.AccountBranding, error) {
	return s[accountID], nil
}

func TestPublicPageTheme(t *testing.T) {
	svc, _, _ := newPublicPageService(t)
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))

	theme, layout := "dark_minimal", "compact"
	page, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{Theme: &theme, LayoutMode: &layout})
	if err != nil || page.Theme != theme || page.LayoutMode != layout {
		t.Fatalf("set theme: %+v %v", page, err)
	}
	public, _ := svc.PublicProfile(ctx, prof.Username)
	if public.Theme != theme || public.LayoutMode != layout {
		t.Fatalf("public theme: %q / %q", public.Theme, public.LayoutMode)
	}
	// Unrelated updates keep the theme; "" clears it.
	page, _ = svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{})
	if page.Theme != theme {
		t.Fatal("theme dropped by an unrelated update")
	}
	empty := ""
	page, _ = svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{Theme: &empty, LayoutMode: &empty})
	if page.Theme != "" || page.LayoutMode != "" {
		t.Fatalf("clear theme: %+v", page)
	}
	for _, bad := range []model.UpdatePublicPageInput{
		{Theme: ptr("neon_<script>")},
		{LayoutMode: ptr("grid")},
	} {
		if _, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, bad); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("invalid value accepted: %v", err)
		}
	}
}

func TestPublicPageInheritsAccountTheme(t *testing.T) {
	svc, _, _ := newPublicPageService(t)
	svc.SetAccountThemes(stubAccountThemes{1: "elegant_serif", 2: "not_a_theme"})
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))

	page, _ := svc.GetPublicPage(ctx, owner, prof.UUID)
	if page.Theme != "" || page.InheritedTheme != "elegant_serif" {
		t.Fatalf("settings: %+v", page)
	}
	public, _ := svc.PublicProfile(ctx, prof.Username)
	if public.Theme != "elegant_serif" {
		t.Fatalf("inherited theme: %q", public.Theme)
	}
	// The profile's own theme wins over the company's.
	own := "glass"
	if _, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{Theme: &own}); err != nil {
		t.Fatal(err)
	}
	public, _ = svc.PublicProfile(ctx, prof.Username)
	if public.Theme != "glass" {
		t.Fatalf("override: %q", public.Theme)
	}
	// An unknown company theme is ignored.
	other := principalIn(2, authctx.AccountRoleOwner)
	p2, _ := svc.Create(ctx, other, profileInput("Alan", "Turing"))
	if public, _ := svc.PublicProfile(ctx, p2.Username); public.Theme != "" {
		t.Fatalf("stale company theme leaked: %q", public.Theme)
	}
}

func ptr(s string) *string { return &s }

// TEC-946: unlocked company branding is the default of an employee profile, locked branding
// overrides the profile's own theme, layout and cover; logo and color always come through.
func TestPublicPageCompanyBrandingLock(t *testing.T) {
	svc, _, _ := newPublicPageService(t)
	brand := &model.AccountBranding{
		Theme: "elegant_serif", LayoutMode: "minimal", PrimaryColor: "#112233",
		LogoURL: "/api/v1/storage/stream/accounts/a/media/logo.png",
		CoverURL: "/api/v1/storage/stream/accounts/a/media/cover.png",
	}
	svc.SetAccountThemes(stubBranding{1: brand})
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))

	// Unlocked, nothing set on the profile: the company look is the default.
	public, _ := svc.PublicProfile(ctx, prof.Username)
	if public.Theme != "elegant_serif" || public.LayoutMode != "minimal" || public.CoverURL != brand.CoverURL ||
		public.PrimaryColor != "#112233" || public.CompanyLogoURL != brand.LogoURL {
		t.Fatalf("unlocked default: %+v", public)
	}
	// Unlocked, the profile's own choices win.
	theme, layout := "glass", "compact"
	if _, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{Theme: &theme, LayoutMode: &layout}); err != nil {
		t.Fatal(err)
	}
	public, _ = svc.PublicProfile(ctx, prof.Username)
	if public.Theme != "glass" || public.LayoutMode != "compact" {
		t.Fatalf("unlocked override: %+v", public)
	}
	// Locked, the company look is forced.
	brand.Locked = true
	public, _ = svc.PublicProfile(ctx, prof.Username)
	if public.Theme != "elegant_serif" || public.LayoutMode != "minimal" || public.CoverURL != brand.CoverURL {
		t.Fatalf("locked: %+v", public)
	}
	page, _ := svc.GetPublicPage(ctx, owner, prof.UUID)
	if !page.BrandingLocked || page.Theme != "glass" || page.InheritedTheme != "elegant_serif" || page.InheritedLayoutMode != "minimal" {
		t.Fatalf("locked settings: %+v", page)
	}
	// Locked fields the company left unset fall back to the profile's own.
	brand.LayoutMode = ""
	public, _ = svc.PublicProfile(ctx, prof.Username)
	if public.Theme != "elegant_serif" || public.LayoutMode != "compact" {
		t.Fatalf("locked fallback: %+v", public)
	}
	// Another account never sees this branding.
	other := principalIn(2, authctx.AccountRoleOwner)
	p2, _ := svc.Create(ctx, other, profileInput("Alan", "Turing"))
	if public, _ := svc.PublicProfile(ctx, p2.Username); public.Theme != "" || public.CompanyLogoURL != "" || public.PrimaryColor != "" {
		t.Fatalf("branding leaked across accounts: %+v", public)
	}
}

func TestPublicPageContactExchange(t *testing.T) {
	svc, _, _ := newPublicPageService(t)
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))
	on, off := true, false

	page, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{ContactExchange: &on, LeadCapture: &on})
	if err != nil || !page.ContactExchangeEnabled || !page.LeadCaptureEnabled {
		t.Fatalf("enable exchange: %+v %v", page, err)
	}
	// The two switches are independent, and an unrelated update keeps both.
	page, _ = svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{LeadCapture: &off})
	if !page.ContactExchangeEnabled || page.LeadCaptureEnabled {
		t.Fatalf("lead capture switch touched exchange: %+v", page)
	}
	page, _ = svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{ContactExchange: &off})
	if page.ContactExchangeEnabled {
		t.Fatal("exchange not disabled")
	}
}

type stubLinks map[string]*routesmodel.LinkPresentation

func (s stubLinks) SmartLinkPresentation(_ context.Context, via string) (*routesmodel.LinkPresentation, error) {
	return s[via], nil
}

// A smart link opening the profile (?via=s/{slug}) restyles it and can only hide modules (TEC-556).
func TestPublicProfileViaSmartLink(t *testing.T) {
	svc, _, repo := newPublicPageService(t)
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))
	d := repo.profiles[prof.UUID]
	d.Translations[0].Bio = "Mathematician"
	d.ContactDetails = []model.ContactDetailDTO{{Category: "phone", Value: "+90"}}
	d.SocialLinks = []model.SocialLinkDTO{{Platform: "x", URL: "https://x.com/ada", IsActive: true}}
	// The owner hides contacts on the page itself.
	mods := []string{"bio", "social_links"}
	if _, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{VisibleModules: &mods}); err != nil {
		t.Fatal(err)
	}
	svc.SetLinkPresentations(stubLinks{
		"s/promo": {ProfileUUID: prof.UUID, Theme: "glass", LayoutMode: "minimal", PrimaryAction: "call",
			VisibleModules: []string{"contacts", "social_links"}},
		"s/hub":   {ProfileUUID: prof.UUID, SocialHub: true},
		"s/other": {ProfileUUID: uuid.New(), Theme: "glass"},
	})

	no := false
	svc.SetFeatures(stubFeatures{premium: &no})
	public, err := svc.PublicProfileVia(ctx, prof.Username, "s/promo")
	if err != nil {
		t.Fatal(err)
	}
	// Without design.premium the link's premium theme renders as the basic one (TEC-947).
	if public.Theme != "dark_minimal" {
		t.Fatalf("premium fallback after the link theme: %q", public.Theme)
	}
	yes := true
	svc.SetFeatures(stubFeatures{premium: &yes})
	public, _ = svc.PublicProfileVia(ctx, prof.Username, "s/promo")
	if public.Theme != "glass" || public.LayoutMode != "minimal" || public.PrimaryAction != "call" {
		t.Fatalf("link presentation not applied: %+v", public)
	}
	if public.Translations[0].Bio != "" || len(public.Contacts) != 0 || len(public.SocialLinks) != 1 {
		t.Fatalf("link must only narrow the owner's modules: %+v", public)
	}

	hub, _ := svc.PublicProfileVia(ctx, prof.Username, "s/hub")
	if hub.Translations[0].Bio != "" || len(hub.SocialLinks) != 1 || strings.Join(hub.VisibleModules, ",") != "social_links" {
		t.Fatalf("social hub shows social links only: %+v", hub)
	}

	// A link of another profile, or none, leaves the page as the owner set it.
	for _, via := range []string{"s/other", "s/unknown", ""} {
		plain, _ := svc.PublicProfileVia(ctx, prof.Username, via)
		if plain.Theme != "" || plain.PrimaryAction != "" || plain.Translations[0].Bio != "Mathematician" {
			t.Fatalf("%q changed the page: %+v", via, plain)
		}
	}
}

const (
	solidDoc  = `{"kind":"solid","params":{"color":"#000"}}`
	shaderDoc = `{"kind":"shader","shaderType":"mesh-gradient","params":{"colors":["#123456"]}}`
)

// TEC-947: premium themes and animated backgrounds need design.premium to be picked.
func TestPremiumPickNeedsEntitlement(t *testing.T) {
	svc, _, _ := newPublicPageService(t)
	premium := false
	svc.SetFeatures(stubFeatures{premium: &premium})
	animated, static := uuid.New(), uuid.New()
	svc.SetBackgrounds(stubBackgrounds{docs: map[uuid.UUID][]byte{animated: []byte(shaderDoc), static: []byte(solidDoc)}})
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))

	for name, in := range map[string]model.UpdatePublicPageInput{
		"premium theme":       {Theme: ptr("glass")},
		"animated background": {Background: model.OptionalUUID{Set: true, Value: &animated}},
	} {
		_, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, in)
		var fe *entitlement.FeatureError
		if !errors.Is(err, entitlement.ErrPlanLimitExceeded) || !errors.As(err, &fe) || fe.Feature != entitlement.FeatureDesignPremium {
			t.Errorf("%s without design.premium: %v", name, err)
		}
	}
	page, _ := svc.GetPublicPage(ctx, owner, prof.UUID)
	if page.Theme != "" || page.BackgroundUUID != nil {
		t.Fatalf("a refused pick was stored: %+v", page)
	}
	// Basic themes and static backgrounds stay free.
	page, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{
		Theme: ptr("compact_card"), Background: model.OptionalUUID{Set: true, Value: &static},
	})
	if err != nil || page.Theme != "compact_card" || page.BackgroundUUID == nil {
		t.Fatalf("basic pick: %+v %v", page, err)
	}
	// Super admins bypass plan limits.
	admin := owner
	admin.IsSuperAdmin = true
	if _, err := svc.UpdatePublicPage(authctx.WithPrincipal(ctx, admin), admin, prof.UUID, model.UpdatePublicPageInput{Theme: ptr("glass")}); err != nil {
		t.Fatalf("super admin: %v", err)
	}
}

// TEC-947: after a downgrade the public page renders the basic theme and background, the saved
// choice stays, other settings can still be saved, and an upgrade brings the choice back.
func TestDowngradeRendersBasicVisuals(t *testing.T) {
	svc, _, _ := newPublicPageService(t)
	premium := true
	svc.SetFeatures(stubFeatures{premium: &premium})
	animated := uuid.New()
	svc.SetBackgrounds(stubBackgrounds{docs: map[uuid.UUID][]byte{animated: []byte(shaderDoc)}})
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))

	if _, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{
		Theme: ptr("glass"), Background: model.OptionalUUID{Set: true, Value: &animated},
	}); err != nil {
		t.Fatal(err)
	}
	public, _ := svc.PublicProfile(ctx, prof.Username)
	if public.Theme != "glass" || len(public.Background) == 0 || public.BackgroundThumbnailURL == "" {
		t.Fatalf("paid plan: %+v", public)
	}

	premium = false // the subscription ended
	public, _ = svc.PublicProfile(ctx, prof.Username)
	if public.Theme != model.PremiumThemes["glass"] || public.Background != nil || public.BackgroundThumbnailURL != "" {
		t.Fatalf("downgraded page: theme %q background %s", public.Theme, public.Background)
	}
	// The form re-sends the kept choice with every save; that must not be refused.
	off := false
	page, err := svc.UpdatePublicPage(ctx, owner, prof.UUID, model.UpdatePublicPageInput{
		LeadCapture: &off,
		Theme: ptr("glass"), Background: model.OptionalUUID{Set: true, Value: &animated},
	})
	if err != nil || page.Theme != "glass" || page.BackgroundUUID == nil || *page.BackgroundUUID != animated {
		t.Fatalf("kept choice: %+v %v", page, err)
	}

	premium = true // upgraded again
	public, _ = svc.PublicProfile(ctx, prof.Username)
	if public.Theme != "glass" || !strings.Contains(string(public.Background), "mesh-gradient") {
		t.Fatalf("after upgrade: %+v", public)
	}
}

// A premium company theme inherited by an employee profile also falls back.
func TestDowngradeInheritedPremiumTheme(t *testing.T) {
	svc, _, _ := newPublicPageService(t)
	premium := false
	svc.SetFeatures(stubFeatures{premium: &premium})
	svc.SetAccountThemes(stubAccountThemes{1: "elegant_serif"})
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	prof, _ := svc.Create(ctx, owner, profileInput("Ada", "Lovelace"))
	public, _ := svc.PublicProfile(ctx, prof.Username)
	if public.Theme != "classic_light" {
		t.Fatalf("inherited premium theme without design.premium: %q", public.Theme)
	}
}
