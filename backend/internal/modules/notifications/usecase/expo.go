package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/pushtext"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/logredact"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Mobile push through the Expo Push Service (TEC-1187, mobile B10). Devices are registered by
// push_devices.go (B9). A delivered in-app (or push-channel) row whose code has a pushtext entry
// is queued as app:push:send; the worker sends it to the user's live devices in batches of 100
// and stores the tickets; app:push:receipts reads the receipts every 15 minutes.

const (
	// DefaultExpoBaseURL is the Expo push API root; tests point ExpoConfig.BaseURL at a fake.
	DefaultExpoBaseURL = "https://exp.host/--/api/v2"

	expoSendPath     = "/push/send"
	expoReceiptsPath = "/push/getReceipts"

	expoSendBatch     = 100
	expoReceiptsBatch = 1000
	// Receipts are read about 15 minutes after sending; Expo keeps them about 24 hours.
	expoReceiptDelay  = 15 * time.Minute
	expoReceiptMaxAge = 24 * time.Hour
	// One receipts run handles at most this many batches; the next run continues.
	expoReceiptsMaxRounds = 20

	expoDefaultAttempts = 3
	expoDefaultBackoff  = time.Second
	expoMaxBackoff      = 30 * time.Second

	expoErrDeviceNotRegistered = "DeviceNotRegistered"
	expoErrInvalidCredentials  = "InvalidCredentials"
)

// errExpoPermanent marks an Expo answer that a retry cannot fix (4xx other than 429).
var errExpoPermanent = errors.New("expo: request rejected")

// ExpoConfig configures the Expo push client. AccessToken is optional: it is needed only when
// "enhanced push security" is on for the Expo project, and is sent as a Bearer header.
type ExpoConfig struct {
	AccessToken string
	// BaseURL defaults to DefaultExpoBaseURL.
	BaseURL string
	HTTP    *http.Client
	// MaxAttempts per request on 429 / 5xx / network errors (default 3); Backoff is the first
	// wait (default 1s, doubled per attempt; a Retry-After header wins, capped at 30s).
	MaxAttempts int
	Backoff     time.Duration
}

type expoClient struct {
	token    string
	base     string
	http     *http.Client
	attempts int
	backoff  time.Duration
}

// WithExpo enables mobile push. The client works without an access token.
func (s *Service) WithExpo(cfg ExpoConfig) *Service {
	c := &expoClient{
		token:    strings.TrimSpace(cfg.AccessToken),
		base:     strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		http:     cfg.HTTP,
		attempts: cfg.MaxAttempts,
		backoff:  cfg.Backoff,
	}
	if c.base == "" {
		c.base = DefaultExpoBaseURL
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: 20 * time.Second}
	}
	if c.attempts <= 0 {
		c.attempts = expoDefaultAttempts
	}
	if c.backoff <= 0 {
		c.backoff = expoDefaultBackoff
	}
	s.expo = c
	return s
}

// WithPushQueue queues mobile pushes (Asynq retries Expo outages) even where notification
// delivery itself runs inline, as in the worker.
func (s *Service) WithPushQueue(enq Enqueuer) *Service {
	s.pushQueue = enq
	return s
}

// pushSendStore is the persistence surface of sending, receipts and cleanup.
type pushSendStore interface {
	UserHasActivePushDevices(ctx context.Context, userID int64) (bool, error)
	ListPushTargetsForNotification(ctx context.Context, arg db.ListPushTargetsForNotificationParams) ([]db.PushDevice, error)
	DisablePushDeviceNotRegistered(ctx context.Context, id int64) (int64, error)
	InsertPushTicket(ctx context.Context, arg db.InsertPushTicketParams) error
	ListDuePushTickets(ctx context.Context, arg db.ListDuePushTicketsParams) ([]db.ListDuePushTicketsRow, error)
	DeletePushTickets(ctx context.Context, ids []int64) error
	DeleteExpiredPushTickets(ctx context.Context) (int64, error)
	DeleteDisabledPushDevices(ctx context.Context) (int64, error)
	DeleteStalePushDevices(ctx context.Context) (int64, error)
}

func (s *Service) pushSendStore() (pushSendStore, bool) {
	st, ok := s.q.(pushSendStore)
	return st, ok
}

// pushEligible reports whether a notification row may be pushed to a phone at all: an in-app or
// push-channel row with a pushtext entry. Rows holding sealed content (security emails,
// invitation links) are never pushed, whatever their code.
func pushEligible(row db.Notification) bool {
	if row.Channel != model.ChannelInapp && row.Channel != model.ChannelPush {
		return false
	}
	if row.SealedContent.Valid {
		return false
	}
	return row.TemplateCode.Valid && pushtext.Known(row.TemplateCode.String)
}

// scheduleMobilePush queues the Expo push of a delivered row when its user has an active device.
// Fail-soft: the notification itself is already delivered.
func (s *Service) scheduleMobilePush(ctx context.Context, row db.Notification) {
	if s.expo == nil || !row.UserID.Valid || !pushEligible(row) {
		return
	}
	st, ok := s.pushSendStore()
	if !ok {
		return
	}
	has, err := st.UserHasActivePushDevices(ctx, row.UserID.Int64)
	if err != nil {
		s.log.Error("push_device_check_failed", "notification_id", row.ID, "error", logredact.Error(err))
		return
	}
	if !has {
		return
	}
	enq := s.pushQueue
	if enq == nil && !s.syncMode {
		enq = s.queue
	}
	if enq == nil {
		if err := s.SendMobilePush(ctx, row.ID); err != nil {
			s.log.Error("push_send_failed", "notification_id", row.ID, "error", logredact.Error(err))
		}
		return
	}
	task, err := queue.NewPushSendTask(row.ID)
	if err == nil {
		_, err = enq.Enqueue(queue.WithRequestID(ctx, task), queue.PushSendOptions(row.ID)...)
	}
	if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
		s.log.Error("push_enqueue_failed", "notification_id", row.ID, "error", logredact.Error(err))
	}
}

type expoMessage struct {
	To       string         `json:"to"`
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	Data     map[string]any `json:"data"`
	Sound    string         `json:"sound,omitempty"`
	Priority string         `json:"priority,omitempty"`
}

type expoTicket struct {
	Status  string `json:"status"`
	ID      string `json:"id"`
	Message string `json:"message"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

// buildPushMessage renders the lock-screen message of a row for one device. Title and body come
// from pushtext only (never the in-app text). The locale is the device's, then the user's.
func buildPushMessage(row db.Notification, text pushtext.Text, data map[string]any, dev db.PushDevice) expoMessage {
	priority := "default"
	if row.Priority == model.PriorityHigh || row.Priority == model.PriorityCritical {
		priority = "high"
	}
	return expoMessage{To: dev.Token, Title: text.Title, Body: text.Body, Data: data, Sound: "default", Priority: priority}
}

// pushData is the tap payload: {url, notification_uuid, account_uuid}. url is the in-app path
// of the notification's action (an absolute action URL is reduced to its path); account_uuid
// lets the app switch to the account the notification belongs to.
func pushData(row db.Notification) map[string]any {
	data := map[string]any{"notification_uuid": row.Uuid.String()}
	if path := actionPath(row.ActionUrl); path != "" {
		data["url"] = path
	}
	var payload map[string]any
	if json.Unmarshal(row.Payload, &payload) == nil {
		if acc, ok := payload["account_uuid"].(string); ok && acc != "" {
			data["account_uuid"] = acc
		}
	}
	return data
}

func actionPath(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	raw := strings.TrimSpace(v.String)
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.RequestURI()
}

func pushLocale(dev db.PushDevice, userLocale string) string {
	if dev.Locale.Valid {
		if l := normalizePushLocale(dev.Locale.String); l != "" {
			return l
		}
	}
	if l := normalizePushLocale(userLocale); l != "" {
		return l
	}
	return "tr"
}

// SendMobilePush sends the Expo push of a notification row to the user's live devices (not
// disabled, session family still has a live refresh token) when push_enabled is on. Tickets are
// stored for the receipt check; DeviceNotRegistered disables the device. An error is retried
// by Asynq; devices that already hold a ticket for the row are not pushed again.
func (s *Service) SendMobilePush(ctx context.Context, notificationID int64) error {
	if s.expo == nil {
		return nil
	}
	st, ok := s.pushSendStore()
	if !ok {
		return nil
	}
	row, err := s.q.GetNotificationByID(ctx, notificationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !row.UserID.Valid || !pushEligible(row) {
		return nil
	}
	userID := row.UserID.Int64
	prefs, err := s.GetPreferences(ctx, userID)
	if err != nil {
		return err
	}
	if !prefs.PushEnabled {
		return nil
	}
	devs, err := st.ListPushTargetsForNotification(ctx, db.ListPushTargetsForNotificationParams{
		UserID: userID, NotificationID: pgtype.Int8{Int64: row.ID, Valid: true},
	})
	if err != nil || len(devs) == 0 {
		return err
	}
	userLocale := ""
	if u, err := s.q.GetUserByID(ctx, userID); err == nil {
		userLocale = u.Locale
	}
	data := pushData(row)
	msgs := make([]expoMessage, 0, len(devs))
	for _, d := range devs {
		text, _ := pushtext.For(row.TemplateCode.String, pushLocale(d, userLocale))
		msgs = append(msgs, buildPushMessage(row, text, data, d))
	}
	for start := 0; start < len(msgs); start += expoSendBatch {
		end := min(start+expoSendBatch, len(msgs))
		tickets, err := s.expo.send(ctx, msgs[start:end])
		if err != nil {
			return err
		}
		for i, t := range tickets {
			if start+i >= end {
				break
			}
			s.handleTicket(ctx, st, row, devs[start+i], t)
		}
	}
	return nil
}

func (s *Service) handleTicket(ctx context.Context, st pushSendStore, row db.Notification, dev db.PushDevice, t expoTicket) {
	switch {
	case t.Status == "ok" && t.ID != "":
		if err := st.InsertPushTicket(ctx, db.InsertPushTicketParams{
			TicketID: t.ID, PushDeviceID: dev.ID, NotificationID: pgtype.Int8{Int64: row.ID, Valid: true},
		}); err != nil {
			s.log.Error("push_ticket_store_failed", "device_uuid", dev.Uuid.String(), "error", logredact.Error(err))
		}
	case t.Details.Error == expoErrDeviceNotRegistered:
		if _, err := st.DisablePushDeviceNotRegistered(ctx, dev.ID); err != nil {
			s.log.Error("push_device_disable_failed", "device_uuid", dev.Uuid.String(), "error", logredact.Error(err))
		}
	case t.Details.Error == expoErrInvalidCredentials:
		// APNs / FCM credentials are missing in EAS: an operator problem, the device is fine.
		s.log.Error("expo_push_invalid_credentials", "device_uuid", dev.Uuid.String(), "platform", dev.Platform,
			"message", logredact.String(t.Message))
	default:
		s.log.Warn("expo_push_ticket_error", "device_uuid", dev.Uuid.String(), "error", t.Details.Error,
			"message", logredact.String(t.Message))
	}
}

// ProcessPushReceipts reads the receipts of tickets older than 15 minutes, disables devices
// reported as DeviceNotRegistered and deletes processed tickets. Returns the disabled count.
func (s *Service) ProcessPushReceipts(ctx context.Context) (int, error) {
	if s.expo == nil {
		return 0, nil
	}
	st, ok := s.pushSendStore()
	if !ok {
		return 0, nil
	}
	now := time.Now()
	disabled := 0
	for range expoReceiptsMaxRounds {
		rows, err := st.ListDuePushTickets(ctx, db.ListDuePushTicketsParams{
			BeforeAt:   pgtype.Timestamptz{Time: now.Add(-expoReceiptDelay), Valid: true},
			LimitCount: expoReceiptsBatch,
		})
		if err != nil || len(rows) == 0 {
			return disabled, err
		}
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.TicketID)
		}
		receipts, err := s.expo.receipts(ctx, ids)
		if err != nil {
			return disabled, err
		}
		var done []int64
		for _, r := range rows {
			rc, ok := receipts[r.TicketID]
			if !ok {
				// Not ready yet; given up once Expo no longer keeps it.
				if now.Sub(r.CreatedAt.Time) > expoReceiptMaxAge {
					done = append(done, r.ID)
				}
				continue
			}
			if rc.Status == "error" {
				switch rc.Details.Error {
				case expoErrDeviceNotRegistered:
					n, err := st.DisablePushDeviceNotRegistered(ctx, r.PushDeviceID)
					if err != nil {
						return disabled, err
					}
					disabled += int(n)
				case expoErrInvalidCredentials:
					s.log.Error("expo_push_invalid_credentials", "push_device_id", r.PushDeviceID,
						"message", logredact.String(rc.Message))
				default:
					s.log.Warn("expo_push_receipt_error", "push_device_id", r.PushDeviceID,
						"error", rc.Details.Error, "message", logredact.String(rc.Message))
				}
			}
			done = append(done, r.ID)
		}
		if len(done) == 0 {
			return disabled, nil
		}
		if err := st.DeletePushTickets(ctx, done); err != nil {
			return disabled, err
		}
		if len(rows) < expoReceiptsBatch {
			return disabled, nil
		}
	}
	return disabled, nil
}

// PushCleanup is what one maintenance run deleted.
type PushCleanup struct {
	Tickets         int64
	DisabledDevices int64
	StaleDevices    int64
}

// PrunePushData is the push part of the maintenance sweep: tickets older than 24 hours, devices
// disabled for more than 30 days and devices not seen for 270 days.
func (s *Service) PrunePushData(ctx context.Context) (PushCleanup, error) {
	var out PushCleanup
	st, ok := s.pushSendStore()
	if !ok {
		return out, nil
	}
	var err error
	if out.Tickets, err = st.DeleteExpiredPushTickets(ctx); err != nil {
		return out, fmt.Errorf("delete expired push tickets: %w", err)
	}
	if out.DisabledDevices, err = st.DeleteDisabledPushDevices(ctx); err != nil {
		return out, fmt.Errorf("delete disabled push devices: %w", err)
	}
	if out.StaleDevices, err = st.DeleteStalePushDevices(ctx); err != nil {
		return out, fmt.Errorf("delete stale push devices: %w", err)
	}
	return out, nil
}

// post sends one JSON request, retrying 429 / 5xx / network errors with backoff. Errors never
// carry the response body (it quotes device tokens) or the access token.
func (c *expoClient) post(ctx context.Context, path string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 1; attempt <= c.attempts; attempt++ {
		status, raw, retryAfter, err := c.do(ctx, c.base+path, buf)
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return fmt.Errorf("expo: %w", ctx.Err())
			}
			lastErr = fmt.Errorf("expo: request failed: %w", err)
		case status == http.StatusTooManyRequests || status >= 500:
			lastErr = fmt.Errorf("expo: http %d", status)
		case status >= 300:
			return fmt.Errorf("%w: http %d: %w", errExpoPermanent, status, asynq.SkipRetry)
		default:
			if err := json.Unmarshal(raw, out); err != nil {
				return fmt.Errorf("expo: decode response: %w", err)
			}
			return nil
		}
		if attempt == c.attempts {
			break
		}
		wait := c.backoff << (attempt - 1)
		if retryAfter > 0 {
			wait = retryAfter
		}
		wait = min(wait, expoMaxBackoff)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("expo: %w", ctx.Err())
		case <-timer.C:
		}
	}
	return lastErr
}

func (c *expoClient) do(ctx context.Context, endpoint string, buf []byte) (int, []byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return 0, nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return 0, nil, 0, err
	}
	var retryAfter time.Duration
	if secs, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After"))); err == nil && secs > 0 {
		retryAfter = time.Duration(secs) * time.Second
	}
	return resp.StatusCode, raw, retryAfter, nil
}

func (c *expoClient) send(ctx context.Context, msgs []expoMessage) ([]expoTicket, error) {
	var out struct {
		Data   []expoTicket `json:"data"`
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := c.post(ctx, expoSendPath, msgs, &out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 && len(out.Errors) > 0 {
		return nil, fmt.Errorf("%w: %s: %w", errExpoPermanent, out.Errors[0].Code, asynq.SkipRetry)
	}
	return out.Data, nil
}

func (c *expoClient) receipts(ctx context.Context, ids []string) (map[string]expoTicket, error) {
	var out struct {
		Data map[string]expoTicket `json:"data"`
	}
	if err := c.post(ctx, expoReceiptsPath, map[string]any{"ids": ids}, &out); err != nil {
		return nil, err
	}
	if out.Data == nil {
		out.Data = map[string]expoTicket{}
	}
	return out.Data, nil
}
