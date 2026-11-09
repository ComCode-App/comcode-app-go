package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/providers"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/mail"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	testCode    = "483920"
	testInvite  = "https://app.example.test/platform/invite/tok3n-secret"
	testSubject = "Code {{code}}"
	testBody    = "Hello {{name}}, your code is {{code}}. Accept: {{invite_url}}"
)

// sealQuerier keeps one row the way the database would.
type sealQuerier struct {
	pushTestQuerier
	row     db.Notification
	created int
}

func (q *sealQuerier) GetTemplateByCodeChannelLang(context.Context, db.GetTemplateByCodeChannelLangParams) (db.NotificationTemplate, error) {
	return db.NotificationTemplate{Subject: testSubject, Body: testBody}, nil
}

func (q *sealQuerier) GetUserByID(context.Context, int64) (db.User, error) { return db.User{}, nil }

func (q *sealQuerier) InsertNotificationHistory(context.Context, db.InsertNotificationHistoryParams) (db.NotificationHistory, error) {
	return db.NotificationHistory{}, nil
}

func (q *sealQuerier) CreateNotification(_ context.Context, arg db.CreateNotificationParams) (db.Notification, error) {
	q.created++
	q.row = db.Notification{
		ID: 1, Channel: arg.Channel, Status: arg.Status, Title: arg.Title, Body: arg.Body,
		Recipient: arg.Recipient, TemplateCode: arg.TemplateCode, SealedContent: arg.SealedContent,
		Payload: arg.Payload, ActionUrl: arg.ActionUrl,
	}
	return q.row, nil
}

func (q *sealQuerier) MarkNotificationProcessing(context.Context, int64) (db.Notification, error) {
	q.row.Status = model.StatusProcessing
	return q.row, nil
}

func (q *sealQuerier) MarkNotificationFailed(_ context.Context, arg db.MarkNotificationFailedParams) (db.Notification, error) {
	q.row.Status = model.StatusFailed
	q.row.LastError = arg.LastError
	return q.row, nil
}

func (q *sealQuerier) MarkNotificationSent(_ context.Context, arg db.MarkNotificationSentParams) (db.Notification, error) {
	q.row.Status = arg.Status
	q.row.SealedContent = pgtype.Text{}
	return q.row, nil
}

type recordingSender struct{ sent []mail.Message }

func (r *recordingSender) Send(_ context.Context, msg mail.Message) error {
	r.sent = append(r.sent, msg)
	return nil
}

type discardQueue struct{}

func (discardQueue) Enqueue(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error) {
	return &asynq.TaskInfo{}, nil
}

func testBox(t *testing.T) *crypto.SecretBox {
	t.Helper()
	box, err := crypto.NewSecretBox(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func secretInput() model.EnqueueInput {
	to := "person@example.test"
	link := testInvite
	return model.EnqueueInput{
		Channels: []string{model.ChannelEmail}, Recipient: &to, ActionURL: &link,
		TemplateCode: "auth.email_verification", SecurityEmail: true,
		SecretVars:   []string{"invite_url"},
		Payload:      map[string]any{"echo": testCode},
		TemplateVars: map[string]string{"name": "Ada", "code": testCode, "invite_url": testInvite},
	}
}

func assertNoSecret(t *testing.T, where string, values ...string) {
	t.Helper()
	for _, v := range values {
		if strings.Contains(v, testCode) || strings.Contains(v, "tok3n-secret") {
			t.Fatalf("%s leaks a secret: %q", where, v)
		}
	}
}

func TestSecretVarsAreMaskedOnTheRowAndSealedForDelivery(t *testing.T) {
	t.Parallel()

	q := &sealQuerier{}
	sender := &recordingSender{}
	svc := New(q, discardQueue{}, []providers.Provider{providers.EmailProvider{Mail: sender}}, slog.Default()).
		WithSecretBox(testBox(t))
	items, err := svc.Enqueue(context.Background(), secretInput())
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if !q.row.SealedContent.Valid {
		t.Fatal("sealed_content not set")
	}
	assertNoSecret(t, "queued row", q.row.Title, q.row.Body, q.row.SealedContent.String, string(q.row.Payload))
	if q.row.ActionUrl.Valid {
		t.Fatalf("action_url carrying the secret was kept: %q", q.row.ActionUrl.String)
	}
	if want := "Hello Ada, your code is " + SecretMask + ". Accept: " + SecretMask; q.row.Body != want {
		t.Fatalf("row body = %q, want %q", q.row.Body, want)
	}
	assertNoSecret(t, "API projection", items[0].Title, items[0].Body)

	if err := svc.Deliver(context.Background(), 1); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sent = %d", len(sender.sent))
	}
	msg := sender.sent[0]
	if msg.Subject != "Code "+testCode || !strings.Contains(msg.Body, testCode) || !strings.Contains(msg.Body, testInvite) {
		t.Fatalf("mailed %q / %q", msg.Subject, msg.Body)
	}
	if q.row.SealedContent.Valid {
		t.Fatal("sealed_content kept after send")
	}
	assertNoSecret(t, "sent row", q.row.Title, q.row.Body)
}

func TestSecretWithoutSecretBoxFailsClosed(t *testing.T) {
	t.Parallel()

	q := &sealQuerier{}
	svc := New(q, discardQueue{}, nil, slog.Default())
	if _, err := svc.Enqueue(context.Background(), secretInput()); !errors.Is(err, ErrSecretBoxMissing) {
		t.Fatalf("Enqueue error = %v, want ErrSecretBoxMissing", err)
	}
	if q.created != 0 {
		t.Fatal("row written without encryption")
	}
}

func TestUndecryptableRowIsNotSentMasked(t *testing.T) {
	t.Parallel()

	q := &sealQuerier{row: db.Notification{
		ID: 1, Channel: model.ChannelEmail, Status: model.StatusQueued, Title: "t", Body: SecretMask,
		Recipient:     pgtype.Text{String: "person@example.test", Valid: true},
		SealedContent: pgtype.Text{String: "bm90LXZhbGlk", Valid: true},
	}}
	sender := &recordingSender{}
	svc := New(q, discardQueue{}, []providers.Provider{providers.EmailProvider{Mail: sender}}, slog.Default()).
		WithSecretBox(testBox(t))
	if err := svc.Deliver(context.Background(), 1); err == nil {
		t.Fatal("Deliver: want an error")
	}
	if len(sender.sent) != 0 {
		t.Fatalf("sent = %d, want 0", len(sender.sent))
	}
}

func TestOrdinaryEmailKeepsItsContent(t *testing.T) {
	t.Parallel()

	q := &sealQuerier{}
	svc := New(q, discardQueue{}, nil, slog.Default())
	in := secretInput()
	in.SecurityEmail, in.SecretVars = false, nil
	if _, err := svc.Enqueue(context.Background(), in); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if q.row.SealedContent.Valid || !strings.Contains(q.row.Body, testCode) {
		t.Fatalf("ordinary email changed: %+v", q.row)
	}
}

func TestSecretBoxForEnvRefusesDevKeyOutsideDevelopment(t *testing.T) {
	t.Parallel()

	for _, env := range []string{"production", "staging"} {
		for _, key := range []string{"", devEncryptionKey} {
			if box, err := NewSecretBoxForEnv(env, key); err == nil || box != nil {
				t.Fatalf("%s with key %q: box %v err %v", env, key, box, err)
			}
		}
		if box, err := NewSecretBoxForEnv(env, strings.Repeat("z", 32)); err != nil || box == nil {
			t.Fatalf("%s with a real key: %v", env, err)
		}
	}
	if box, err := NewSecretBoxForEnv("development", devEncryptionKey); err != nil || box == nil {
		t.Fatalf("development key in development: %v", err)
	}
	svc := New(&sealQuerier{}, discardQueue{}, nil, slog.Default()).WithEncryptionKey("production", "")
	if svc.box != nil {
		t.Fatal("production without a key must leave the box unset")
	}
}
