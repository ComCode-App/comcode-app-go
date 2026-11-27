package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type env struct {
	t   *testing.T
	d   *pgtest.DB
	ctx context.Context
	svc *usecase.Service
}

func newEnv(t *testing.T) *env {
	d := pgtest.Open(t)
	svc := usecase.New(repository.NewPostgres(d.Pool, d.Queries), nil)
	svc.SetFeatures(entitlement.New(d.Queries))
	return &env{t: t, d: d, ctx: context.Background(), svc: svc}
}

func (e *env) company(name string) (db.Account, pgtest.User) {
	e.t.Helper()
	owner := e.d.CreateUser(e.t)
	a, err := e.d.Queries.CreateCompanyAccount(e.ctx, db.CreateCompanyAccountParams{Name: name, Slug: "biz-" + pgtest.Unique()})
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

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.d.Pool.Exec(e.ctx, sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func principal(accountID int64, accountType string, u pgtest.User, role string) authctx.Principal {
	return authctx.Principal{
		UserID: u.UUID, UserInternal: u.ID,
		Permissions: []string{rbac.PermAppBusinessPageRead, rbac.PermAppBusinessPageWrite},
		Account:     authctx.Account{ID: accountID, Type: accountType, Role: role},
	}
}

func raw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestIntegrationBusinessPage(t *testing.T) {
	e := newEnv(t)
	acme, acmeOwner := e.company("Acme")
	other, otherOwner := e.company("Other")
	memberUser := e.d.CreateUser(t)
	e.member(acme, memberUser, db.AccountRoleMember)
	owner := principal(acme.ID, authctx.AccountTypeCompany, acmeOwner, authctx.AccountRoleOwner)
	member := principal(acme.ID, authctx.AccountTypeCompany, memberUser, authctx.AccountRoleMember)
	foreign := principal(other.ID, authctx.AccountTypeCompany, otherOwner, authctx.AccountRoleOwner)
	personalUser := e.d.CreateUser(t)
	personal := principal(personalUser.AccountID, authctx.AccountTypePersonal, personalUser, authctx.AccountRoleOwner)
	username := "acme-" + pgtest.Unique()

	// Company data the blocks read.
	e.exec(`INSERT INTO company_offices (account_id, name, city, district, address) VALUES ($1, 'Merkez', 'İstanbul', 'Kadıköy', 'Moda Cd. 1')`, acme.ID)
	var bankID, foreignBankID uuid.UUID
	if err := e.d.Pool.QueryRow(e.ctx, `INSERT INTO user_bank_accounts (account_id, account_holder, iban) VALUES ($1, 'Acme AŞ', 'TR000000000000000000000001') RETURNING uuid`, acme.ID).Scan(&bankID); err != nil {
		t.Fatal(err)
	}
	if err := e.d.Pool.QueryRow(e.ctx, `INSERT INTO user_bank_accounts (account_id, account_holder, iban) VALUES ($1, 'Other', 'TR000000000000000000000002') RETURNING uuid`, other.ID).Scan(&foreignBankID); err != nil {
		t.Fatal(err)
	}
	// An employee profile with a published page, linked to the member.
	var profileID int64
	empName := "emp-" + pgtest.Unique()
	if err := e.d.Pool.QueryRow(e.ctx, `INSERT INTO profiles (user_id, account_id) VALUES ($1, $2) RETURNING id`, acmeOwner.ID, acme.ID).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	e.exec(`INSERT INTO profile_translations (profile_id, locale, first_name, last_name, title, is_default) VALUES ($1, 'tr', 'Ayşe', 'Yılmaz', 'Satış', true)`, profileID)
	e.exec(`INSERT INTO routes (path, route_type, status, target_type, profile_id, account_id) VALUES ($1, 'canonical_profile', 'active', 'profile', $2, $3)`, empName, profileID, acme.ID)
	e.exec(`INSERT INTO company_member_profiles (account_id, member_id, profile_id)
		SELECT $1, id, $2 FROM account_members WHERE account_id = $1 AND user_id = $3`, acme.ID, profileID, memberUser.ID)

	page := model.SaveInput{
		Username: username, PrimaryLocale: "tr",
		Translations: map[string]model.PageText{"tr": {Name: "Acme", Tagline: "Her şey burada"}, "en": {Name: "Acme", Tagline: "All here"}},
		Blocks: []model.BlockInput{
			{Type: model.BlockHeader},
			{Type: model.BlockButton, Config: raw(model.ButtonConfig{Target: model.TargetWhatsApp, Value: "+90 555 111 22 33"}),
				Translations: map[string]model.BlockText{"tr": {Title: "WhatsApp", Subtitle: "Hemen yazın"}}},
			{Type: model.BlockButton, Config: raw(model.ButtonConfig{Target: model.TargetGoogleReview, Value: "https://g.page/r/acme/review"})},
			{Type: model.BlockButton, Config: raw(model.ButtonConfig{Target: model.TargetLeadForm, Value: "ignored"})},
			{Type: model.BlockSocial, Config: raw(model.SocialConfig{Links: []model.SocialLink{{Platform: "Instagram", URL: "https://instagram.com/acme"}}})},
			{Type: model.BlockOffices},
			{Type: model.BlockTeam},
			{Type: model.BlockBankAccounts, Config: raw(model.BankAccountsConfig{BankAccountUUIDs: []uuid.UUID{bankID}})},
		},
	}

	// Personal accounts and members cannot; the first save needs a username.
	if _, err := e.svc.Get(e.ctx, personal); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("personal get: %v", err)
	}
	if _, err := e.svc.Save(e.ctx, member, page); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("member save: %v", err)
	}
	if _, err := e.svc.Get(e.ctx, owner); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("get before save: %v", err)
	}
	noName := page
	noName.Username = ""
	if _, err := e.svc.Save(e.ctx, owner, noName); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("save without username: %v", err)
	}
	// The username pool is shared with profiles.
	taken := page
	taken.Username = empName
	if _, err := e.svc.Save(e.ctx, owner, taken); !errors.Is(err, model.ErrUsernameTaken) {
		t.Fatalf("profile username: %v", err)
	}
	// Another company's bank account and bad button targets are refused.
	bad := page
	bad.Blocks = []model.BlockInput{{Type: model.BlockBankAccounts, Config: raw(model.BankAccountsConfig{BankAccountUUIDs: []uuid.UUID{foreignBankID}})}}
	if _, err := e.svc.Save(e.ctx, owner, bad); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("foreign bank account: %v", err)
	}
	bad.Blocks = []model.BlockInput{{Type: model.BlockButton, Config: raw(model.ButtonConfig{Target: model.TargetURL, Value: "javascript:alert(1)"})}}
	if _, err := e.svc.Save(e.ctx, owner, bad); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("javascript url: %v", err)
	}
	bad.Blocks = []model.BlockInput{{Type: model.BlockButton, Config: json.RawMessage(`{"target":"url","value":"https://x.test","extra":1}`)}}
	if _, err := e.svc.Save(e.ctx, owner, bad); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("unknown config field: %v", err)
	}

	saved, err := e.svc.Save(e.ctx, owner, page)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.StatusDraft || saved.Username != username || len(saved.Blocks) != 8 || saved.Blocks[1].Type != model.BlockButton {
		t.Fatalf("saved: %+v", saved)
	}
	// Another company sees only its own (none).
	if _, err := e.svc.Get(e.ctx, foreign); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("foreign get: %v", err)
	}
	// A draft is not public.
	if _, err := e.svc.Public(e.ctx, username); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("draft public: %v", err)
	}
	if _, err := e.svc.SetPublished(e.ctx, member, true); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("member publish: %v", err)
	}
	if _, err := e.svc.SetPublished(e.ctx, owner, true); err != nil {
		t.Fatal(err)
	}
	pub, err := e.svc.Public(e.ctx, username)
	if err != nil {
		t.Fatal(err)
	}
	if pub.CompanyName != "Acme" || pub.Translations["tr"].Tagline != "Her şey burada" || len(pub.Blocks) != 8 {
		t.Fatalf("public: %+v", pub)
	}
	b := pub.Blocks
	if b[1].Button == nil || b[1].Button.Target != "whatsapp" || b[1].Translations["tr"].Title != "WhatsApp" ||
		b[3].Button.Value != "" || b[4].Social[0].Platform != "instagram" ||
		len(b[5].Offices) != 1 || b[5].Offices[0].District != "Kadıköy" ||
		len(b[6].Team) != 1 || b[6].Team[0].Username != empName ||
		len(b[7].BankAccounts) != 1 || b[7].BankAccounts[0].AccountHolder != "Acme AŞ" {
		t.Fatalf("public blocks: %+v", b)
	}
	// A closed employee profile leaves the team block (TEC-1326).
	e.exec(`UPDATE profiles SET is_open = false WHERE id = $1`, profileID)
	if pub, _ := e.svc.Public(e.ctx, username); len(pub.Blocks[6].Team) != 0 {
		t.Fatalf("closed profile in team: %+v", pub.Blocks[6].Team)
	}

	// A disabled block is not served.
	page.Blocks[7].Enabled = new(bool)
	if _, err := e.svc.Save(e.ctx, owner, page); err != nil {
		t.Fatal(err)
	}
	if pub, _ := e.svc.Public(e.ctx, username); len(pub.Blocks) != 7 {
		t.Fatalf("disabled block served: %d", len(pub.Blocks))
	}

	// Rename: the old username redirects, the page stays published.
	renamed := username + "-new"
	page.Username = renamed
	if out, err := e.svc.Save(e.ctx, owner, page); err != nil || out.Username != renamed || out.Status != model.StatusPublished {
		t.Fatalf("rename: %+v %v", out, err)
	}
	if old, err := e.svc.Public(e.ctx, username); err != nil || old.RedirectTo != renamed {
		t.Fatalf("old username: %+v %v", old, err)
	}
	// Renaming back takes the retired username again.
	page.Username = username
	if _, err := e.svc.Save(e.ctx, owner, page); err != nil {
		t.Fatalf("rename back: %v", err)
	}
	if _, err := e.svc.Public(e.ctx, username); err != nil {
		t.Fatalf("renamed back: %v", err)
	}

	// Unpublish: 404 again.
	if _, err := e.svc.SetPublished(e.ctx, owner, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Public(e.ctx, username); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unpublished public: %v", err)
	}

	// Plan without the feature: no save, no publish, and the public page is gone.
	if _, err := e.svc.SetPublished(e.ctx, owner, true); err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE plan_limits SET value_bool = false WHERE feature_key = 'business_page.enabled' AND plan_id = (SELECT id FROM plans WHERE slug = 'free')`)
	t.Cleanup(func() {
		_, _ = e.d.Pool.Exec(context.Background(), `UPDATE plan_limits SET value_bool = true WHERE feature_key = 'business_page.enabled'`)
	})
	if _, err := e.svc.Save(e.ctx, owner, page); !errors.Is(err, entitlement.ErrPlanLimitExceeded) {
		t.Fatalf("save without feature: %v", err)
	}
	if _, err := e.svc.Public(e.ctx, username); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("public without feature: %v", err)
	}
}
