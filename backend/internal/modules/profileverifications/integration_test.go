package profileverifications_test

// Verified badge flow against real Postgres (TEC-549). Enabled by TEST_DATABASE_URL.
import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	profilesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type allowQuotas struct{}

func (allowQuotas) CheckQuota(context.Context, int64, bool, string) error { return nil }

type sent struct {
	userID int64
	code   string
	vars   map[string]string
}

type fakeNotifier struct {
	mu   sync.Mutex
	sent []sent
}

func (f *fakeNotifier) Enqueue(_ context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sent{userID: *in.UserID, code: in.TemplateCode, vars: in.TemplateVars})
	return nil, nil
}

func (f *fakeNotifier) codes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	for i, s := range f.sent {
		out[i] = s.code
	}
	return out
}

func (f *fakeNotifier) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = nil
}

type env struct {
	t        *testing.T
	ctx      context.Context
	d        *pgtest.DB
	profiles *profilesusecase.Service
	svc      *usecase.Service
	notifier *fakeNotifier
	admin    pgtest.User
}

func newEnv(t *testing.T) *env {
	d := pgtest.Open(t)
	repo := profilesrepo.NewPostgres(d.Pool, d.Queries)
	profiles := profilesusecase.NewService(repo, nil)
	profiles.SetQuotas(allowQuotas{})
	profiles.SetPublicPages(repo)
	notifier := &fakeNotifier{}
	svc := usecase.New(d.Pool, d.Queries, notifier, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.SetFrontendURL("https://app.comcode.test/")
	profiles.SetVerifications(svc)
	return &env{t: t, ctx: context.Background(), d: d, profiles: profiles, svc: svc, notifier: notifier, admin: d.CreateUser(t)}
}

func owner(u pgtest.User) authctx.Principal {
	return authctx.Principal{
		UserID: u.UUID, UserInternal: u.ID,
		Permissions: []string{rbac.PermAppProfilesRead, rbac.PermAppProfilesWrite},
		Account:     authctx.Account{ID: u.AccountID, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner},
	}
}

func (e *env) platformAdmin() authctx.Principal {
	return authctx.Principal{
		UserID: e.admin.UUID, UserInternal: e.admin.ID, IsSuperAdmin: true,
		Permissions: []string{rbac.PermPlatformProfileVerificationsManage},
		Account:     authctx.Account{ID: e.admin.AccountID, Role: authctx.AccountRoleOwner},
	}
}

func input(first, last, company, title string) pmodel.UpsertProfileInput {
	return pmodel.UpsertProfileInput{
		PrimaryLocale: "tr",
		Translations: []pmodel.UpsertTranslationInput{
			{Locale: "tr", FirstName: first, LastName: last, CompanyName: company, Title: title, IsDefault: true},
		},
		ContactDetails: []pmodel.CreateContactDetailInput{{Category: "email", Type: "work", Value: "x@example.test"}},
	}
}

func (e *env) profile(u pgtest.User, first string) *pmodel.ProfileDetail {
	e.t.Helper()
	p, err := e.profiles.Create(e.ctx, owner(u), input(first, "Yılmaz", "ComCode", "CTO"))
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}

func (e *env) status(u pgtest.User, id uuid.UUID) string {
	e.t.Helper()
	d, err := e.profiles.Get(e.ctx, owner(u), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return d.VerificationStatus
}

func (e *env) verified(username string) bool {
	e.t.Helper()
	pub, err := e.profiles.PublicProfile(e.ctx, username)
	if err != nil {
		e.t.Fatal(err)
	}
	return pub.Verified
}

// approve requests and approves verification of a fresh profile.
func (e *env) approved(u pgtest.User, first string) (*pmodel.ProfileDetail, uuid.UUID) {
	e.t.Helper()
	p := e.profile(u, first)
	view, err := e.svc.Request(e.ctx, owner(u), p.UUID, model.RequestInput{Note: "Resmi şirket profili"})
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.svc.Approve(e.ctx, e.platformAdmin(), view.Request.UUID); err != nil {
		e.t.Fatal(err)
	}
	return p, view.Request.UUID
}

func TestRequestApproveShowsBadgeAndNotifies(t *testing.T) {
	e := newEnv(t)
	u := e.d.CreateUser(t)
	p := e.profile(u, "Ada")
	if got := e.status(u, p.UUID); got != model.StatusNone {
		t.Fatalf("new profile status = %q, want none", got)
	}

	view, err := e.svc.Request(e.ctx, owner(u), p.UUID, model.RequestInput{Note: "  Resmi şirket profili  "})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != model.StatusPending || view.CanRequest || view.Request.Note != "Resmi şirket profili" {
		t.Fatalf("after request: %+v", view)
	}
	if _, err := e.svc.Request(e.ctx, owner(u), p.UUID, model.RequestInput{}); !errors.Is(err, usecase.ErrConflict) {
		t.Fatalf("second request err = %v, want conflict", err)
	}
	if e.verified(p.Username) {
		t.Fatal("pending profile must not show the badge")
	}

	list, total, err := e.svc.List(e.ctx, model.ListFilter{Status: model.RequestPending, Q: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	if total < 1 || list[0].Profile.UUID != p.UUID || list[0].Profile.CompanyName != "ComCode" || list[0].Profile.Username != p.Username {
		t.Fatalf("review list = %+v (total %d)", list, total)
	}

	review, err := e.svc.Approve(e.ctx, e.platformAdmin(), view.Request.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if review.Status != model.RequestApproved || review.ReviewedAt == nil || review.ReviewedByEmail == "" {
		t.Fatalf("approved review = %+v", review)
	}
	if got := e.status(u, p.UUID); got != model.StatusVerified {
		t.Fatalf("status = %q, want verified", got)
	}
	if !e.verified(p.Username) {
		t.Fatal("verified profile must show the badge on the visitor API")
	}
	if _, err := e.svc.Approve(e.ctx, e.platformAdmin(), view.Request.UUID); !errors.Is(err, usecase.ErrConflict) {
		t.Fatalf("approve twice err = %v, want conflict", err)
	}
	if _, err := e.svc.Request(e.ctx, owner(u), p.UUID, model.RequestInput{}); !errors.Is(err, usecase.ErrConflict) {
		t.Fatalf("request while verified err = %v, want conflict", err)
	}

	e.notifier.mu.Lock()
	defer e.notifier.mu.Unlock()
	if len(e.notifier.sent) != 1 || e.notifier.sent[0].code != "profiles.verification.approved" || e.notifier.sent[0].userID != u.ID {
		t.Fatalf("notifications = %+v, want one approved to the owner", e.notifier.sent)
	}
	if got := e.notifier.sent[0].vars["profile_url"]; got != "https://app.comcode.test/profiles/"+p.UUID.String() {
		t.Fatalf("profile_url = %q", got)
	}
}

func TestRejectNeedsReasonAndAllowsNewRequest(t *testing.T) {
	e := newEnv(t)
	u := e.d.CreateUser(t)
	p := e.profile(u, "Grace")
	view, err := e.svc.Request(e.ctx, owner(u), p.UUID, model.RequestInput{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Reject(e.ctx, e.platformAdmin(), view.Request.UUID, "   "); !errors.Is(err, usecase.ErrInvalidRequest) {
		t.Fatalf("reject without reason err = %v, want invalid", err)
	}
	if _, err := e.svc.Reject(e.ctx, e.platformAdmin(), view.Request.UUID, "Kurum belgesi eksik"); err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.OwnerGet(e.ctx, owner(u), p.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.StatusRejected || !got.CanRequest || got.Request.ReviewReason != "Kurum belgesi eksik" {
		t.Fatalf("after reject: %+v", got)
	}
	if codes := e.notifier.codes(); len(codes) != 1 || codes[0] != "profiles.verification.rejected" {
		t.Fatalf("notifications = %v", codes)
	}
	if e.verified(p.Username) {
		t.Fatal("rejected profile must not show the badge")
	}
	again, err := e.svc.Request(e.ctx, owner(u), p.UUID, model.RequestInput{})
	if err != nil || again.Status != model.StatusPending {
		t.Fatalf("re-request = %+v, %v", again, err)
	}
	cancelled, err := e.svc.Cancel(e.ctx, owner(u), p.UUID)
	if err != nil || cancelled.Status != model.StatusNone || !cancelled.CanRequest {
		t.Fatalf("cancel = %+v, %v", cancelled, err)
	}
	if _, err := e.svc.Cancel(e.ctx, owner(u), p.UUID); !errors.Is(err, usecase.ErrConflict) {
		t.Fatalf("cancel twice err = %v, want conflict", err)
	}
}

func TestRenameRevokesAndReopensReview(t *testing.T) {
	e := newEnv(t)
	u := e.d.CreateUser(t)
	p, approvedID := e.approved(u, "Linus")
	e.notifier.reset()

	// Title, contacts and an extra translation with the same name keep the badge.
	keep := input("Linus", "Yılmaz", "ComCode", "Founder")
	keep.Translations = append(keep.Translations, pmodel.UpsertTranslationInput{Locale: "en", FirstName: "linus ", LastName: " YILMAZ", CompanyName: "comcode"})
	if _, err := e.profiles.Update(e.ctx, owner(u), p.UUID, keep); err != nil {
		t.Fatal(err)
	}
	if got := e.status(u, p.UUID); got != model.StatusVerified {
		t.Fatalf("title change: status = %q, want verified", got)
	}

	// A new organization revokes the badge and opens a re-review.
	moved := input("Linus", "Yılmaz", "Other Corp", "Founder")
	updated, err := e.profiles.Update(e.ctx, owner(u), p.UUID, moved)
	if err != nil {
		t.Fatal(err)
	}
	if updated.VerificationStatus != model.StatusPending {
		t.Fatalf("after organization change status = %q, want pending", updated.VerificationStatus)
	}
	if e.verified(p.Username) {
		t.Fatal("badge must disappear after an organization change")
	}
	old, err := e.svc.Get(e.ctx, approvedID)
	if err != nil {
		t.Fatal(err)
	}
	if old.Status != model.RequestRevoked || old.RevokeReason != model.ReasonOrganizationChanged || old.RevokedAt == nil {
		t.Fatalf("revoked row = %+v", old)
	}
	view, err := e.svc.OwnerGet(e.ctx, owner(u), p.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Request.Origin != model.OriginReReview || view.Status != model.StatusPending {
		t.Fatalf("re-review = %+v", view.Request)
	}
	if codes := e.notifier.codes(); len(codes) != 1 || codes[0] != "profiles.verification.re_review" {
		t.Fatalf("notifications = %v", codes)
	}

	// Approve the re-review, then a rename records the name reason.
	if _, err := e.svc.Approve(e.ctx, e.platformAdmin(), view.Request.UUID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.profiles.Update(e.ctx, owner(u), p.UUID, input("Linus B.", "Yılmaz", "Other Corp", "Founder")); err != nil {
		t.Fatal(err)
	}
	second, err := e.svc.Get(e.ctx, view.Request.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != model.RequestRevoked || second.RevokeReason != model.ReasonDisplayNameChanged {
		t.Fatalf("rename revoke = %+v", second)
	}

	// Renaming a profile that is not verified changes nothing.
	plain := e.profile(u, "Plain")
	if _, err := e.profiles.Update(e.ctx, owner(u), plain.UUID, input("Renamed", "Yılmaz", "ComCode", "CTO")); err != nil {
		t.Fatal(err)
	}
	if got := e.status(u, plain.UUID); got != model.StatusNone {
		t.Fatalf("unverified rename status = %q, want none", got)
	}
}

func TestAdminRevoke(t *testing.T) {
	e := newEnv(t)
	u := e.d.CreateUser(t)
	p, id := e.approved(u, "Barbara")
	if _, err := e.svc.Revoke(e.ctx, e.platformAdmin(), id, ""); !errors.Is(err, usecase.ErrInvalidRequest) {
		t.Fatalf("revoke without reason err = %v", err)
	}
	if _, err := e.svc.Revoke(e.ctx, e.platformAdmin(), id, "Sahte belge"); err != nil {
		t.Fatal(err)
	}
	view, err := e.svc.OwnerGet(e.ctx, owner(u), p.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != model.StatusNone || !view.CanRequest || view.Request.RevokeReason != "Sahte belge" {
		t.Fatalf("after admin revoke: %+v", view)
	}
	if _, err := e.svc.Revoke(e.ctx, e.platformAdmin(), id, "again"); !errors.Is(err, usecase.ErrConflict) {
		t.Fatalf("revoke twice err = %v, want conflict", err)
	}
	if _, err := e.svc.Approve(e.ctx, e.platformAdmin(), uuid.New()); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("approve unknown err = %v, want not found", err)
	}
}

func TestAccountIsolationAndRoles(t *testing.T) {
	e := newEnv(t)
	u := e.d.CreateUser(t)
	other := e.d.CreateUser(t)
	p := e.profile(u, "Isolated")

	if _, err := e.svc.OwnerGet(e.ctx, owner(other), p.UUID); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("other account get err = %v, want not found", err)
	}
	if _, err := e.svc.Request(e.ctx, owner(other), p.UUID, model.RequestInput{}); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("other account request err = %v, want not found", err)
	}

	member := owner(u)
	member.Account.Role = authctx.AccountRoleMember
	if _, err := e.svc.Request(e.ctx, member, p.UUID, model.RequestInput{}); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("member request err = %v, want forbidden", err)
	}
	if view, err := e.svc.OwnerGet(e.ctx, member, p.UUID); err != nil || view.Status != model.StatusNone {
		t.Fatalf("member read = %+v, %v", view, err)
	}

	// A company admin may request for the company account.
	company, err := e.d.Queries.CreateCompanyAccount(e.ctx, db.CreateCompanyAccountParams{Name: "Verify Co", Slug: "verify-" + pgtest.Unique()})
	if err != nil {
		t.Fatal(err)
	}
	admin := owner(u)
	admin.Account = authctx.Account{ID: company.ID, UUID: company.Uuid, Type: authctx.AccountTypeCompany, Role: authctx.AccountRoleOwner}
	cp, err := e.profiles.Create(e.ctx, admin, input("Company", "Person", "Verify Co", ""))
	if err != nil {
		t.Fatal(err)
	}
	admin.Account.Role = authctx.AccountRoleAdmin
	if _, err := e.svc.Request(e.ctx, admin, cp.UUID, model.RequestInput{}); err != nil {
		t.Fatalf("company admin request: %v", err)
	}
	if _, err := e.svc.OwnerGet(e.ctx, owner(u), cp.UUID); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("personal account sees company profile: %v", err)
	}
}

type identity struct{ principal authctx.Principal }

func (i identity) LoadPrincipal(*http.Request, jwt.Claims) (authctx.Principal, error) {
	return i.principal, nil
}

func TestRoutePermissions(t *testing.T) {
	e := newEnv(t)
	u := e.d.CreateUser(t)
	p := e.profile(u, "Routes")
	tokens, err := jwt.NewManager("fixture-jwt-secret", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	bearer, _, err := tokens.IssueAccess(jwt.AccessInput{UserID: u.UUID})
	if err != nil {
		t.Fatal(err)
	}
	call := func(pr authctx.Principal, method, path, body string) *httptest.ResponseRecorder {
		mux := http.NewServeMux()
		profileverifications.RegisterRoutes(mux, handler.New(e.svc), tokens, identity{principal: pr})
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+bearer)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	readOnly := owner(u)
	readOnly.Permissions = []string{rbac.PermAppProfilesRead}
	path := "/v1/profiles/" + p.UUID.String() + "/verification"
	if rec := call(readOnly, http.MethodGet, path, ""); rec.Code != http.StatusOK {
		t.Fatalf("read GET = %d %s", rec.Code, rec.Body)
	}
	if rec := call(readOnly, http.MethodPost, path, `{}`); rec.Code != http.StatusForbidden {
		t.Fatalf("POST without app.profiles.write = %d, want 403", rec.Code)
	}
	rec := call(owner(u), http.MethodPost, path, `{"note":"lütfen"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("owner POST = %d %s", rec.Code, rec.Body)
	}
	if rec := call(owner(u), http.MethodPost, path, ``); rec.Code != http.StatusConflict {
		t.Fatalf("second POST = %d, want 409", rec.Code)
	}

	if rec := call(owner(u), http.MethodGet, "/v1/platform/profile-verifications?status=pending", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("platform list without permission = %d, want 403", rec.Code)
	}
	rec = call(e.platformAdmin(), http.MethodGet, "/v1/platform/profile-verifications?status=pending&q=Routes", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("platform list = %d %s", rec.Code, rec.Body)
	}
	var page struct {
		Data struct {
			Items []model.Review `json:"items"`
			Total int            `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Data.Total != 1 || page.Data.Items[0].Note != "lütfen" {
		t.Fatalf("platform list body = %s", rec.Body)
	}
	if rec := call(e.platformAdmin(), http.MethodGet, "/v1/platform/profile-verifications?status=bogus", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad status filter = %d, want 400", rec.Code)
	}
	id := page.Data.Items[0].UUID.String()
	if rec := call(owner(u), http.MethodPost, "/v1/platform/profile-verifications/"+id+"/approve", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("approve without permission = %d, want 403", rec.Code)
	}
	if rec := call(e.platformAdmin(), http.MethodPost, "/v1/platform/profile-verifications/"+id+"/reject", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("reject without reason = %d, want 400", rec.Code)
	}
	if rec := call(e.platformAdmin(), http.MethodPost, "/v1/platform/profile-verifications/"+id+"/approve", ""); rec.Code != http.StatusOK {
		t.Fatalf("approve = %d %s", rec.Code, rec.Body)
	}
}

// Avatar uploads change the score outside the upsert payload; the stored completion_rate (used by
// the profile list) must follow (TEC-549 single-source score).
func TestAvatarUploadSyncsStoredCompletion(t *testing.T) {
	e := newEnv(t)
	u := e.d.CreateUser(t)
	p := e.profile(u, "Avatar")
	before := p.CompletionRate
	up, err := e.profiles.UploadAvatar(e.ctx, owner(u), p.UUID, strings.NewReader("img"), "a.png", "image/png", 3)
	if err != nil {
		t.Fatal(err)
	}
	if up.CompletionRate != before+15 {
		t.Fatalf("detail rate = %d, want %d", up.CompletionRate, before+15)
	}
	for _, s := range up.CompletionSuggestions {
		if s.Key == "avatar" {
			t.Fatal("avatar suggestion must be gone after upload")
		}
	}
	items, _, err := e.profiles.List(e.ctx, owner(u), 100, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.UUID == p.UUID && it.CompletionRate != before+15 {
			t.Fatalf("stored rate = %d, want %d", it.CompletionRate, before+15)
		}
	}
}
