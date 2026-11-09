package usecase_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts/usecase"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

// mailbox records queued invite emails.
type mailbox struct {
	mu   sync.Mutex
	sent []notifmodel.EnqueueInput
}

func (m *mailbox) Enqueue(_ context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, in)
	return nil, nil
}

func (m *mailbox) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// lastToken is the token of the newest email's link.
func (m *mailbox) lastToken(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		t.Fatal("no invite email")
	}
	url := m.sent[len(m.sent)-1].TemplateVars["invite_url"]
	i := strings.LastIndex(url, "/platform/invite/")
	if i < 0 {
		t.Fatalf("invite url %q", url)
	}
	return url[i+len("/platform/invite/"):]
}

type inviteEnv struct {
	t    *testing.T
	d    *pgtest.DB
	ctx  context.Context
	svc  *usecase.Invites
	mail *mailbox
}

func newInviteEnv(t *testing.T) *inviteEnv {
	d := pgtest.Open(t)
	mail := &mailbox{}
	return &inviteEnv{t: t, d: d, ctx: context.Background(), mail: mail,
		svc: usecase.NewInvites(d.Pool, d.Queries, mail, "https://app.example.test/", nil)}
}

type companyFixture struct {
	account db.Account
	owner   pgtest.User
}

func (e *inviteEnv) company(name string) companyFixture {
	e.t.Helper()
	owner := e.d.CreateUser(e.t)
	a, err := e.d.Queries.CreateCompanyAccount(e.ctx, db.CreateCompanyAccountParams{Name: name, Slug: "inv-" + pgtest.Unique()})
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.d.Queries.AddAccountMember(e.ctx, db.AddAccountMemberParams{AccountID: a.ID, UserID: owner.ID, Role: db.AccountRoleOwner}); err != nil {
		e.t.Fatal(err)
	}
	return companyFixture{account: a, owner: owner}
}

func (e *inviteEnv) email(u pgtest.User) string {
	e.t.Helper()
	var email string
	if err := e.d.Pool.QueryRow(e.ctx, `SELECT email FROM users WHERE id = $1`, u.ID).Scan(&email); err != nil {
		e.t.Fatal(err)
	}
	return email
}

// signUp creates the user an invitee registers as.
func (e *inviteEnv) signUp(email string) pgtest.User {
	e.t.Helper()
	var u pgtest.User
	if err := e.d.Pool.QueryRow(e.ctx, `
		INSERT INTO users (email, password_hash, name, surname, status)
		VALUES ($1, 'x', 'New', 'Person', 'active') RETURNING id, uuid`, email).Scan(&u.ID, &u.UUID); err != nil {
		e.t.Fatal(err)
	}
	return u
}

func (e *inviteEnv) role(a db.Account, u pgtest.User) (db.AccountRole, bool) {
	m, err := e.d.Queries.GetAccountMemberIDByUser(e.ctx, db.GetAccountMemberIDByUserParams{AccountID: a.ID, UserID: u.ID})
	if err != nil {
		return "", false
	}
	return m.Role, true
}

func asRole(a db.Account, u pgtest.User, role string) authctx.Principal {
	return authctx.Principal{
		UserID: u.UUID, UserInternal: u.ID,
		Account: authctx.Account{ID: a.ID, UUID: a.Uuid, Type: authctx.AccountTypeCompany, Role: role},
	}
}

func TestIntegrationInviteAcceptExistingUser(t *testing.T) {
	e := newInviteEnv(t)
	c := e.company("Acme")
	invitee := e.d.CreateUser(t)
	stranger := e.d.CreateUser(t)
	owner := asRole(c.account, c.owner, authctx.AccountRoleOwner)

	inv, err := e.svc.Create(e.ctx, owner, "  "+strings.ToUpper(e.email(invitee))+" ", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != usecase.InviteStatusPending || inv.Role != "admin" || inv.Email != e.email(invitee) || inv.SendCount != 1 {
		t.Fatalf("created: %+v", inv)
	}
	if e.mail.count() != 1 || *e.mail.sent[0].Recipient != e.email(invitee) || e.mail.sent[0].TemplateCode != "accounts.invite" {
		t.Fatalf("mail: %+v", e.mail.sent)
	}
	token := e.mail.lastToken(t)
	if !strings.HasPrefix(e.mail.sent[0].TemplateVars["invite_url"], "https://app.example.test/platform/invite/") {
		t.Fatalf("url %q", e.mail.sent[0].TemplateVars["invite_url"])
	}
	// The stored hash is not the token.
	var stored []byte
	_ = e.d.Pool.QueryRow(e.ctx, `SELECT token_hash FROM account_invites WHERE uuid = $1`, inv.UUID).Scan(&stored)
	if strings.Contains(string(stored), token) {
		t.Fatal("token stored in clear")
	}

	// A second open invite for the address is a conflict; so is inviting a member.
	if _, err := e.svc.Create(e.ctx, owner, e.email(invitee), "member"); !errors.Is(err, usecase.ErrConflict) {
		t.Fatalf("duplicate invite: %v", err)
	}
	if _, err := e.svc.Create(e.ctx, owner, e.email(c.owner), "member"); !errors.Is(err, usecase.ErrConflict) {
		t.Fatalf("invite member: %v", err)
	}

	pv, err := e.svc.Preview(e.ctx, token)
	if err != nil || !pv.UserExists || pv.AccountName != "Acme" || pv.Status != usecase.InviteStatusPending || pv.Role != "admin" {
		t.Fatalf("preview: %+v %v", pv, err)
	}
	if _, err := e.svc.Preview(e.ctx, "not-a-token"); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("bad token: %v", err)
	}

	// Only the invited address may accept.
	if _, err := e.svc.Accept(e.ctx, stranger.ID, token); !errors.Is(err, usecase.ErrInviteEmailMismatch) {
		t.Fatalf("stranger accept: %v", err)
	}
	if _, ok := e.role(c.account, stranger); ok {
		t.Fatal("stranger joined")
	}
	res, err := e.svc.Accept(e.ctx, invitee.ID, token)
	if err != nil || res.AccountUUID != c.account.Uuid || res.Role != "admin" || res.ProfileUUID != nil {
		t.Fatalf("accept: %+v %v", res, err)
	}
	if role, ok := e.role(c.account, invitee); !ok || role != db.AccountRoleAdmin {
		t.Fatalf("membership: %v %v", role, ok)
	}
	if _, err := e.svc.Accept(e.ctx, invitee.ID, token); !errors.Is(err, usecase.ErrInviteClosed) {
		t.Fatalf("second accept: %v", err)
	}
	accepted, _, err := e.svc.List(e.ctx, owner, usecase.InviteFilter{Status: usecase.InviteStatusAccepted})
	if err != nil || len(accepted) != 1 || accepted[0].AcceptedAt == "" {
		t.Fatalf("accepted list: %+v %v", accepted, err)
	}
	open, total, err := e.svc.List(e.ctx, owner, usecase.InviteFilter{})
	if err != nil || len(open) != 0 || total != 0 {
		t.Fatalf("open list: %+v %v", open, err)
	}
}

func TestIntegrationInviteAcceptNewUser(t *testing.T) {
	e := newInviteEnv(t)
	c := e.company("Newco")
	owner := asRole(c.account, c.owner, authctx.AccountRoleOwner)
	address := "fresh-" + pgtest.Unique() + "@example.test"

	if _, err := e.svc.Create(e.ctx, owner, address, ""); err != nil {
		t.Fatal(err)
	}
	token := e.mail.lastToken(t)
	pv, err := e.svc.Preview(e.ctx, token)
	if err != nil || pv.UserExists || pv.Role != "member" || pv.Email != address {
		t.Fatalf("preview: %+v %v", pv, err)
	}
	// Registration accepts the token for the invited address only.
	if !e.svc.PendingFor(e.ctx, token, strings.ToUpper(address)) || e.svc.PendingFor(e.ctx, token, "other@example.test") ||
		e.svc.PendingFor(e.ctx, "nope", address) {
		t.Fatal("PendingFor")
	}
	u := e.signUp(address)
	res, err := e.svc.Accept(e.ctx, u.ID, token)
	if err != nil || res.Role != "member" {
		t.Fatalf("accept: %+v %v", res, err)
	}
	if role, ok := e.role(c.account, u); !ok || role != db.AccountRoleMember {
		t.Fatalf("membership: %v %v", role, ok)
	}
	if e.svc.PendingFor(e.ctx, token, address) {
		t.Fatal("accepted invite still opens registration")
	}
}

func TestIntegrationInviteExpiryAndResend(t *testing.T) {
	e := newInviteEnv(t)
	c := e.company("Slowpoke")
	owner := asRole(c.account, c.owner, authctx.AccountRoleOwner)
	invitee := e.d.CreateUser(t)
	inv, err := e.svc.Create(e.ctx, owner, e.email(invitee), "member")
	if err != nil {
		t.Fatal(err)
	}
	oldToken := e.mail.lastToken(t)
	if _, err := e.d.Pool.Exec(e.ctx, `UPDATE account_invites SET expires_at = now() - interval '1 minute' WHERE uuid = $1`, inv.UUID); err != nil {
		t.Fatal(err)
	}

	expired, _, err := e.svc.List(e.ctx, owner, usecase.InviteFilter{Status: usecase.InviteStatusExpired})
	if err != nil || len(expired) != 1 || expired[0].Status != usecase.InviteStatusExpired {
		t.Fatalf("expired list: %+v %v", expired, err)
	}
	pending, _, _ := e.svc.List(e.ctx, owner, usecase.InviteFilter{Status: usecase.InviteStatusPending})
	if len(pending) != 0 {
		t.Fatalf("pending list: %+v", pending)
	}
	// The default list shows open invites, the expired one included.
	open, _, _ := e.svc.List(e.ctx, owner, usecase.InviteFilter{})
	if len(open) != 1 || open[0].Status != usecase.InviteStatusExpired {
		t.Fatalf("open list: %+v", open)
	}
	if pv, err := e.svc.Preview(e.ctx, oldToken); err != nil || pv.Status != usecase.InviteStatusExpired {
		t.Fatalf("preview: %+v %v", pv, err)
	}
	if _, err := e.svc.Accept(e.ctx, invitee.ID, oldToken); !errors.Is(err, usecase.ErrInviteExpired) {
		t.Fatalf("accept expired: %v", err)
	}

	resent, err := e.svc.Resend(e.ctx, owner, inv.UUID)
	if err != nil || resent.Status != usecase.InviteStatusPending || resent.SendCount != 2 || e.mail.count() != 2 {
		t.Fatalf("resend: %+v %v (mails %d)", resent, err, e.mail.count())
	}
	newToken := e.mail.lastToken(t)
	if newToken == oldToken {
		t.Fatal("resend kept the token")
	}
	if _, err := e.svc.Preview(e.ctx, oldToken); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("old token after resend: %v", err)
	}
	if _, err := e.svc.Accept(e.ctx, invitee.ID, newToken); err != nil {
		t.Fatalf("accept resent: %v", err)
	}
	if _, err := e.svc.Resend(e.ctx, owner, inv.UUID); !errors.Is(err, usecase.ErrInviteClosed) {
		t.Fatalf("resend accepted: %v", err)
	}
}

func TestIntegrationInviteRevoke(t *testing.T) {
	e := newInviteEnv(t)
	c := e.company("Revoker")
	owner := asRole(c.account, c.owner, authctx.AccountRoleOwner)
	invitee := e.d.CreateUser(t)
	inv, err := e.svc.Create(e.ctx, owner, e.email(invitee), "member")
	if err != nil {
		t.Fatal(err)
	}
	token := e.mail.lastToken(t)
	if err := e.svc.Revoke(e.ctx, owner, inv.UUID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Revoke(e.ctx, owner, inv.UUID); !errors.Is(err, usecase.ErrInviteClosed) {
		t.Fatalf("revoke twice: %v", err)
	}
	if _, err := e.svc.Accept(e.ctx, invitee.ID, token); !errors.Is(err, usecase.ErrInviteClosed) {
		t.Fatalf("accept revoked: %v", err)
	}
	if _, ok := e.role(c.account, invitee); ok {
		t.Fatal("revoked invite joined")
	}
	if pv, _ := e.svc.Preview(e.ctx, token); pv.Status != usecase.InviteStatusRevoked {
		t.Fatalf("preview: %+v", pv)
	}
	if _, err := e.svc.Resend(e.ctx, owner, inv.UUID); !errors.Is(err, usecase.ErrInviteClosed) {
		t.Fatalf("resend revoked: %v", err)
	}
	revoked, _, _ := e.svc.List(e.ctx, owner, usecase.InviteFilter{Status: usecase.InviteStatusRevoked})
	if len(revoked) != 1 || revoked[0].RevokedAt == "" {
		t.Fatalf("revoked list: %+v", revoked)
	}
	// A revoked address can be invited again.
	if _, err := e.svc.Create(e.ctx, owner, e.email(invitee), "member"); err != nil {
		t.Fatalf("re-invite: %v", err)
	}
}

func TestIntegrationInviteAccessControl(t *testing.T) {
	e := newInviteEnv(t)
	a := e.company("Alpha")
	b := e.company("Beta")
	ownerA := asRole(a.account, a.owner, authctx.AccountRoleOwner)
	ownerB := asRole(b.account, b.owner, authctx.AccountRoleOwner)
	inv, err := e.svc.Create(e.ctx, ownerA, "x-"+pgtest.Unique()+"@example.test", "member")
	if err != nil {
		t.Fatal(err)
	}

	// Another account's owner sees, resends and revokes nothing of Alpha's.
	if items, total, err := e.svc.List(e.ctx, ownerB, usecase.InviteFilter{}); err != nil || len(items) != 0 || total != 0 {
		t.Fatalf("cross-account list: %+v %v", items, err)
	}
	if _, err := e.svc.Resend(e.ctx, ownerB, inv.UUID); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("cross-account resend: %v", err)
	}
	if err := e.svc.Revoke(e.ctx, ownerB, inv.UUID); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("cross-account revoke: %v", err)
	}

	// Plain members and office managers cannot manage invites; admins cannot invite owners.
	for _, role := range []string{authctx.AccountRoleMember, authctx.AccountRoleOfficeManager} {
		p := asRole(a.account, e.d.CreateUser(t), role)
		if _, err := e.svc.Create(e.ctx, p, "y@example.test", "member"); !errors.Is(err, usecase.ErrForbidden) {
			t.Fatalf("%s create: %v", role, err)
		}
		if _, _, err := e.svc.List(e.ctx, p, usecase.InviteFilter{}); !errors.Is(err, usecase.ErrForbidden) {
			t.Fatalf("%s list: %v", role, err)
		}
		if err := e.svc.Revoke(e.ctx, p, inv.UUID); !errors.Is(err, usecase.ErrForbidden) {
			t.Fatalf("%s revoke: %v", role, err)
		}
	}
	admin := asRole(a.account, e.d.CreateUser(t), authctx.AccountRoleAdmin)
	if _, err := e.svc.Create(e.ctx, admin, "z-"+pgtest.Unique()+"@example.test", "owner"); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("admin invites owner: %v", err)
	}
	if _, err := e.svc.Resend(e.ctx, admin, inv.UUID); err != nil {
		t.Fatalf("admin resend: %v", err)
	}

	// Personal accounts take no invites.
	solo := e.d.CreateUser(t)
	personal := authctx.Principal{UserID: solo.UUID, UserInternal: solo.ID,
		Account: authctx.Account{ID: solo.AccountID, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner}}
	if _, err := e.svc.Create(e.ctx, personal, "p@example.test", "member"); !errors.Is(err, usecase.ErrPersonalAccount) {
		t.Fatalf("personal create: %v", err)
	}
	if _, err := e.svc.Create(e.ctx, ownerA, "not-an-email", "member"); !errors.Is(err, usecase.ErrInvalidRequest) {
		t.Fatalf("bad email: %v", err)
	}

	// A suspended account's invite cannot be accepted.
	invitee := e.d.CreateUser(t)
	if _, err := e.svc.Create(e.ctx, ownerA, e.email(invitee), "member"); err != nil {
		t.Fatal(err)
	}
	token := e.mail.lastToken(t)
	if _, err := e.d.Pool.Exec(e.ctx, `UPDATE accounts SET status = 'suspended' WHERE id = $1`, a.account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Accept(e.ctx, invitee.ID, token); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("suspended accept: %v", err)
	}
}
