package notifications_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	routeSecretCode = "483920"
	routeSecretLink = "https://app.example.test/platform/invite/tok3n-secret"
)

// rowQuerier holds one stored email row; calls the routes do not need panic via the nil interface.
type rowQuerier struct {
	usecase.Querier
	row   db.Notification
	owner db.User
}

func (q *rowQuerier) GetNotificationByUUID(_ context.Context, id uuid.UUID) (db.Notification, error) {
	if id != q.row.Uuid {
		return db.Notification{}, pgx.ErrNoRows
	}
	return q.row, nil
}

func (q *rowQuerier) GetUserByID(_ context.Context, id int64) (db.User, error) {
	if id != q.owner.ID {
		return db.User{}, pgx.ErrNoRows
	}
	return q.owner, nil
}

type principals map[string]authctx.Principal

func (p principals) LoadPrincipal(_ *http.Request, c jwt.Claims) (authctx.Principal, error) {
	return p[c.Subject], nil
}

func TestPlatformNotificationDetailRoute(t *testing.T) {
	t.Parallel()

	owner := db.User{ID: 42, Uuid: uuid.New(), Name: "Ada", Surname: "Lovelace", Email: "person@example.test"}
	// A security email as TEC-1099 stores it: masked row, real content only in sealed_content.
	row := db.Notification{
		ID: 1, Uuid: uuid.New(), UserID: pgtype.Int8{Int64: owner.ID, Valid: true},
		Channel: model.ChannelEmail, Status: model.StatusQueued, Priority: model.PriorityNormal,
		Title: "Code " + usecase.SecretMask, Body: "Your code is " + usecase.SecretMask + ". Accept: " + usecase.SecretMask,
		Payload:       []byte(`{}`),
		Recipient:     pgtype.Text{String: owner.Email, Valid: true},
		TemplateCode:  pgtype.Text{String: "auth.email_verification", Valid: true},
		SealedContent: pgtype.Text{String: routeSecretCode + " " + routeSecretLink, Valid: true},
	}
	q := &rowQuerier{row: row, owner: owner}
	svc := usecase.New(q, nil, nil, slog.Default())
	tokens, err := jwt.NewManager(strings.Repeat("s", 32), time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	adminID, ownerID, plainID := uuid.New(), owner.Uuid, uuid.New()
	mux := http.NewServeMux()
	notifications.RegisterRoutes(mux, handler.New(svc), tokens, principals{
		adminID.String(): {UserID: adminID, UserInternal: 7, Permissions: []string{
			rbac.PermNotificationsRead, rbac.PermPlatformNotificationsRead, rbac.PermPlatformNotificationsReadAll,
		}},
		ownerID.String(): {UserID: ownerID, UserInternal: owner.ID, Permissions: []string{rbac.PermNotificationsRead}},
		plainID.String(): {UserID: plainID, UserInternal: 9, Permissions: []string{rbac.PermNotificationsRead}},
	})

	get := func(user uuid.UUID, path string) *httptest.ResponseRecorder {
		t.Helper()
		token, _, err := tokens.IssueAccess(jwt.AccessInput{UserID: user, SessionID: uuid.New()})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	platformPath := "/v1/platform/notifications/" + row.Uuid.String()

	rec := get(adminID, platformPath)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin platform detail = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, routeSecretCode) || strings.Contains(body, "tok3n-secret") || strings.Contains(body, "sealed") {
		t.Fatalf("platform detail leaks sealed content: %s", body)
	}
	var env struct {
		Data model.Notification `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.UUID != row.Uuid || env.Data.Channel != model.ChannelEmail || env.Data.Body != row.Body {
		t.Fatalf("detail = %+v", env.Data)
	}
	if env.Data.User == nil || env.Data.User.UUID != owner.Uuid {
		t.Fatalf("user = %+v", env.Data.User)
	}

	// The inbox endpoint the page used before answers 404 for email rows.
	if rec := get(adminID, "/v1/notifications/"+row.Uuid.String()); rec.Code == http.StatusOK {
		t.Fatalf("inbox endpoint served an email row: %s", rec.Body.String())
	}

	// No platform permission: the route is forbidden, even for the row's owner.
	for _, user := range []uuid.UUID{plainID, ownerID} {
		if rec := get(user, platformPath); rec.Code != http.StatusForbidden {
			t.Fatalf("user without platform.notifications.read = %d, want 403", rec.Code)
		}
	}

	if rec := get(adminID, "/v1/platform/notifications/not-a-uuid"); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid uuid = %d, want 400", rec.Code)
	}
	if rec := get(adminID, "/v1/platform/notifications/"+uuid.New().String()); rec.Code != http.StatusNotFound {
		t.Fatalf("missing row = %d, want 404", rec.Code)
	}
	if rec := get(adminID, "/v1/platform/notifications/meta"); rec.Code != http.StatusOK {
		t.Fatalf("meta route shadowed: %d", rec.Code)
	}
}
