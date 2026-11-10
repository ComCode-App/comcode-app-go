package repository_test

// Postgres integration tests for QR sign-in (TEC-1184): the web session is issued on the
// personal account as a web session, and the approval notice template is seeded.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

type recordingNotifier struct {
	code string
	last notifmodel.EnqueueInput
}

func (n *recordingNotifier) Enqueue(_ context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	if in.TemplateCode == "auth.login_code" {
		n.code = in.TemplateVars["code"]
	} else {
		n.last = in
	}
	return nil, nil
}

func TestIntegrationQRLoginSession(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := repository.NewPostgres(d.Pool, d.Queries)
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uc := usecase.New(repo, tokens)
	uc.SetAuthSettings(openRegistration{})
	notif := &recordingNotifier{}
	uc.SetNotifier(notif)

	email := "qr-" + pgtest.Unique() + "@example.com"
	if err := uc.RequestLoginCode(ctx, email); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.VerifyLoginCode(ctx, email, notif.code, "", model.SessionMeta{}); err != nil {
		t.Fatal(err)
	}
	user, err := repo.FindUserByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	personal, err := repo.GetPersonalAccount(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := uc.CheckQRLoginApprover(ctx, user.UUID); err != nil {
		t.Fatalf("active approver: %v", err)
	}
	// Whatever the caller passes, the session is a web session on the personal account.
	other := int64(-1)
	tok, qrUser, err := uc.IssueQRLoginSession(ctx, user.UUID, model.SessionMeta{
		UserAgent: "Mozilla/5.0 Firefox/130.0", IP: "203.0.113.9",
		ActiveAccountID: &other, ClientType: model.ClientMobile, DeviceName: "phone",
	})
	if err != nil {
		t.Fatal(err)
	}
	if qrUser.UUID != user.UUID || qrUser.Email != email {
		t.Fatalf("user = %+v", qrUser)
	}
	claims, err := tokens.ParseAccess(tok.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.AccountUUID() != personal.UUID {
		t.Fatalf("account = %v, want personal %v", claims.AccountUUID(), personal.UUID)
	}
	var clientType, ip string
	var device *string
	if err := d.Pool.QueryRow(ctx, `SELECT client_type, ip_address::text, device_name FROM refresh_tokens
		WHERE user_id = $1 ORDER BY id DESC LIMIT 1`, user.ID).Scan(&clientType, &ip, &device); err != nil {
		t.Fatal(err)
	}
	if clientType != model.ClientWeb || device != nil || ip == "" {
		t.Fatalf("session row: client_type=%s ip=%s device=%v", clientType, ip, device)
	}

	uc.NotifyQRLoginApproved(ctx, user.UUID, usecase.QRLoginNotice{Device: "Firefox 130 · Windows", IP: "203.0.113.9"})
	if notif.last.TemplateCode != "auth.qr_login_approved" || !notif.last.SecurityEmail ||
		len(notif.last.Channels) != 2 || notif.last.TemplateVars["location"] != "-" {
		t.Fatalf("notice = %+v", notif.last)
	}
	var templates int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM notification_templates
		WHERE code = 'auth.qr_login_approved' AND active`).Scan(&templates); err != nil {
		t.Fatal(err)
	}
	if templates != 4 {
		t.Fatalf("templates = %d, want email+inapp in tr+en", templates)
	}

	if _, err := d.Pool.Exec(ctx, `UPDATE users SET status = 'disabled' WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := uc.CheckQRLoginApprover(ctx, user.UUID); !errors.Is(err, usecase.ErrUserDisabled) {
		t.Fatalf("disabled approver: %v", err)
	}
	if _, _, err := uc.IssueQRLoginSession(ctx, user.UUID, model.SessionMeta{}); !errors.Is(err, usecase.ErrUserDisabled) {
		t.Fatalf("disabled exchange: %v", err)
	}
}
