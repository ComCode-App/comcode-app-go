package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// expoTestStore is the notification querier plus the push send store, in memory.
type expoTestStore struct {
	pushTestQuerier
	rows     map[int64]db.Notification
	user     db.User
	devices  []db.PushDevice
	tickets  []db.InsertPushTicketParams
	disabled []int64
	due      []db.ListDuePushTicketsRow
	deleted  []int64
}

func (s *expoTestStore) GetNotificationByID(_ context.Context, id int64) (db.Notification, error) {
	r, ok := s.rows[id]
	if !ok {
		return db.Notification{}, pgx.ErrNoRows
	}
	return r, nil
}
func (s *expoTestStore) GetUserByID(context.Context, int64) (db.User, error) { return s.user, nil }
func (s *expoTestStore) UserHasActivePushDevices(context.Context, int64) (bool, error) {
	return len(s.devices) > 0, nil
}
func (s *expoTestStore) ListPushTargetsForNotification(context.Context, db.ListPushTargetsForNotificationParams) ([]db.PushDevice, error) {
	return s.devices, nil
}
func (s *expoTestStore) DisablePushDeviceNotRegistered(_ context.Context, id int64) (int64, error) {
	s.disabled = append(s.disabled, id)
	return 1, nil
}
func (s *expoTestStore) InsertPushTicket(_ context.Context, arg db.InsertPushTicketParams) error {
	s.tickets = append(s.tickets, arg)
	return nil
}
func (s *expoTestStore) ListDuePushTickets(context.Context, db.ListDuePushTicketsParams) ([]db.ListDuePushTicketsRow, error) {
	out := s.due
	s.due = nil
	return out, nil
}
func (s *expoTestStore) DeletePushTickets(_ context.Context, ids []int64) error {
	s.deleted = append(s.deleted, ids...)
	return nil
}
func (s *expoTestStore) DeleteExpiredPushTickets(context.Context) (int64, error)  { return 0, nil }
func (s *expoTestStore) DeleteDisabledPushDevices(context.Context) (int64, error) { return 0, nil }
func (s *expoTestStore) DeleteStalePushDevices(context.Context) (int64, error)    { return 0, nil }

const testAccessToken = "expo-access-token-secret-value"

func newExpoStore(devices int) *expoTestStore {
	st := &expoTestStore{
		pushTestQuerier: pushTestQuerier{prefs: db.NotificationPreference{PushEnabled: true}},
		rows:            map[int64]db.Notification{},
		user:            db.User{ID: 7, Locale: "tr"},
	}
	for i := range devices {
		st.devices = append(st.devices, db.PushDevice{
			ID: int64(i + 1), Uuid: uuid.New(), UserID: 7, Platform: "ios",
			Token:  fmt.Sprintf("ExponentPushToken[device-%03d]", i),
			Locale: pgtype.Text{String: "en", Valid: i%2 == 0},
		})
	}
	st.rows[1] = db.Notification{
		ID: 1, Uuid: uuid.New(), UserID: pgtype.Int8{Int64: 7, Valid: true}, Channel: model.ChannelInapp,
		Title: "Ayşe Yılmaz sizinle iletişime geçti", Body: "Telefon: 0532 123 45 67",
		TemplateCode: pgtype.Text{String: "leads.received", Valid: true},
		ActionUrl:    pgtype.Text{String: "/leads?lead=abc", Valid: true},
		Payload:      []byte(`{"lead_uuid":"abc","account_uuid":"5b1d0c1e-0000-4000-8000-000000000001"}`),
		Priority:     model.PriorityNormal,
	}
	return st
}

// fakeExpo answers /push/send with one ticket per message (ok, or the error named by the
// token) and /push/getReceipts with the receipts in its map. fail holds status codes returned
// before the first success.
type fakeExpo struct {
	mu       sync.Mutex
	batches  [][]expoMessage
	auth     []string
	fail     []int
	calls    int
	receipts map[string]expoTicket
}

func (f *fakeExpo) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls++
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		if len(f.fail) > 0 {
			code := f.fail[0]
			f.fail = f.fail[1:]
			if code == http.StatusTooManyRequests {
				w.Header().Set("Retry-After", "0")
			}
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"errors":[{"code":"X","message":"ExponentPushToken[leaky] down"}]}`))
			return
		}
		switch r.URL.Path {
		case "/--/api/v2/push/send":
			var msgs []expoMessage
			if err := json.NewDecoder(r.Body).Decode(&msgs); err != nil {
				t.Errorf("decode send: %v", err)
			}
			f.batches = append(f.batches, msgs)
			data := make([]map[string]any, 0, len(msgs))
			for _, m := range msgs {
				if strings.Contains(m.To, "big") {
					data = append(data, map[string]any{
						"status": "error", "message": `"` + m.To + `" message is too big`,
						"details": map[string]string{"error": "MessageTooBig"},
					})
					continue
				}
				if strings.Contains(m.To, "dead") {
					data = append(data, map[string]any{
						"status": "error", "message": `"` + m.To + `" is not a registered push notification recipient`,
						"details": map[string]string{"error": "DeviceNotRegistered"},
					})
					continue
				}
				data = append(data, map[string]any{"status": "ok", "id": "ticket-" + m.To})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
		case "/--/api/v2/push/getReceipts":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": f.receipts})
		default:
			http.NotFound(w, r)
		}
	}
}

func newExpoService(t *testing.T, st *expoTestStore, f *fakeExpo, logBuf *bytes.Buffer) *Service {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	log := slog.New(slog.NewTextHandler(logBuf, nil))
	return New(st, nil, nil, log).WithExpo(ExpoConfig{
		AccessToken: testAccessToken, BaseURL: srv.URL + "/--/api/v2", HTTP: srv.Client(),
		MaxAttempts: 3, Backoff: time.Millisecond,
	})
}

func TestSendMobilePushBatchesOf100(t *testing.T) {
	st, f, logs := newExpoStore(250), &fakeExpo{}, &bytes.Buffer{}
	svc := newExpoService(t, st, f, logs)
	if err := svc.SendMobilePush(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if len(f.batches) != 3 || len(f.batches[0]) != 100 || len(f.batches[1]) != 100 || len(f.batches[2]) != 50 {
		t.Fatalf("batches = %d (%v)", len(f.batches), batchSizes(f.batches))
	}
	if len(st.tickets) != 250 || st.tickets[0].TicketID != "ticket-ExponentPushToken[device-000]" || st.tickets[249].PushDeviceID != 250 {
		t.Fatalf("tickets = %d %+v", len(st.tickets), st.tickets[:1])
	}
	for _, a := range f.auth {
		if a != "Bearer "+testAccessToken {
			t.Fatalf("authorization header = %q", a)
		}
	}
	// Lock-screen text from the catalog in the device locale; data is {url, notification_uuid, account_uuid}.
	en, tr := f.batches[0][0], f.batches[0][1]
	if en.Title != "New contact request" || tr.Title != "Yeni iletişim talebi" {
		t.Fatalf("titles en=%q tr=%q", en.Title, tr.Title)
	}
	for _, m := range []expoMessage{en, tr} {
		if strings.Contains(m.Title+m.Body, "Ayşe") || strings.Contains(m.Title+m.Body, "0532") {
			t.Fatalf("in-app text leaked into the push: %+v", m)
		}
	}
	want := map[string]any{
		"url": "/leads?lead=abc", "notification_uuid": st.rows[1].Uuid.String(),
		"account_uuid": "5b1d0c1e-0000-4000-8000-000000000001",
	}
	if len(en.Data) != len(want) {
		t.Fatalf("data = %v, want %v", en.Data, want)
	}
	for k, v := range want {
		if en.Data[k] != v {
			t.Fatalf("data[%s] = %v, want %v", k, en.Data[k], v)
		}
	}
}

func TestSendMobilePushWithoutAccessTokenSendsNoAuthorization(t *testing.T) {
	st, f := newExpoStore(1), &fakeExpo{}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	svc := New(st, nil, nil, slog.New(slog.DiscardHandler)).WithExpo(ExpoConfig{BaseURL: srv.URL + "/--/api/v2"})
	if err := svc.SendMobilePush(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if len(f.auth) != 1 || f.auth[0] != "" {
		t.Fatalf("authorization = %q, want none", f.auth)
	}
}

func TestSendMobilePushRetriesOn429And5xx(t *testing.T) {
	st, f, logs := newExpoStore(2), &fakeExpo{fail: []int{http.StatusServiceUnavailable, http.StatusTooManyRequests}}, &bytes.Buffer{}
	svc := newExpoService(t, st, f, logs)
	if err := svc.SendMobilePush(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if f.calls != 3 || len(st.tickets) != 2 {
		t.Fatalf("calls = %d tickets = %d, want 3 and 2", f.calls, len(st.tickets))
	}

	// Still failing after the client's attempts: a retryable error for Asynq, without the body.
	f.fail = []int{502, 502, 502}
	st.tickets = nil
	err := svc.SendMobilePush(context.Background(), 1)
	if err == nil || errors.Is(err, asynq.SkipRetry) || strings.Contains(err.Error(), "leaky") {
		t.Fatalf("err = %v, want retryable without the body", err)
	}

	// A 4xx other than 429 is not retried.
	f.fail, f.calls = []int{http.StatusBadRequest}, 0
	err = svc.SendMobilePush(context.Background(), 1)
	if f.calls != 1 || !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("400: calls = %d err = %v, want one call and SkipRetry", f.calls, err)
	}
}

func TestSendMobilePushTicketDeviceNotRegisteredDisablesDevice(t *testing.T) {
	st, f, logs := newExpoStore(3), &fakeExpo{}, &bytes.Buffer{}
	st.devices[1].Token = "ExponentPushToken[dead-phone]"
	svc := newExpoService(t, st, f, logs)
	if err := svc.SendMobilePush(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if len(st.disabled) != 1 || st.disabled[0] != st.devices[1].ID || len(st.tickets) != 2 {
		t.Fatalf("disabled = %v tickets = %d", st.disabled, len(st.tickets))
	}
}

func TestProcessPushReceiptsDisablesDeviceNotRegistered(t *testing.T) {
	st, logs := newExpoStore(0), &bytes.Buffer{}
	old := pgtype.Timestamptz{Time: time.Now().Add(-20 * time.Minute), Valid: true}
	st.due = []db.ListDuePushTicketsRow{
		{ID: 1, TicketID: "t-ok", PushDeviceID: 10, CreatedAt: old},
		{ID: 2, TicketID: "t-dead", PushDeviceID: 11, CreatedAt: old},
		{ID: 3, TicketID: "t-pending", PushDeviceID: 12, CreatedAt: old},
		{ID: 4, TicketID: "t-other", PushDeviceID: 13, CreatedAt: old},
	}
	f := &fakeExpo{receipts: map[string]expoTicket{
		"t-ok": {Status: "ok"},
		"t-dead": {Status: "error", Message: `"ExponentPushToken[secret-dead]" is not registered`, Details: struct {
			Error string `json:"error"`
		}{"DeviceNotRegistered"}},
		"t-other": {Status: "error", Message: `"ExponentPushToken[secret-other]" too big`, Details: struct {
			Error string `json:"error"`
		}{"MessageTooBig"}},
	}}
	svc := newExpoService(t, st, f, logs)
	n, err := svc.ProcessPushReceipts(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("disabled = %d err = %v", n, err)
	}
	if len(st.disabled) != 1 || st.disabled[0] != 11 {
		t.Fatalf("disabled devices = %v", st.disabled)
	}
	// The pending receipt stays for the next run; the others are done.
	if fmt.Sprint(st.deleted) != "[1 2 4]" {
		t.Fatalf("deleted tickets = %v", st.deleted)
	}
	if strings.Contains(logs.String(), "secret-other") || !strings.Contains(logs.String(), "MessageTooBig") {
		t.Fatalf("logs: %s", logs.String())
	}
}

// Security emails, sealed rows, codes without a push text and push_enabled off are never pushed.
func TestSendMobilePushSkipsIneligibleRows(t *testing.T) {
	st, f, logs := newExpoStore(2), &fakeExpo{}, &bytes.Buffer{}
	base := st.rows[1]
	email := base
	email.ID, email.Channel = 2, model.ChannelEmail
	email.TemplateCode = pgtype.Text{String: "auth.password_reset", Valid: true}
	sealed := base
	sealed.ID, sealed.SealedContent = 3, pgtype.Text{String: "v1:sealed", Valid: true}
	security := base
	security.ID, security.TemplateCode = 4, pgtype.Text{String: "auth.login_code", Valid: true}
	custom := base
	custom.ID, custom.TemplateCode = 5, pgtype.Text{String: "admin.custom", Valid: true}
	noCode := base
	noCode.ID, noCode.TemplateCode = 6, pgtype.Text{}
	for _, r := range []db.Notification{email, sealed, security, custom, noCode} {
		st.rows[r.ID] = r
	}
	svc := newExpoService(t, st, f, logs)
	for id := int64(2); id <= 6; id++ {
		if err := svc.SendMobilePush(context.Background(), id); err != nil {
			t.Fatal(err)
		}
		svc.scheduleMobilePush(context.Background(), st.rows[id])
	}
	st.prefs.PushEnabled = false
	if err := svc.SendMobilePush(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if f.calls != 0 {
		t.Fatalf("expo called %d times for ineligible rows", f.calls)
	}
}

func TestSendMobilePushLogsNeverCarryTokens(t *testing.T) {
	st, f, logs := newExpoStore(1), &fakeExpo{}, &bytes.Buffer{}
	st.devices[0].Token = "ExponentPushToken[big-private]"
	svc := newExpoService(t, st, f, logs)
	// The ticket error message quotes the device token.
	if err := svc.SendMobilePush(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "MessageTooBig") {
		t.Fatalf("ticket error not logged: %s", logs.String())
	}
	f.fail = []int{500, 500, 500}
	err := svc.SendMobilePush(context.Background(), 1)
	svc.log.Error("push_send_failed", "error", err.Error())
	out := logs.String()
	for _, secret := range []string{"private", testAccessToken, "leaky"} {
		if strings.Contains(out, secret) {
			t.Fatalf("logs contain %q: %s", secret, out)
		}
	}
}

// Deliver queues the push of an in-app row on the push queue (worker), with a task id.
func TestDeliverQueuesMobilePush(t *testing.T) {
	st, logs := newExpoStore(1), &bytes.Buffer{}
	enq := &recordingEnqueuer{}
	svc := newExpoService(t, st, &fakeExpo{}, logs).WithPushQueue(enq)
	svc.scheduleMobilePush(context.Background(), st.rows[1])
	if len(enq.tasks) != 1 || enq.tasks[0] != "app:push:send" {
		t.Fatalf("tasks = %v", enq.tasks)
	}
	st.devices = nil
	svc.scheduleMobilePush(context.Background(), st.rows[1])
	if len(enq.tasks) != 1 {
		t.Fatalf("queued without an active device: %v", enq.tasks)
	}
}

type recordingEnqueuer struct{ tasks []string }

func (r *recordingEnqueuer) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	r.tasks = append(r.tasks, task.Type())
	return &asynq.TaskInfo{}, nil
}

func TestActionPath(t *testing.T) {
	cases := map[string]string{
		"/leads?lead=1":                  "/leads?lead=1",
		"https://app.example.com/orders": "/orders",
		"//evil.example.com/x":           "",
		"javascript:alert(1)":            "",
	}
	for in, want := range cases {
		if got := actionPath(pgtype.Text{String: in, Valid: true}); got != want {
			t.Errorf("actionPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func batchSizes(b [][]expoMessage) []int {
	out := make([]int, 0, len(b))
	for _, x := range b {
		out = append(out, len(x))
	}
	return out
}
