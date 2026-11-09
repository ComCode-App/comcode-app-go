package usecase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	bulkusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk/usecase"
	brandmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/model"
	brandingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/usecase"
	importusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports/usecase"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	profilesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type allowQuotas struct{}

func (allowQuotas) CheckQuota(context.Context, int64, bool, string) error { return nil }

// fakeMedia is an object store that records uploads and deletes; URLs use the stream proxy path.
type fakeMedia struct {
	mu      sync.Mutex
	objects map[string]bool
}

func newFakeMedia() *fakeMedia { return &fakeMedia{objects: map[string]bool{}} }

func (m *fakeMedia) Upload(_ context.Context, _ storage.File, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[path] = true
	return nil
}

func (m *fakeMedia) Delete(_ context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, path)
	return nil
}

func (m *fakeMedia) PublicURL(path string) string { return "/api/v1/storage/stream/" + path }

func (m *fakeMedia) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.objects)
}

// memDriver serves import uploads from memory; other storage calls are not used by imports.
type memDriver struct {
	storage.Driver
	m *storage.Memory
}

func (d memDriver) Upload(ctx context.Context, f storage.File, path string) error {
	return d.m.Upload(ctx, f, path)
}

func (d memDriver) Download(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	return d.m.Download(ctx, path)
}

type env struct {
	t        *testing.T
	d        *pgtest.DB
	ctx      context.Context
	profiles *profilesusecase.Service
	media    *fakeMedia
	branding *brandingusecase.Service
	bulk     *bulkusecase.Service
	imports  *importusecase.Service
}

func newEnv(t *testing.T) *env {
	d := pgtest.Open(t)
	repo := profilesrepo.NewPostgres(d.Pool, d.Queries)
	profiles := profilesusecase.NewService(repo, nil)
	profiles.SetQuotas(allowQuotas{})
	profiles.SetPublicPages(repo)
	profiles.SetFeatures(entitlement.New(d.Queries))
	media := newFakeMedia()
	branding := brandingusecase.New(d.Queries, media)
	profiles.SetAccountThemes(branding)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bulkAdapter := usecase.NewBulkAdapter(d.Queries, profiles, media, branding)
	bulkAdapter.SetFeatures(entitlement.New(d.Queries))
	bulkReg := bulkengine.NewRegistry(bulkAdapter)
	ioReg := ioengine.NewRegistry(usecase.NewImportAdapter(d.Queries, profiles))
	return &env{
		t: t, d: d, ctx: context.Background(), profiles: profiles, media: media, branding: branding,
		bulk:    bulkusecase.New(d.Queries, bulkReg, nil, nil, nil, config.BulkConfig{SyncMax: 50, RollbackHours: 24}, log),
		imports: importusecase.New(d.Queries, memDriver{m: storage.NewMemory()}, ioReg, nil, nil, nil, log),
	}
}

type company struct {
	account db.Account
	owner   pgtest.User
}

func (e *env) company(name string) company {
	e.t.Helper()
	owner := e.d.CreateUser(e.t)
	a, err := e.d.Queries.CreateCompanyAccount(e.ctx, db.CreateCompanyAccountParams{Name: name, Slug: "bulk-" + pgtest.Unique()})
	if err != nil {
		e.t.Fatal(err)
	}
	e.addMember(a, owner, db.AccountRoleOwner)
	return company{account: a, owner: owner}
}

func (e *env) addMember(a db.Account, u pgtest.User, role db.AccountRole) {
	e.t.Helper()
	if err := e.d.Queries.AddAccountMember(e.ctx, db.AddAccountMemberParams{AccountID: a.ID, UserID: u.ID, Role: role}); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) office(a db.Account, name, city, address, phone string) db.CompanyOffice {
	e.t.Helper()
	o, err := e.d.Queries.CreateStructureOffice(e.ctx, db.CreateStructureOfficeParams{AccountID: a.ID, Name: name, City: city, Address: address, Phone: phone})
	if err != nil {
		e.t.Fatal(err)
	}
	return o
}

func (e *env) group(a db.Account, name string, office *uuid.UUID) db.CompanyGroup {
	e.t.Helper()
	off := pgtype.UUID{}
	if office != nil {
		off = pgtype.UUID{Bytes: *office, Valid: true}
	}
	g, err := e.d.Queries.CreateStructureGroup(e.ctx, db.CreateStructureGroupParams{AccountID: a.ID, Name: name, Kind: "team", OfficeUuid: off})
	if err != nil {
		e.t.Fatal(err)
	}
	return g
}

func (e *env) assign(a db.Account, u pgtest.User, office uuid.UUID) {
	e.t.Helper()
	if _, err := e.d.Queries.SetStructureAssignment(e.ctx, db.SetStructureAssignmentParams{
		AccountID: a.ID, UserUuid: u.UUID, OfficeUuid: pgtype.UUID{Bytes: office, Valid: true},
	}); err != nil {
		e.t.Fatal(err)
	}
}

func principal(a db.Account, u pgtest.User, role string) authctx.Principal {
	return authctx.Principal{
		UserID: u.UUID, UserInternal: u.ID,
		Permissions: []string{rbac.PermAppProfilesRead, rbac.PermAppProfilesWrite, rbac.PermAppCompanyProfilesBulk, rbac.PermAppCompanyProfilesImport},
		Account:     authctx.Account{ID: a.ID, UUID: a.Uuid, Type: authctx.AccountTypeCompany, Role: role},
	}
}

// employee adds a member with a linked profile (work email, a personal phone, one link).
func (e *env) employee(c company, role db.AccountRole, first string) (pgtest.User, uuid.UUID) {
	e.t.Helper()
	u := e.d.CreateUser(e.t)
	e.addMember(c.account, u, role)
	p, err := e.profiles.Create(e.ctx, principal(c.account, c.owner, "owner"), pmodel.UpsertProfileInput{
		PrimaryLocale: "tr",
		Translations: []pmodel.UpsertTranslationInput{
			{Locale: "tr", FirstName: first, LastName: "Test", Title: "Engineer", IsDefault: true},
			{Locale: "en", FirstName: first, LastName: "Test", Title: "Engineer"},
		},
		Addresses:      []pmodel.CreateAddressInput{{Type: "home", City: "Ankara", IsPrimary: true}},
		ContactDetails: []pmodel.CreateContactDetailInput{{Category: "phone", Type: "mobile", Value: "+90 500 000 00 01", IsPrimary: true}},
		SocialLinks:    []pmodel.CreateSocialLinkInput{{Platform: "custom", Label: "Blog", URL: "https://blog.example.test", IsActive: true}},
	})
	if err != nil {
		e.t.Fatal(err)
	}
	member, err := e.d.Queries.GetCompanyMemberByUserUUID(e.ctx, db.GetCompanyMemberByUserUUIDParams{AccountID: c.account.ID, UserUuid: u.UUID})
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.d.Queries.SetCompanyMemberProfile(e.ctx, db.SetCompanyMemberProfileParams{AccountID: c.account.ID, MemberID: member.MemberID, ProfileUuid: p.UUID}); err != nil {
		e.t.Fatal(err)
	}
	return u, p.UUID
}

func (e *env) detail(c company, id uuid.UUID) *pmodel.ProfileDetail {
	e.t.Helper()
	d, err := e.profiles.Get(e.ctx, principal(c.account, c.owner, "owner"), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return d
}

// content is the comparable part of a profile (list uuids change on every update).
func content(d *pmodel.ProfileDetail) string {
	type view struct {
		Avatar, Cover string
		T             []pmodel.ProfileTranslationDTO
		A             []pmodel.AddressDTO
		C             []pmodel.ContactDetailDTO
		S             []pmodel.SocialLinkDTO
	}
	v := view{Avatar: d.AvatarURL, Cover: d.CoverURL, T: d.Translations}
	for _, a := range d.Addresses {
		a.UUID = uuid.Nil
		v.A = append(v.A, a)
	}
	for _, c := range d.ContactDetails {
		c.UUID = uuid.Nil
		v.C = append(v.C, c)
	}
	for _, s := range d.SocialLinks {
		s.UUID = uuid.Nil
		v.S = append(v.S, s)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func (e *env) run(p authctx.Principal, action string, target bulkengine.BulkTarget, params any) bulkusecase.BulkJobView {
	e.t.Helper()
	raw, _ := json.Marshal(params)
	res, err := e.bulk.Execute(e.ctx, p.UserInternal, bulkusecase.ExecuteInput{
		Resource: model.ResourceCompanyProfiles, Action: action, Target: target, Locale: "tr",
		AccountID: p.Account.ID, Params: raw,
	})
	if err != nil {
		e.t.Fatalf("execute %s: %v", action, err)
	}
	if !res.Persisted || res.Job.Status != "completed" {
		e.t.Fatalf("execute %s: job %+v", action, res.Job)
	}
	return res.Job
}

// TEC-947: a premium theme can be bulk-applied only on a plan with design.premium.
func TestIntegrationBulkPremiumTheme(t *testing.T) {
	e := newEnv(t)
	c := e.company("Premium")
	_, pa := e.employee(c, db.AccountRoleMember, "Ayla")
	owner := principal(c.account, c.owner, "owner")
	raw, _ := json.Marshal(model.ThemeParams{Theme: strp("glass")})
	in := bulkusecase.ExecuteInput{
		Resource: model.ResourceCompanyProfiles, Action: model.ActionSetTheme, Target: ids(pa), Locale: "tr",
		AccountID: c.account.ID, Params: raw,
	}
	if _, err := e.bulk.Execute(e.ctx, owner.UserInternal, in); !errors.Is(err, bulkusecase.ErrInvalidRequest) || !strings.Contains(err.Error(), "design.premium") {
		t.Fatalf("free plan, premium theme: %v", err)
	}
	if _, err := e.d.Pool.Exec(e.ctx, `
		INSERT INTO subscriptions (account_id, plan_id, status, source, starts_at, next_billing_at)
		SELECT $1, id, 'active', 'manual', now(), now() + interval '30 days' FROM plans WHERE slug = 'pro-monthly'`,
		c.account.ID); err != nil {
		t.Fatal(err)
	}
	job := e.run(owner, model.ActionSetTheme, ids(pa), model.ThemeParams{Theme: strp("glass")})
	if job.Summary.Succeeded != 1 {
		t.Fatalf("paid plan, premium theme: %+v", job.Summary)
	}
}

func ids(v ...uuid.UUID) bulkengine.BulkTarget {
	out := make([]string, 0, len(v))
	for _, id := range v {
		out = append(out, id.String())
	}
	return bulkengine.BulkTarget{Scope: "ids", IDs: out}
}

func strp(s string) *string { return &s }

// pngBytes is a 1x1 PNG.
var pngBytes = func() []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	return buf.Bytes()
}()

func TestIntegrationBulkApplyAndRollback(t *testing.T) {
	e := newEnv(t)
	c := e.company("Acme")
	owner := principal(c.account, c.owner, "owner")
	hq := e.office(c.account, "HQ", "İstanbul", "Büyükdere Cd. 1", "+90 212 000 00 00")
	ua, pa := e.employee(c, db.AccountRoleMember, "Ada")
	_, pb := e.employee(c, db.AccountRoleMember, "Bora")
	e.assign(c.account, ua, hq.Uuid)
	slug := "lnk" + pgtest.Unique()
	if _, err := e.d.Pool.Exec(e.ctx, `INSERT INTO social_platforms (name, slug, icon_url) VALUES ('LinkedIn test', $1, 'https://icons.test/li.svg')`, slug); err != nil {
		t.Fatal(err)
	}
	if _, err := e.d.Pool.Exec(e.ctx, `UPDATE profiles SET avatar_url = $2, cover_url = $3 WHERE uuid = $1`,
		pa, "/api/v1/storage/stream/profiles/"+pa.String()+"/avatar.png", "/api/v1/storage/stream/profiles/"+pa.String()+"/cover.png"); err != nil {
		t.Fatal(err)
	}
	beforeA, beforeB := content(e.detail(c, pa)), content(e.detail(c, pb))

	contact := e.run(owner, model.ActionSetContact, ids(pa, pb), model.ContactParams{
		CompanyName: strp("Acme A.Ş."), UseOfficeAddress: true, Email: strp("info@acme.test"), Website: strp("https://acme.test"),
	})
	if contact.Summary.Succeeded != 1 || contact.Summary.Failed != 1 {
		t.Fatalf("office address needs an office: %+v", contact.Summary)
	}
	contact = e.run(owner, model.ActionSetContact, ids(pa, pb), model.ContactParams{
		CompanyName: strp("Acme A.Ş."), Address: &model.AddressParams{Street: "Büyükdere Cd. 1", City: "İstanbul", Country: "TR"},
		Phone: strp("+90 212 000 00 00"), Website: strp("https://acme.test"),
	})
	theme := e.run(owner, model.ActionSetTheme, ids(pa, pb), model.ThemeParams{Theme: strp("dark_minimal"), LayoutMode: strp("compact")})
	links := e.run(owner, model.ActionAddLinks, ids(pa, pb), model.LinksParams{Links: []model.LinkParams{
		{Platform: slug, Label: "Acme", URL: "https://linkedin.test/acme"},
		{Platform: "custom", Label: "Blog", URL: "https://blog.example.test"},
	}})
	// The logo action references the company branding logo (TEC-946); nothing is copied.
	brander := owner
	brander.Permissions = append(brander.Permissions, rbac.PermAppCompanyBrandingRead, rbac.PermAppCompanyBrandingWrite)
	raw, _ := json.Marshal(model.LogoParams{Source: model.BrandingLogo, TargetField: "avatar"})
	if _, err := e.bulk.Execute(e.ctx, c.owner.ID, bulkusecase.ExecuteInput{
		Resource: model.ResourceCompanyProfiles, Action: model.ActionSetLogo, Target: ids(pa), AccountID: c.account.ID, Params: raw,
	}); !errors.Is(err, bulkusecase.ErrInvalidRequest) {
		t.Fatalf("logo without a branding logo: %v", err)
	}
	logoFile, err := e.branding.Upload(e.ctx, brander, brandmodel.UploadInput{Kind: brandmodel.KindLogo, Name: "Logo", Filename: "logo.png", Size: int64(len(pngBytes))}, bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.branding.UpdateBranding(e.ctx, brander, brandmodel.UpdateBrandingInput{LogoMediaUUID: &logoFile.UUID}); err != nil {
		t.Fatal(err)
	}
	logo := e.run(owner, model.ActionSetLogo, ids(pa, pb), model.LogoParams{Source: model.BrandingLogo, TargetField: "avatar"})
	for _, j := range []bulkusecase.BulkJobView{contact, theme, links, logo} {
		if j.Summary.Succeeded != 2 || j.Summary.Failed != 0 || j.RollbackUntil == nil {
			t.Fatalf("%s: %+v", j.Action, j.Summary)
		}
	}

	b := e.detail(c, pb)
	if b.Translations[0].CompanyName != "Acme A.Ş." || b.Translations[1].CompanyName != "Acme A.Ş." {
		t.Fatalf("company name not set: %+v", b.Translations)
	}
	if len(b.Addresses) != 2 || b.Addresses[0].Type != "work" || !b.Addresses[0].IsPrimary || b.Addresses[1].IsPrimary {
		t.Fatalf("work address: %+v", b.Addresses)
	}
	if len(b.SocialLinks) != 2 || b.SocialLinks[1].PlatformID == nil {
		t.Fatalf("links (duplicate URL skipped, platform link added): %+v", b.SocialLinks)
	}
	if b.AvatarURL != logoFile.URL || e.media.count() != 1 {
		t.Fatalf("logo not referenced from the library: %q (%d objects)", b.AvatarURL, e.media.count())
	}
	files, _, err := e.branding.List(e.ctx, brander, brandmodel.Filter{})
	if err != nil || len(files) != 1 || len(files[0].Usage) != 3 {
		t.Fatalf("library usage (branding + 2 avatars): %+v %v", files, err)
	}
	page, err := e.profiles.GetPublicPage(e.ctx, owner, pb)
	if err != nil || page.Theme != "dark_minimal" || page.LayoutMode != "compact" {
		t.Fatalf("theme: %+v %v", page, err)
	}

	for _, j := range []bulkusecase.BulkJobView{logo, links, theme, contact} {
		rolled, err := e.bulk.Rollback(e.ctx, j.UUID, c.owner.ID, bulkusecase.JobAccess{AccountID: c.account.ID, AnyActor: true})
		if err != nil || rolled.Status != "rolled_back" {
			t.Fatalf("rollback %s: %+v %v", j.Action, rolled, err)
		}
	}
	// The first contact job changed profile A only; undo it last.
	first, _, err := e.bulk.ListAccountJobs(e.ctx, c.owner.ID, bulkusecase.JobAccess{AccountID: c.account.ID, AnyActor: true}, model.ResourceCompanyProfiles, 10, 0)
	if err != nil || len(first) != 5 {
		t.Fatalf("jobs: %d %v", len(first), err)
	}
	if _, err := e.bulk.Rollback(e.ctx, first[4].UUID, c.owner.ID, bulkusecase.JobAccess{AccountID: c.account.ID, AnyActor: true}); err != nil {
		t.Fatal(err)
	}
	if got := content(e.detail(c, pa)); got != beforeA {
		t.Fatalf("profile A not restored:\n got %s\nwant %s", got, beforeA)
	}
	if got := content(e.detail(c, pb)); got != beforeB {
		t.Fatalf("profile B not restored:\n got %s\nwant %s", got, beforeB)
	}
	if page, _ := e.profiles.GetPublicPage(e.ctx, owner, pb); page.Theme != "" || page.LayoutMode != "" {
		t.Fatalf("theme not restored: %+v", page)
	}
	if e.media.count() != 1 {
		t.Fatalf("rollback touched the library file: %d", e.media.count())
	}
	if _, err := e.bulk.Rollback(e.ctx, logo.UUID, c.owner.ID, bulkusecase.JobAccess{AccountID: c.account.ID, AnyActor: true}); !errors.Is(err, bulkusecase.ErrInvalidRequest) {
		t.Fatalf("second rollback: %v", err)
	}

	// Invalid parameters are rejected before any profile is touched.
	raw, _ = json.Marshal(model.ThemeParams{Theme: strp("neon")})
	if _, err := e.bulk.Execute(e.ctx, c.owner.ID, bulkusecase.ExecuteInput{
		Resource: model.ResourceCompanyProfiles, Action: model.ActionSetTheme, Target: ids(pa), AccountID: c.account.ID, Params: raw,
	}); !errors.Is(err, bulkusecase.ErrInvalidRequest) {
		t.Fatalf("unknown theme: %v", err)
	}
}

func TestIntegrationBulkIsolationAndOfficeScope(t *testing.T) {
	e := newEnv(t)
	c := e.company("Scoped")
	other := e.company("Other")
	x := e.office(c.account, "X", "", "", "")
	y := e.office(c.account, "Y", "", "", "")
	ux, px := e.employee(c, db.AccountRoleMember, "Xena")
	uy, py := e.employee(c, db.AccountRoleMember, "Yusuf")
	e.assign(c.account, ux, x.Uuid)
	e.assign(c.account, uy, y.Uuid)
	_, foreign := e.employee(other, db.AccountRoleMember, "Foreign")
	mgrUser := e.d.CreateUser(t)
	e.addMember(c.account, mgrUser, db.AccountRoleOfficeManager)
	if _, err := e.d.Queries.AddStructureScope(e.ctx, db.AddStructureScopeParams{AccountID: c.account.ID, Uuid: mgrUser.UUID, Uuid_2: x.Uuid}); err != nil {
		t.Fatal(err)
	}
	mgr := principal(c.account, mgrUser, "office_manager")
	theme := model.ThemeParams{Theme: strp("compact_card")}

	// Another company's owner reaches none of this company's profiles, and vice versa.
	job := e.run(principal(other.account, other.owner, "owner"), model.ActionSetTheme, ids(px, foreign), theme)
	if job.Summary.Succeeded != 1 || job.Summary.Failed != 1 || job.Summary.Errors[0].EntityUUID != px.String() {
		t.Fatalf("cross-account ids: %+v", job.Summary)
	}
	if page, _ := e.profiles.GetPublicPage(e.ctx, principal(c.account, c.owner, "owner"), px); page.Theme != "" {
		t.Fatalf("foreign job changed profile: %q", page.Theme)
	}
	if _, err := e.bulk.GetAccountJob(e.ctx, job.UUID, c.owner.ID, bulkusecase.JobAccess{AccountID: c.account.ID, AnyActor: true}); !errors.Is(err, bulkusecase.ErrNotFound) {
		t.Fatalf("job of another account: %v", err)
	}
	if _, err := e.bulk.Rollback(e.ctx, job.UUID, other.owner.ID, bulkusecase.JobAccess{}); !errors.Is(err, bulkusecase.ErrNotFound) {
		t.Fatalf("platform rollback of an app job: %v", err)
	}

	// Office manager: only profiles of members in granted offices, by ids and by query.
	job = e.run(mgr, model.ActionSetTheme, ids(px, py), theme)
	if job.Summary.Succeeded != 1 || job.Summary.Errors[0].EntityUUID != py.String() {
		t.Fatalf("office manager ids: %+v", job.Summary)
	}
	job = e.run(mgr, model.ActionSetTheme, bulkengine.BulkTarget{Scope: "query", Query: map[string]string{}}, model.ThemeParams{Theme: strp("classic_light")})
	if job.Summary.Total != 1 || job.Summary.Succeeded != 1 {
		t.Fatalf("office manager query: %+v", job.Summary)
	}
	if _, err := e.bulk.Rollback(e.ctx, job.UUID, mgrUser.ID, bulkusecase.JobAccess{AccountID: c.account.ID}); err != nil {
		t.Fatalf("manager rollback: %v", err)
	}
	ownerJob := e.run(principal(c.account, c.owner, "owner"), model.ActionSetTheme, bulkengine.BulkTarget{Scope: "query", Query: map[string]string{"office_uuid": y.Uuid.String()}}, theme)
	if ownerJob.Summary.Total != 1 || ownerJob.Summary.Succeeded != 1 {
		t.Fatalf("owner query by office: %+v", ownerJob.Summary)
	}
	// Office managers see and roll back only their own jobs.
	if _, err := e.bulk.Rollback(e.ctx, ownerJob.UUID, mgrUser.ID, bulkusecase.JobAccess{AccountID: c.account.ID}); !errors.Is(err, bulkusecase.ErrNotFound) {
		t.Fatalf("manager rolled back owner's job: %v", err)
	}

	// Losing the grant takes effect immediately, also for rollback.
	job = e.run(mgr, model.ActionSetTheme, ids(px), theme)
	if err := e.d.Queries.ClearStructureScopes(e.ctx, db.ClearStructureScopesParams{AccountID: c.account.ID, Uuid: mgrUser.UUID}); err != nil {
		t.Fatal(err)
	}
	rolled, err := e.bulk.Rollback(e.ctx, job.UUID, mgrUser.ID, bulkusecase.JobAccess{AccountID: c.account.ID})
	if err != nil || rolled.Status != "rolled_back_partial" {
		t.Fatalf("revoked manager rollback: %+v %v", rolled, err)
	}

	// Plain members and personal accounts cannot run bulk actions at all.
	plainUser := e.d.CreateUser(t)
	e.addMember(c.account, plainUser, db.AccountRoleMember)
	raw, _ := json.Marshal(theme)
	if _, err := e.bulk.Execute(e.ctx, plainUser.ID, bulkusecase.ExecuteInput{
		Resource: model.ResourceCompanyProfiles, Action: model.ActionSetTheme, Target: ids(px), AccountID: c.account.ID, Params: raw,
	}); !errors.Is(err, bulkusecase.ErrInvalidRequest) {
		t.Fatalf("plain member: %v", err)
	}
	if _, err := e.bulk.Execute(e.ctx, plainUser.ID, bulkusecase.ExecuteInput{
		Resource: model.ResourceCompanyProfiles, Action: model.ActionSetTheme, Target: ids(px), AccountID: plainUser.AccountID, Params: raw,
	}); !errors.Is(err, bulkusecase.ErrInvalidRequest) {
		t.Fatalf("personal account: %v", err)
	}
	if _, err := e.bulk.Execute(e.ctx, c.owner.ID, bulkusecase.ExecuteInput{
		Resource: model.ResourceCompanyProfiles, Action: model.ActionSetTheme, Target: ids(px), Params: raw,
	}); !errors.Is(err, bulkusecase.ErrInvalidRequest) {
		t.Fatalf("no account: %v", err)
	}
}

func TestIntegrationMemberProfilesAndLink(t *testing.T) {
	e := newEnv(t)
	c := e.company("Links")
	other := e.company("Elsewhere")
	svc := usecase.NewService(e.d.Queries)
	owner := principal(c.account, c.owner, "owner")
	x := e.office(c.account, "X", "", "", "")
	u, linked := e.employee(c, db.AccountRoleMember, "Linked")
	e.assign(c.account, u, x.Uuid)
	loose := e.d.CreateUser(t)
	e.addMember(c.account, loose, db.AccountRoleMember)
	_, foreign := e.employee(other, db.AccountRoleMember, "Foreign")

	items, total, err := svc.MemberProfiles(e.ctx, owner, model.MemberProfileFilter{OfficeUUID: &x.Uuid})
	if err != nil || total != 1 || items[0].Profile == nil || items[0].Profile.UUID != linked || items[0].Profile.FirstName != "Linked" {
		t.Fatalf("by office: %+v %d %v", items, total, err)
	}
	no := false
	if _, total, _ = svc.MemberProfiles(e.ctx, owner, model.MemberProfileFilter{HasProfile: &no}); total != 2 {
		t.Fatalf("members without a profile (owner, loose): %d", total)
	}
	if err := svc.LinkProfile(e.ctx, owner, loose.UUID, &linked); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("profile linked twice: %v", err)
	}
	if err := svc.LinkProfile(e.ctx, owner, loose.UUID, &foreign); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("foreign profile: %v", err)
	}
	if err := svc.LinkProfile(e.ctx, principal(c.account, loose, "member"), u.UUID, nil); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("member unlinks: %v", err)
	}
	if err := svc.LinkProfile(e.ctx, owner, u.UUID, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.LinkProfile(e.ctx, owner, loose.UUID, &linked); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationEmployeeImport(t *testing.T) {
	e := newEnv(t)
	c := e.company("Importer")
	other := e.company("Bystander")
	ist := e.office(c.account, "İstanbul", "İstanbul", "", "")
	ank := e.office(c.account, "Ankara", "Ankara", "", "")
	e.group(c.account, "Satış", &ist.Uuid)
	e.group(c.account, "Satış", &ank.Uuid)
	e.group(c.account, "Yönetim", nil)
	existing, existingProfile := e.employee(c, db.AccountRoleMember, "Eski")
	e.assign(c.account, existing, ank.Uuid)
	newcomer := e.d.CreateUser(t)
	var newcomerEmail, existingEmail string
	_ = e.d.Pool.QueryRow(e.ctx, `SELECT email FROM users WHERE id = $1`, newcomer.ID).Scan(&newcomerEmail)
	_ = e.d.Pool.QueryRow(e.ctx, `SELECT email FROM users WHERE id = $1`, existing.ID).Scan(&existingEmail)
	beforeExisting := content(e.detail(c, existingProfile))

	csv := "Ad,Soyad,E-posta,Unvan,Ofis,Grup,Telefon\n" +
		"Yeni,Kişi," + strings.ToUpper(newcomerEmail) + ",Satış Temsilcisi,İstanbul,Satış,+90 555 111 22 33\n" +
		"Eski,Güncel," + existingEmail + ",Müdür,,Yönetim,\n" +
		"Kayıtsız,Biri,nobody-" + pgtest.Unique() + "@example.test,,,,\n" +
		"Ofissiz,Biri," + newcomerEmail + ",,Mars,,\n"
	job, err := e.imports.Upload(e.ctx, c.owner.ID, c.account.ID, model.ResourceCompanyEmployees, ioengine.ImportCSV, "tr", "employees.csv", bytes.NewBufferString(csv))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"name", "surname", "email", "title", "office", "group", "phone"} {
		found := false
		for _, v := range job.Mapping {
			found = found || v == key
		}
		if !found {
			t.Fatalf("header for %s not suggested: %+v", key, job.Mapping)
		}
	}
	// Jobs are invisible from another account and from platform routes.
	if _, err := e.imports.Preview(e.ctx, job.UUID, other.owner.ID, other.account.ID); !errors.Is(err, importusecase.ErrNotFound) {
		t.Fatalf("other account preview: %v", err)
	}
	if _, err := e.imports.Preview(e.ctx, job.UUID, c.owner.ID, 0); !errors.Is(err, importusecase.ErrNotFound) {
		t.Fatalf("platform preview: %v", err)
	}
	if _, err := e.imports.Upload(e.ctx, c.owner.ID, 0, model.ResourceCompanyEmployees, ioengine.ImportCSV, "tr", "x.csv", bytes.NewBufferString(csv)); !errors.Is(err, importusecase.ErrInvalidRequest) {
		t.Fatalf("platform upload: %v", err)
	}

	previewed, err := e.imports.Preview(e.ctx, job.UUID, c.owner.ID, c.account.ID)
	if err != nil {
		t.Fatal(err)
	}
	ps := previewed.PreviewSummary
	if ps.Total != 4 || ps.Valid != 2 || ps.Invalid != 2 || len(ps.Rows) != 2 || ps.Rows[0].Op != "create" || ps.Rows[1].Op != "update" {
		t.Fatalf("preview: %+v", ps)
	}
	if ps.Errors[0].Index != 3 || ps.Errors[0].Field != "email" || ps.Errors[1].Index != 4 || ps.Errors[1].Field != "office" {
		t.Fatalf("preview errors: %+v", ps.Errors)
	}
	// The dry run wrote nothing.
	if _, err := e.d.Queries.GetCompanyMemberByUserID(e.ctx, db.GetCompanyMemberByUserIDParams{AccountID: c.account.ID, UserID: newcomer.ID}); err == nil {
		t.Fatal("preview added a member")
	}

	applied, err := e.imports.Confirm(e.ctx, job.UUID, c.owner.ID, c.account.ID)
	if err != nil || applied.Status != "applied" || applied.PreviewSummary.Valid != 2 || applied.PreviewSummary.Invalid != 2 {
		t.Fatalf("apply: %+v %v", applied, err)
	}
	m, err := e.d.Queries.GetCompanyMemberByUserID(e.ctx, db.GetCompanyMemberByUserIDParams{AccountID: c.account.ID, UserID: newcomer.ID})
	if err != nil || m.Role != db.AccountRoleMember {
		t.Fatalf("newcomer membership: %+v %v", m, err)
	}
	link, err := e.d.Queries.GetCompanyMemberLink(e.ctx, db.GetCompanyMemberLinkParams{AccountID: c.account.ID, MemberID: m.MemberID})
	if err != nil {
		t.Fatal(err)
	}
	created := e.detail(c, link.ProfileUuid)
	if created.Translations[0].FirstName != "Yeni" || created.Translations[0].Title != "Satış Temsilcisi" || created.Translations[0].CompanyName != "Importer" {
		t.Fatalf("created profile: %+v", created.Translations)
	}
	if len(created.ContactDetails) != 2 || created.ContactDetails[0].Value != newcomerEmail {
		t.Fatalf("created contacts: %+v", created.ContactDetails)
	}
	asg, _ := e.d.Queries.GetStructureAssignment(e.ctx, db.GetStructureAssignmentParams{AccountID: c.account.ID, Uuid: newcomer.UUID})
	if !asg.OfficeUuid.Valid || uuid.UUID(asg.OfficeUuid.Bytes) != ist.Uuid || !asg.GroupUuid.Valid {
		t.Fatalf("newcomer assignment: %+v", asg)
	}
	upd := e.detail(c, existingProfile)
	if upd.Translations[0].LastName != "Güncel" || upd.Translations[0].Title != "Müdür" {
		t.Fatalf("updated profile: %+v", upd.Translations)
	}
	asg, _ = e.d.Queries.GetStructureAssignment(e.ctx, db.GetStructureAssignmentParams{AccountID: c.account.ID, Uuid: existing.UUID})
	if !asg.OfficeUuid.Valid || uuid.UUID(asg.OfficeUuid.Bytes) != ank.Uuid || !asg.GroupUuid.Valid {
		t.Fatalf("existing member keeps office, gets company-wide group: %+v", asg)
	}

	rolled, err := e.imports.Rollback(e.ctx, job.UUID, c.owner.ID, c.account.ID)
	if err != nil || rolled.Status != "rolled_back" {
		t.Fatalf("rollback: %+v %v", rolled, err)
	}
	if _, err := e.d.Queries.GetCompanyMemberByUserID(e.ctx, db.GetCompanyMemberByUserIDParams{AccountID: c.account.ID, UserID: newcomer.ID}); err == nil {
		t.Fatal("rollback kept the added member")
	}
	if _, err := e.profiles.Get(e.ctx, principal(c.account, c.owner, "owner"), link.ProfileUuid); err == nil {
		t.Fatal("rollback kept the created profile")
	}
	if got := content(e.detail(c, existingProfile)); got != beforeExisting {
		t.Fatalf("existing profile not restored:\n got %s\nwant %s", got, beforeExisting)
	}
	asg, _ = e.d.Queries.GetStructureAssignment(e.ctx, db.GetStructureAssignmentParams{AccountID: c.account.ID, Uuid: existing.UUID})
	if asg.GroupUuid.Valid || uuid.UUID(asg.OfficeUuid.Bytes) != ank.Uuid {
		t.Fatalf("existing assignment not restored: %+v", asg)
	}

	// Office managers cannot import.
	mgrUser := e.d.CreateUser(t)
	e.addMember(c.account, mgrUser, db.AccountRoleOfficeManager)
	mjob, err := e.imports.Upload(e.ctx, mgrUser.ID, c.account.ID, model.ResourceCompanyEmployees, ioengine.ImportCSV, "tr", "m.csv", bytes.NewBufferString(csv))
	if err != nil {
		t.Fatal(err)
	}
	mp, err := e.imports.Preview(e.ctx, mjob.UUID, mgrUser.ID, c.account.ID)
	if err != nil || mp.PreviewSummary.Valid != 0 {
		t.Fatalf("office manager preview: %+v %v", mp.PreviewSummary, err)
	}
}
