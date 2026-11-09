package usecase_test

import (
	"bytes"
	"io"
	"log/slog"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	accountsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/usecase"
	importusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
)

// TestIntegrationEmployeeImportUnverifiedUserIsInvited: a row whose email belongs to a user who
// never verified it (anyone can register someone else's address while sign-up is open) must not
// add that user to the company. The row is invited instead, so only the mailbox owner can join
// (TEC-1104). A verified user is still added directly.
func TestIntegrationEmployeeImportUnverifiedUserIsInvited(t *testing.T) {
	e := newEnv(t)
	mails := &inviteMails{}
	invites := accountsusecase.NewInvites(e.d.Pool, e.d.Queries, mails, "https://app.example.test", nil)
	adapter := usecase.NewImportAdapter(e.d.Queries, e.profiles)
	adapter.SetInviter(invites)
	imports := importusecase.New(e.d.Queries, memDriver{m: storage.NewMemory()}, ioengine.NewRegistry(adapter), nil, nil, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	c := e.company("Squat Co")
	squatter := e.d.CreateUnverifiedUser(t)
	verified := e.d.CreateUser(t)
	emailOf := func(id int64) string {
		var s string
		if err := e.d.Pool.QueryRow(e.ctx, `SELECT email FROM users WHERE id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	csv := "Ad,Soyad,E-posta\n" +
		"Gerçek,Kişi," + emailOf(squatter.ID) + "\n" +
		"Doğrulanmış,Kişi," + emailOf(verified.ID) + "\n"
	job, err := imports.Upload(e.ctx, c.owner.ID, c.account.ID, model.ResourceCompanyEmployees, ioengine.ImportCSV, "tr", "e.csv", bytes.NewBufferString(csv))
	if err != nil {
		t.Fatal(err)
	}
	if pv, err := imports.Preview(e.ctx, job.UUID, c.owner.ID, c.account.ID); err != nil || pv.PreviewSummary.Valid != 2 {
		t.Fatalf("preview: %+v %v", pv.PreviewSummary, err)
	}
	if applied, err := imports.Confirm(e.ctx, job.UUID, c.owner.ID, c.account.ID); err != nil || applied.PreviewSummary.Valid != 2 {
		t.Fatalf("apply: %+v %v", applied, err)
	}

	if _, err := e.d.Queries.GetAccountMemberIDByUser(e.ctx, db.GetAccountMemberIDByUserParams{AccountID: c.account.ID, UserID: squatter.ID}); err == nil {
		t.Fatal("unverified user was added to the company")
	}
	inv, err := e.d.Queries.GetPendingAccountInviteByEmail(e.ctx, db.GetPendingAccountInviteByEmailParams{AccountID: c.account.ID, Email: emailOf(squatter.ID)})
	if err != nil || !inv.ProfileUuid.Valid {
		t.Fatalf("expected an invite carrying the profile: %+v %v", inv, err)
	}
	if n := len(mails.tokens()); n != 1 {
		t.Fatalf("expected one invite email, got %d", n)
	}
	if _, err := e.d.Queries.GetAccountMemberIDByUser(e.ctx, db.GetAccountMemberIDByUserParams{AccountID: c.account.ID, UserID: verified.ID}); err != nil {
		t.Fatalf("verified user should be added directly: %v", err)
	}
}
