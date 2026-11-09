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
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// platformGetQuerier serves the row written by Enqueue back by uuid.
type platformGetQuerier struct {
	sealQuerier
	owner db.User
}

func (q *platformGetQuerier) GetNotificationByUUID(_ context.Context, id uuid.UUID) (db.Notification, error) {
	if q.row.Uuid != id {
		return db.Notification{}, pgx.ErrNoRows
	}
	return q.row, nil
}

func (q *platformGetQuerier) GetUserByID(_ context.Context, id int64) (db.User, error) {
	if id != q.owner.ID {
		return db.User{}, pgx.ErrNoRows
	}
	return q.owner, nil
}

// enqueueSecretEmail stores a security email for user 42 the way production does.
func enqueueSecretEmail(t *testing.T) (*platformGetQuerier, *Service, uuid.UUID) {
	t.Helper()
	q := &platformGetQuerier{owner: db.User{
		ID: 42, Uuid: uuid.New(), Name: "Ada", Surname: "Lovelace", Email: "person@example.test",
	}}
	svc := New(q, discardQueue{}, []providers.Provider{providers.EmailProvider{Mail: &recordingSender{}}}, slog.Default()).
		WithSecretBox(testBox(t)).
		WithActionSigner("action-secret", 0)
	in := secretInput()
	owner := q.owner.ID
	in.UserID = &owner
	if _, err := svc.Enqueue(context.Background(), in); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := uuid.New()
	q.row.Uuid = id
	q.row.UserID = pgtype.Int8{Int64: owner, Valid: true}
	return q, svc, id
}

func TestGetPlatformReadsEmailRowMasked(t *testing.T) {
	t.Parallel()

	q, svc, id := enqueueSecretEmail(t)
	if q.row.Channel != model.ChannelEmail || !q.row.SealedContent.Valid {
		t.Fatalf("fixture is not a sealed email row: %+v", q.row)
	}

	// The inbox endpoint keeps answering 404 for email rows, even to the owner.
	if _, err := svc.GetOwned(context.Background(), q.owner.ID, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetOwned(email) error = %v, want ErrNotFound", err)
	}

	admin := authctx.Principal{
		UserInternal: 7,
		Permissions:  []string{rbac.PermPlatformNotificationsRead, rbac.PermPlatformNotificationsReadAll},
	}
	n, err := svc.GetPlatform(context.Background(), admin, id)
	if err != nil {
		t.Fatalf("GetPlatform: %v", err)
	}
	if n.UUID != id || n.Channel != model.ChannelEmail {
		t.Fatalf("got %+v", n)
	}
	if !strings.Contains(n.Body, SecretMask) {
		t.Fatalf("body is not the masked rendering: %q", n.Body)
	}
	var action, signed string
	if n.ActionURL != nil {
		action = *n.ActionURL
	}
	if n.SignedActionURL != nil {
		signed = *n.SignedActionURL
	}
	assertNoSecret(t, "platform detail", n.Title, n.Body, action, signed, mustString(n.Payload))
	if n.User == nil || n.User.UUID != q.owner.Uuid {
		t.Fatalf("user = %+v, want owner", n.User)
	}
}

func TestGetPlatformWithoutReadAllSeesOnlyOwnRows(t *testing.T) {
	t.Parallel()

	q, svc, id := enqueueSecretEmail(t)
	operator := authctx.Principal{UserInternal: 7, Permissions: []string{rbac.PermPlatformNotificationsRead}}
	if _, err := svc.GetPlatform(context.Background(), operator, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user's row: error = %v, want ErrNotFound", err)
	}

	self := authctx.Principal{UserInternal: q.owner.ID, Permissions: []string{rbac.PermPlatformNotificationsRead}}
	n, err := svc.GetPlatform(context.Background(), self, id)
	if err != nil {
		t.Fatalf("own row: %v", err)
	}
	assertNoSecret(t, "own platform detail", n.Title, n.Body)

	if _, err := svc.GetPlatform(context.Background(), self, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing row: error = %v, want ErrNotFound", err)
	}
}

func mustString(v map[string]any) string { return string(mustJSON(v)) }
