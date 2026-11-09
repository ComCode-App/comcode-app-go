// Package usecase implements outbound webhooks (TEC-603): endpoint management, fan-out of
// domain events into deliveries, signed HTTP delivery with retries, and manual resends.
package usecase

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound       = errors.New("webhook not found")
	ErrForbidden      = errors.New("account role cannot manage webhooks")
	ErrInvalidEvents  = errors.New("at least one known event is required")
	ErrTooMany        = errors.New("webhook endpoint limit reached")
	ErrDeliveryActive = errors.New("delivery is still pending")
)

const (
	// MaxEndpointsPerAccount bounds fan-out per event.
	MaxEndpointsPerAccount = 10
	// MaxAttempts per delivery: 8 tries over roughly 9 hours with the backoff below.
	MaxAttempts = 8
	// DefaultRetention of delivery rows (see Prune).
	DefaultRetention = 30 * 24 * time.Hour
	maxResponseBytes = 4 * 1024
	// MaxPendingPerEndpoint caps the queue of one endpoint: a dead receiver or a scan flood cannot
	// grow the table without bound; further events are dropped (logged) until it drains.
	MaxPendingPerEndpoint = 500
)

// Enqueuer schedules delivery tasks (queue.Client in production).
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Service is the webhooks usecase.
type Service struct {
	q             *db.Queries
	box           *crypto.SecretBox
	queue         Enqueuer
	client        *http.Client
	log           *slog.Logger
	allowInsecure bool
	now           func() time.Time
}

// New builds the service. queue may be nil in tests: deliveries are then created but not sent.
func New(q *db.Queries, box *crypto.SecretBox, queueClient Enqueuer, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		q: q, box: box, queue: queueClient, log: log.With("component", "webhooks"),
		client: NewHTTPClient(false), now: func() time.Time { return time.Now().UTC() },
	}
}

// AllowInsecure accepts http:// and private hosts (local development and tests only).
func (s *Service) AllowInsecure(allow bool) *Service {
	s.allowInsecure = allow
	s.client = NewHTTPClient(allow)
	return s
}

// Events returns the registry.
func (s *Service) Events() []model.EventDefinition { return model.Registry }

// List returns the account's endpoints, newest first.
func (s *Service) List(ctx context.Context, account authctx.Account) ([]model.Endpoint, error) {
	rows, err := s.q.ListWebhookEndpoints(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Endpoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapEndpoint(row))
	}
	return out, nil
}

// Get returns one endpoint of the account.
func (s *Service) Get(ctx context.Context, account authctx.Account, id uuid.UUID) (model.Endpoint, error) {
	row, err := s.q.GetWebhookEndpointByUUID(ctx, db.GetWebhookEndpointByUUIDParams{Uuid: id, AccountID: account.ID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Endpoint{}, ErrNotFound
		}
		return model.Endpoint{}, err
	}
	return mapEndpoint(row), nil
}

// Create registers an endpoint and returns it with the secret, which is shown once.
func (s *Service) Create(ctx context.Context, account authctx.Account, userID int64, in model.EndpointInput) (model.EndpointWithSecret, error) {
	if !account.CanWrite() {
		return model.EndpointWithSecret{}, ErrForbidden
	}
	url, events, err := s.validate(in)
	if err != nil {
		return model.EndpointWithSecret{}, err
	}
	n, err := s.q.CountWebhookEndpoints(ctx, account.ID)
	if err != nil {
		return model.EndpointWithSecret{}, err
	}
	if n >= MaxEndpointsPerAccount {
		return model.EndpointWithSecret{}, ErrTooMany
	}
	secret, enc, hint, err := s.newSecret()
	if err != nil {
		return model.EndpointWithSecret{}, err
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	row, err := s.q.CreateWebhookEndpoint(ctx, db.CreateWebhookEndpointParams{
		AccountID: account.ID, Url: url, Description: strings.TrimSpace(in.Description), SecretEnc: enc, SecretHint: hint,
		Events: events, IsActive: active, CreatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
	})
	if err != nil {
		return model.EndpointWithSecret{}, err
	}
	return model.EndpointWithSecret{Endpoint: mapEndpoint(row), Secret: secret}, nil
}

// Update replaces the URL, description, events and active flag.
func (s *Service) Update(ctx context.Context, account authctx.Account, id uuid.UUID, in model.EndpointInput) (model.Endpoint, error) {
	if !account.CanWrite() {
		return model.Endpoint{}, ErrForbidden
	}
	url, events, err := s.validate(in)
	if err != nil {
		return model.Endpoint{}, err
	}
	current, err := s.Get(ctx, account, id)
	if err != nil {
		return model.Endpoint{}, err
	}
	active := current.IsActive
	if in.IsActive != nil {
		active = *in.IsActive
	}
	row, err := s.q.UpdateWebhookEndpoint(ctx, db.UpdateWebhookEndpointParams{
		Uuid: id, AccountID: account.ID, Url: url, Description: strings.TrimSpace(in.Description), Events: events, IsActive: active,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Endpoint{}, ErrNotFound
		}
		return model.Endpoint{}, err
	}
	return mapEndpoint(row), nil
}

// RotateSecret replaces the signing secret and returns the new one once. Deliveries created
// before the rotation that are still retrying are signed with the new secret from now on.
func (s *Service) RotateSecret(ctx context.Context, account authctx.Account, id uuid.UUID) (model.EndpointWithSecret, error) {
	if !account.CanWrite() {
		return model.EndpointWithSecret{}, ErrForbidden
	}
	secret, enc, hint, err := s.newSecret()
	if err != nil {
		return model.EndpointWithSecret{}, err
	}
	row, err := s.q.RotateWebhookEndpointSecret(ctx, db.RotateWebhookEndpointSecretParams{Uuid: id, AccountID: account.ID, SecretEnc: enc, SecretHint: hint})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.EndpointWithSecret{}, ErrNotFound
		}
		return model.EndpointWithSecret{}, err
	}
	return model.EndpointWithSecret{Endpoint: mapEndpoint(row), Secret: secret}, nil
}

// Delete soft-deletes an endpoint; its deliveries stay as history until pruned.
func (s *Service) Delete(ctx context.Context, account authctx.Account, id uuid.UUID) error {
	if !account.CanWrite() {
		return ErrForbidden
	}
	n, err := s.q.DeleteWebhookEndpoint(ctx, db.DeleteWebhookEndpointParams{Uuid: id, AccountID: account.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SendTest queues a webhook.test delivery to one endpoint, whatever its subscriptions.
func (s *Service) SendTest(ctx context.Context, account authctx.Account, id uuid.UUID) (model.Delivery, error) {
	if !account.CanWrite() {
		return model.Delivery{}, ErrForbidden
	}
	row, err := s.q.GetWebhookEndpointByUUID(ctx, db.GetWebhookEndpointByUUIDParams{Uuid: id, AccountID: account.ID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Delivery{}, ErrNotFound
		}
		return model.Delivery{}, err
	}
	data := map[string]any{"message": "ComCode webhook test", "endpoint_uuid": row.Uuid.String(), "sent_at": s.now().Format(time.RFC3339)}
	d, err := s.createDelivery(ctx, row, model.EventWebhookTest, uuid.New(), data)
	if err != nil {
		return model.Delivery{}, err
	}
	return mapDelivery(d), nil
}

// ListDeliveries pages the delivery log of an endpoint, newest first.
func (s *Service) ListDeliveries(ctx context.Context, account authctx.Account, id uuid.UUID, status string, limit, offset int32) (model.DeliveryPage, error) {
	if _, err := s.Get(ctx, account, id); err != nil {
		return model.DeliveryPage{}, err
	}
	var st db.NullWebhookDeliveryStatus
	if status != "" {
		st = db.NullWebhookDeliveryStatus{WebhookDeliveryStatus: db.WebhookDeliveryStatus(status), Valid: true}
	}
	rows, err := s.q.ListWebhookDeliveries(ctx, db.ListWebhookDeliveriesParams{
		AccountID: account.ID, EndpointUuid: id, Status: st, PageLimit: limit, PageOffset: offset,
	})
	if err != nil {
		return model.DeliveryPage{}, err
	}
	page := model.DeliveryPage{Items: make([]model.Delivery, 0, len(rows)), Limit: limit, Offset: offset}
	for _, row := range rows {
		page.Total = row.Total
		page.Items = append(page.Items, mapDelivery(db.WebhookDelivery{
			ID: row.ID, Uuid: row.Uuid, EndpointID: row.EndpointID, AccountID: row.AccountID, EventName: row.EventName,
			EventID: row.EventID, Payload: row.Payload, Status: row.Status, Attempts: row.Attempts, MaxAttempts: row.MaxAttempts,
			NextAttemptAt: row.NextAttemptAt, LastStatusCode: row.LastStatusCode, LastError: row.LastError,
			LastAttemptAt: row.LastAttemptAt, DeliveredAt: row.DeliveredAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return page, nil
}

// Resend queues a delivery again from attempt zero (failed, exhausted or succeeded ones).
func (s *Service) Resend(ctx context.Context, account authctx.Account, endpointID, deliveryID uuid.UUID) (model.Delivery, error) {
	if !account.CanWrite() {
		return model.Delivery{}, ErrForbidden
	}
	row, err := s.q.GetWebhookDeliveryByUUID(ctx, db.GetWebhookDeliveryByUUIDParams{Uuid: deliveryID, AccountID: account.ID, EndpointUuid: endpointID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Delivery{}, ErrNotFound
		}
		return model.Delivery{}, err
	}
	// A pending or failed delivery still has a scheduled task; a second task would double-post.
	if row.Status == db.WebhookDeliveryStatusPending || row.Status == db.WebhookDeliveryStatusFailed {
		return model.Delivery{}, ErrDeliveryActive
	}
	reset, err := s.q.ResetWebhookDelivery(ctx, row.ID)
	if err != nil {
		return model.Delivery{}, err
	}
	s.enqueue(ctx, reset.ID, reset.Attempts, 0)
	return mapDelivery(reset), nil
}

// Sweep re-queues deliveries whose task was lost (enqueue failed, worker crashed) and are long
// overdue. The task id carries the attempt number, so a task that still exists is not duplicated.
func (s *Service) Sweep(ctx context.Context) (int, error) {
	rows, err := s.q.ListStaleWebhookDeliveries(ctx)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		s.enqueue(ctx, row.ID, row.Attempts, 0)
	}
	return len(rows), nil
}

// Dispatch fans one domain event out to every active endpoint of the account subscribed to it.
// It is fail-soft: callers already committed their own write, so errors are logged, never
// returned. Cheap when the account has no endpoints (one indexed query).
func (s *Service) Dispatch(ctx context.Context, accountID int64, eventName string, data map[string]any) {
	if s == nil || accountID <= 0 || !model.IsSubscribable(eventName) {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	endpoints, err := s.q.ListActiveWebhookEndpointsForEvent(ctx, db.ListActiveWebhookEndpointsForEventParams{AccountID: accountID, EventName: eventName})
	if err != nil {
		s.log.Warn("webhook_dispatch_lookup_failed", "account_id", accountID, "event", eventName, "error", err)
		return
	}
	if len(endpoints) == 0 {
		return
	}
	eventID := uuid.New()
	for _, ep := range endpoints {
		if _, err := s.createDelivery(ctx, ep, eventName, eventID, data); err != nil {
			s.log.Warn("webhook_dispatch_failed", "account_id", accountID, "event", eventName, "endpoint_uuid", ep.Uuid, "error", err)
		}
	}
}

func (s *Service) createDelivery(ctx context.Context, ep db.WebhookEndpoint, eventName string, eventID uuid.UUID, data map[string]any) (db.WebhookDelivery, error) {
	if data == nil {
		data = map[string]any{}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return db.WebhookDelivery{}, fmt.Errorf("webhook payload: %w", err)
	}
	if n, err := s.q.CountPendingWebhookDeliveries(ctx, ep.ID); err != nil {
		return db.WebhookDelivery{}, err
	} else if n >= MaxPendingPerEndpoint {
		return db.WebhookDelivery{}, fmt.Errorf("endpoint has %d undelivered events, dropping %s", n, eventName)
	}
	row, err := s.q.CreateWebhookDelivery(ctx, db.CreateWebhookDeliveryParams{
		EndpointID: ep.ID, AccountID: ep.AccountID, EventName: eventName, EventID: eventID, Payload: raw, MaxAttempts: MaxAttempts,
	})
	if err != nil {
		return db.WebhookDelivery{}, err
	}
	s.enqueue(ctx, row.ID, row.Attempts, 0)
	return row, nil
}

// enqueue schedules one attempt. The task id is unique per delivery and attempt number, so a
// sweep cannot add a second task for an attempt that is still queued (asynq.ErrTaskIDConflict).
func (s *Service) enqueue(ctx context.Context, deliveryID int64, attempt int32, delay time.Duration) {
	if s.queue == nil {
		return
	}
	task, err := queue.NewWebhookDeliverTask(deliveryID)
	if err != nil {
		s.log.Error("webhook_task_build_failed", "delivery_id", deliveryID, "error", err)
		return
	}
	opts := []asynq.Option{
		asynq.Queue(queue.QueueWebhooks), asynq.MaxRetry(0), asynq.Timeout(60 * time.Second),
		asynq.TaskID(fmt.Sprintf("webhook:%d:%d", deliveryID, attempt)),
	}
	if delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
	}
	if _, err := s.queue.Enqueue(queue.WithRequestID(ctx, task), opts...); err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
		s.log.Error("webhook_enqueue_failed", "delivery_id", deliveryID, "error", err)
	}
}

// Deliver is the worker entry point: POST the delivery, record the attempt and schedule the
// next one on failure. It returns nil in every case except a load error, because retries are
// scheduled here rather than by Asynq.
func (s *Service) Deliver(ctx context.Context, deliveryID int64) error {
	row, err := s.q.GetWebhookDeliveryByID(ctx, deliveryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("webhook delivery %d: %w", deliveryID, err)
	}
	if row.Status != db.WebhookDeliveryStatusPending && row.Status != db.WebhookDeliveryStatusFailed {
		return nil
	}
	// Claim this attempt: a duplicate or early task finds nothing to do.
	if n, err := s.q.ClaimWebhookDelivery(ctx, row.ID); err != nil {
		return fmt.Errorf("claim webhook delivery %d: %w", deliveryID, err)
	} else if n == 0 {
		return nil
	}
	if row.EndpointDeletedAt.Valid || !row.EndpointIsActive {
		_, err := s.q.RecordWebhookDeliveryAttempt(ctx, db.RecordWebhookDeliveryAttemptParams{
			ID: row.ID, Status: db.WebhookDeliveryStatusExhausted, NextAttemptAt: row.NextAttemptAt, LastError: "endpoint inactive",
		})
		return err
	}
	secret, err := s.box.Decrypt(row.EndpointSecretEnc)
	if err != nil {
		// An undecryptable secret (key rotation) cannot be retried; the user rotates the secret.
		_, recErr := s.q.RecordWebhookDeliveryAttempt(ctx, db.RecordWebhookDeliveryAttemptParams{
			ID: row.ID, Status: db.WebhookDeliveryStatusExhausted, NextAttemptAt: row.NextAttemptAt, LastError: "secret unavailable: rotate the endpoint secret",
		})
		s.log.Error("webhook_secret_decrypt_failed", "delivery_id", row.ID, "error", err)
		return recErr
	}
	var data map[string]any
	_ = json.Unmarshal(row.Payload, &data)
	body, err := json.Marshal(model.Envelope{
		ID: row.Uuid, Event: row.EventName, EventID: row.EventID, OccurredAt: row.CreatedAt.Time.UTC(), Attempt: row.Attempts + 1, Data: data,
	})
	if err != nil {
		return err
	}
	statusCode, sendErr := s.post(ctx, row.EndpointUrl, secret, row, body)

	succeeded := sendErr == nil
	status := db.WebhookDeliveryStatusSucceeded
	next := row.NextAttemptAt
	errText := ""
	if !succeeded {
		errText = truncate(sendErr.Error(), 500)
		attempt := row.Attempts + 1
		if attempt >= row.MaxAttempts {
			status = db.WebhookDeliveryStatusExhausted
		} else {
			status = db.WebhookDeliveryStatusFailed
			delay := Backoff(attempt)
			next = pgtype.Timestamptz{Time: s.now().Add(delay), Valid: true}
			defer s.enqueue(ctx, row.ID, attempt, delay)
		}
	}
	code := pgtype.Int4{}
	if statusCode > 0 {
		code = pgtype.Int4{Int32: int32(statusCode), Valid: true}
	}
	if _, err := s.q.RecordWebhookDeliveryAttempt(ctx, db.RecordWebhookDeliveryAttemptParams{
		ID: row.ID, Status: status, NextAttemptAt: next, LastStatusCode: code, LastError: errText,
	}); err != nil {
		return fmt.Errorf("record webhook attempt: %w", err)
	}
	if err := s.q.TouchWebhookEndpointDelivery(ctx, db.TouchWebhookEndpointDeliveryParams{ID: row.EndpointID, Succeeded: succeeded}); err != nil {
		s.log.Warn("webhook_endpoint_touch_failed", "endpoint_id", row.EndpointID, "error", err)
	}
	s.log.Info("webhook_delivery_attempt", "delivery_uuid", row.Uuid, "endpoint_uuid", row.EndpointUuid, "event", row.EventName,
		"status", status, "http_status", statusCode, "attempt", row.Attempts+1, "error", errText)
	return nil
}

// post sends the body and returns the HTTP status (0 when no response) and an error for anything
// but a 2xx.
func (s *Service) post(ctx context.Context, url, secret string, row db.GetWebhookDeliveryByIDRow, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set(HeaderEvent, row.EventName)
	req.Header.Set(HeaderDelivery, row.Uuid.String())
	req.Header.Set(HeaderEventID, row.EventID.String())
	req.Header.Set(HeaderSignature, Sign(secret, s.now(), body))
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, fmt.Errorf("endpoint answered %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// Backoff is the wait before attempt n+1 (n = attempts so far): 30s, 1m, 2m, 4m, ... capped at 6h.
func Backoff(attempt int32) time.Duration {
	d := 30 * time.Second
	for i := int32(1); i < attempt && d < 6*time.Hour; i++ {
		d *= 2
	}
	if d > 6*time.Hour {
		d = 6 * time.Hour
	}
	return d
}

// Prune removes deliveries older than the retention window.
func (s *Service) Prune(ctx context.Context, retention time.Duration) (int64, error) {
	if retention <= 0 {
		retention = DefaultRetention
	}
	return s.q.PruneWebhookDeliveries(ctx, pgtype.Timestamptz{Time: s.now().Add(-retention), Valid: true})
}

func (s *Service) validate(in model.EndpointInput) (string, []string, error) {
	url, err := ValidateURL(in.URL, s.allowInsecure)
	if err != nil {
		return "", nil, err
	}
	seen := map[string]bool{}
	events := make([]string, 0, len(in.Events))
	for _, name := range in.Events {
		name = strings.TrimSpace(name)
		if !model.IsSubscribable(name) {
			return "", nil, fmt.Errorf("%w: unknown event %q", ErrInvalidEvents, name)
		}
		if !seen[name] {
			seen[name] = true
			events = append(events, name)
		}
	}
	if len(events) == 0 {
		return "", nil, ErrInvalidEvents
	}
	return url, events, nil
}

// newSecret returns a fresh 32-byte hex secret ("whsec_" prefixed), its ciphertext and hint.
func (s *Service) newSecret() (secret, enc, hint string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", "", err
	}
	secret = "whsec_" + hex.EncodeToString(buf)
	enc, err = s.box.Encrypt(secret)
	if err != nil {
		return "", "", "", fmt.Errorf("encrypt webhook secret: %w", err)
	}
	return secret, enc, secret[len(secret)-6:], nil
}

func mapEndpoint(row db.WebhookEndpoint) model.Endpoint {
	events := row.Events
	if events == nil {
		events = []string{}
	}
	return model.Endpoint{
		UUID: row.Uuid, URL: row.Url, Description: row.Description, Events: events, IsActive: row.IsActive,
		SecretHint: row.SecretHint, ConsecutiveFailures: row.ConsecutiveFailures,
		LastDeliveryAt: timePtr(row.LastDeliveryAt), LastSuccessAt: timePtr(row.LastSuccessAt),
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func mapDelivery(row db.WebhookDelivery) model.Delivery {
	var payload map[string]any
	_ = json.Unmarshal(row.Payload, &payload)
	if payload == nil {
		payload = map[string]any{}
	}
	d := model.Delivery{
		UUID: row.Uuid, EventName: row.EventName, EventID: row.EventID, Payload: payload, Status: string(row.Status),
		Attempts: row.Attempts, MaxAttempts: row.MaxAttempts, LastError: row.LastError,
		NextAttemptAt: timePtr(row.NextAttemptAt), LastAttemptAt: timePtr(row.LastAttemptAt), DeliveredAt: timePtr(row.DeliveredAt),
		CreatedAt: row.CreatedAt.Time.UTC(),
	}
	if row.Status == db.WebhookDeliveryStatusSucceeded || row.Status == db.WebhookDeliveryStatusExhausted {
		d.NextAttemptAt = nil
	}
	if row.LastStatusCode.Valid {
		code := row.LastStatusCode.Int32
		d.LastStatusCode = &code
	}
	return d
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
