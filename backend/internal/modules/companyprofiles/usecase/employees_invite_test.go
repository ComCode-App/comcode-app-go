package usecase_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	accountsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/usecase"
	importusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports/usecase"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

type inviteMails struct {
	mu   sync.Mutex
	urls []string
}

func (m *inviteMails) Enqueue(_ context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.urls = append(m.urls, in.TemplateVars["invite_url"])
	return nil, nil
}

func (m *inviteMails) tokens() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.urls))
	for _, u := range m.urls {
		out = append(out, u[strings.LastIndex(u, "/")+1:])
	}
	return out
}

// TestIntegrationEmployeeImportInvites: a row whose email has no user creates the profile at
// once and an invite; accepting links the profile and places the member (TEC-945).
func TestIntegrationEmployeeImportInvites(t *testing.T) {
	e := newEnv(t)
	mails := &inviteMails{}
	invites := accountsusecase.NewInvites(e.d.Pool, e.d.Queries, mails, "https://app.example.test", nil)
	adapter := usecase.NewImportAdapter(e.d.Queries, e.profiles)
	adapter.SetInviter(invites)
	imports := importusecase.New(e.d.Queries, memDriver{m: storage.NewMemory()}, ioengine.NewRegistry(adapter), nil, nil, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	c := e.company("Inviter Co")
	ist := e.office(c.account, "İstanbul", "İstanbul", "", "")
	sales := e.group(c.account, "Satış", &ist.Uuid)
	address := "employee-" + pgtest.Unique() + "@example.test"
	upload := func(csv string) uuid.UUID {
		t.Helper()
		job, err := imports.Upload(e.ctx, c.owner.ID, c.account.ID, model.ResourceCompanyEmployees, ioengine.ImportCSV, "tr", "e.csv", bytes.NewBufferString(csv))
		if err != nil {
			t.Fatal(err)
		}
		return job.UUID
	}
	csv := "Ad,Soyad,E-posta,Unvan,Ofis,Grup,Telefon\n" +
		"Davetli,Çalışan," + strings.ToUpper(address) + ",Satış Temsilcisi,İstanbul,Satış,+90 555 000 11 22\n"

	// Preview: the row is valid and would create a profile; nothing is written or sent.
	job := upload(csv)
	pv, err := imports.Preview(e.ctx, job, c.owner.ID, c.account.ID)
	if err != nil || pv.PreviewSummary.Valid != 1 || pv.PreviewSummary.Rows[0].Op != "create" {
		t.Fatalf("preview: %+v %v", pv.PreviewSummary, err)
	}
	if len(mails.tokens()) != 0 {
		t.Fatal("preview sent mail")
	}

	// Apply then roll back: profile and invite are both removed.
	if applied, err := imports.Confirm(e.ctx, job, c.owner.ID, c.account.ID); err != nil || applied.PreviewSummary.Valid != 1 {
		t.Fatalf("apply: %+v %v", applied, err)
	}
	first, err := e.d.Queries.GetPendingAccountInviteByEmail(e.ctx, db.GetPendingAccountInviteByEmailParams{AccountID: c.account.ID, Email: address})
	if err != nil || !first.ProfileUuid.Valid {
		t.Fatalf("invite after apply: %+v %v", first, err)
	}
	if rolled, err := imports.Rollback(e.ctx, job, c.owner.ID, c.account.ID); err != nil || rolled.Status != "rolled_back" {
		t.Fatalf("rollback: %+v %v", rolled, err)
	}
	if _, err := e.d.Queries.GetPendingAccountInviteByEmail(e.ctx, db.GetPendingAccountInviteByEmailParams{AccountID: c.account.ID, Email: address}); err == nil {
		t.Fatal("rollback kept the invite")
	}
	if pv, err := invites.Preview(e.ctx, mails.tokens()[0]); err == nil {
		t.Fatalf("rolled back link still works: %+v", pv)
	}
	if _, err := e.profiles.Get(e.ctx, principal(c.account, c.owner, "owner"), uuid.UUID(first.ProfileUuid.Bytes)); err == nil {
		t.Fatal("rollback kept the created profile")
	}

	// Apply again: the card profile exists at once, the invite carries it.
	job = upload(csv)
	if _, err := imports.Preview(e.ctx, job, c.owner.ID, c.account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := imports.Confirm(e.ctx, job, c.owner.ID, c.account.ID); err != nil {
		t.Fatal(err)
	}
	inv, err := e.d.Queries.GetPendingAccountInviteByEmail(e.ctx, db.GetPendingAccountInviteByEmailParams{AccountID: c.account.ID, Email: address})
	if err != nil || !inv.ProfileUuid.Valid || uuid.UUID(inv.OfficeUuid.Bytes) != ist.Uuid || uuid.UUID(inv.GroupUuid.Bytes) != sales.Uuid {
		t.Fatalf("invite: %+v %v", inv, err)
	}
	profileID := uuid.UUID(inv.ProfileUuid.Bytes)
	created := e.detail(c, profileID)
	if created.Translations[0].FirstName != "Davetli" || created.Translations[0].CompanyName != "Inviter Co" {
		t.Fatalf("profile: %+v", created.Translations)
	}
	if len(mails.tokens()) != 2 {
		t.Fatalf("mails: %v", mails.urls)
	}

	// Re-importing the same person updates the invite's profile; no second email.
	job = upload("Ad,Soyad,E-posta,Unvan\nDavetli,Yeni," + address + ",Müdür\n")
	pv, err = imports.Preview(e.ctx, job, c.owner.ID, c.account.ID)
	if err != nil || pv.PreviewSummary.Rows[0].Op != "update" {
		t.Fatalf("re-import preview: %+v %v", pv.PreviewSummary, err)
	}
	if _, err := imports.Confirm(e.ctx, job, c.owner.ID, c.account.ID); err != nil {
		t.Fatal(err)
	}
	if got := e.detail(c, profileID).Translations[0]; got.LastName != "Yeni" || got.Title != "Müdür" {
		t.Fatalf("re-import profile: %+v", got)
	}
	if len(mails.tokens()) != 2 {
		t.Fatalf("re-import mailed again: %v", mails.urls)
	}

	// The person signs up and accepts: the profile becomes theirs and they are placed.
	var u pgtest.User
	if err := e.d.Pool.QueryRow(e.ctx, `INSERT INTO users (email, password_hash, name, surname, status)
		VALUES ($1, 'x', 'Davetli', 'Çalışan', 'active') RETURNING id, uuid`, address).Scan(&u.ID, &u.UUID); err != nil {
		t.Fatal(err)
	}
	res, err := invites.Accept(e.ctx, u.ID, mails.tokens()[1])
	if err != nil || res.ProfileUUID == nil || *res.ProfileUUID != profileID || res.Role != "member" {
		t.Fatalf("accept: %+v %v", res, err)
	}
	m, err := e.d.Queries.GetCompanyMemberByUserID(e.ctx, db.GetCompanyMemberByUserIDParams{AccountID: c.account.ID, UserID: u.ID})
	if err != nil {
		t.Fatal(err)
	}
	link, err := e.d.Queries.GetCompanyMemberLink(e.ctx, db.GetCompanyMemberLinkParams{AccountID: c.account.ID, MemberID: m.MemberID})
	if err != nil || link.ProfileUuid != profileID {
		t.Fatalf("link: %+v %v", link, err)
	}
	asg, _ := e.d.Queries.GetStructureAssignment(e.ctx, db.GetStructureAssignmentParams{AccountID: c.account.ID, Uuid: u.UUID})
	if uuid.UUID(asg.OfficeUuid.Bytes) != ist.Uuid || uuid.UUID(asg.GroupUuid.Bytes) != sales.Uuid {
		t.Fatalf("assignment: %+v", asg)
	}

	// Now registered: a later import updates the linked profile as a member row, no email.
	job = upload("Ad,Soyad,E-posta\nDavetli,Son," + address + "\n")
	if _, err := imports.Preview(e.ctx, job, c.owner.ID, c.account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := imports.Confirm(e.ctx, job, c.owner.ID, c.account.ID); err != nil {
		t.Fatal(err)
	}
	if got := e.detail(c, profileID).Translations[0].LastName; got != "Son" || len(mails.tokens()) != 2 {
		t.Fatalf("member row: %q, mails %d", got, len(mails.tokens()))
	}
}
