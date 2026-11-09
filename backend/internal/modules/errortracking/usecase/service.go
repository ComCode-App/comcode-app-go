// Package usecase manages the platform "Hata izleme" setting (TEC-952): the backend and
// frontend Sentry DSNs, stored encrypted in error_tracking_settings.
//
// Every API and worker process keeps the decrypted row in memory and applies the backend DSN
// to errreport. A change made through Patch reloads the local copy at once and publishes on a
// Redis channel so the other processes reload too; a periodic reload covers a missed message.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/errreport"
)

// ErrInvalidDSN wraps a DSN that does not parse.
var ErrInvalidDSN = errors.New("invalid dsn")

// ErrReportingOff is returned by SendTest when no backend DSN is active.
var ErrReportingOff = errors.New("error reporting is off")

// DefaultRefreshInterval bounds how long a process keeps a stale DSN when a Redis
// invalidation message is missed.
const DefaultRefreshInterval = time.Minute

// Backend DSN sources reported to the admin panel.
const (
	SourcePanel = "panel"
	SourceEnv   = "env"
	SourceOff   = "off"
)

type store interface {
	GetErrorTrackingSettings(ctx context.Context) (db.ErrorTrackingSetting, error)
	UpdateErrorTrackingSettings(ctx context.Context, arg db.UpdateErrorTrackingSettingsParams) (db.ErrorTrackingSetting, error)
}

// Options are the process-level inputs that do not live in the database.
type Options struct {
	// FallbackDSN is SENTRY_DSN, used while the panel has no backend DSN.
	FallbackDSN string
	Environment string
	Release     string
	// Channel is the Redis pub/sub channel for invalidations (per app env).
	Channel         string
	RefreshInterval time.Duration
}

// ChannelName returns the invalidation channel for an app env.
func ChannelName(env string) string {
	return fmt.Sprintf("app:%s:errortracking:changed", env)
}

type snapshot struct {
	enabled     bool
	backendDSN  string
	frontendDSN string
}

// Service reads and writes the setting and keeps errreport in sync with it.
type Service struct {
	q    store
	box  *crypto.SecretBox
	rdb  *redis.Client
	log  *slog.Logger
	opts Options

	cur atomic.Pointer[snapshot]
}

// New creates the service. rdb may be nil (no cross-process invalidation, polling only).
func New(q store, box *crypto.SecretBox, rdb *redis.Client, log *slog.Logger, opts Options) *Service {
	if log == nil {
		log = slog.Default()
	}
	if opts.RefreshInterval <= 0 {
		opts.RefreshInterval = DefaultRefreshInterval
	}
	opts.FallbackDSN = strings.TrimSpace(opts.FallbackDSN)
	return &Service{q: q, box: box, rdb: rdb, log: log, opts: opts}
}

// Settings is the admin payload. DSNs are never returned, only a masked hint.
type Settings struct {
	Enabled               bool      `json:"enabled"`
	BackendDSNConfigured  bool      `json:"backend_dsn_configured"`
	BackendDSNHint        string    `json:"backend_dsn_hint"`
	FrontendDSNConfigured bool      `json:"frontend_dsn_configured"`
	FrontendDSNHint       string    `json:"frontend_dsn_hint"`
	EnvFallbackConfigured bool      `json:"env_fallback_configured"`
	BackendSource         string    `json:"backend_source"`
	BackendActive         bool      `json:"backend_active"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// PatchInput is a partial update. A nil DSN keeps the stored value, "" clears it.
type PatchInput struct {
	Enabled     *bool   `json:"enabled"`
	BackendDSN  *string `json:"backend_dsn"`
	FrontendDSN *string `json:"frontend_dsn"`
}

// FrontendConfig is what the browser SDK needs. It is public: a frontend DSN only allows
// sending events, and every browser that loads the SDK sees it anyway.
type FrontendConfig struct {
	Enabled     bool   `json:"enabled"`
	DSN         string `json:"dsn,omitempty"`
	Environment string `json:"environment,omitempty"`
	Release     string `json:"release,omitempty"`
}

// Start loads the setting, applies it, and keeps it fresh until ctx ends.
func (s *Service) Start(ctx context.Context) {
	if err := s.Reload(ctx); err != nil {
		s.log.Warn("error_tracking_settings_load_failed", "error", err)
	}
	if s.rdb != nil && s.opts.Channel != "" {
		go s.subscribe(ctx)
	}
	go s.poll(ctx)
}

func (s *Service) subscribe(ctx context.Context) {
	sub := s.rdb.Subscribe(ctx, s.opts.Channel)
	defer func() { _ = sub.Close() }()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			if err := s.Reload(ctx); err != nil {
				s.log.Warn("error_tracking_settings_reload_failed", "error", err)
			}
		}
	}
}

func (s *Service) poll(ctx context.Context) {
	t := time.NewTicker(s.opts.RefreshInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.Reload(ctx); err != nil {
				s.log.Warn("error_tracking_settings_reload_failed", "error", err)
			}
		}
	}
}

// Reload reads the row, refreshes the in-memory copy, and applies the backend DSN.
func (s *Service) Reload(ctx context.Context) error {
	row, err := s.q.GetErrorTrackingSettings(ctx)
	if err != nil {
		return fmt.Errorf("load error tracking settings: %w", err)
	}
	snap, err := s.decrypt(row)
	if err != nil {
		return err
	}
	s.cur.Store(&snap)
	if err := errreport.SetDSN(s.effectiveBackendDSN(snap)); err != nil {
		return err
	}
	return nil
}

func (s *Service) decrypt(row db.ErrorTrackingSetting) (snapshot, error) {
	snap := snapshot{enabled: row.Enabled}
	var err error
	if snap.backendDSN, err = s.open(row.BackendDsnEnc); err != nil {
		return snapshot{}, fmt.Errorf("decrypt backend dsn: %w", err)
	}
	if snap.frontendDSN, err = s.open(row.FrontendDsnEnc); err != nil {
		return snapshot{}, fmt.Errorf("decrypt frontend dsn: %w", err)
	}
	return snap, nil
}

func (s *Service) open(v pgtype.Text) (string, error) {
	if !v.Valid || v.String == "" {
		return "", nil
	}
	return s.box.Decrypt(v.String)
}

func (s *Service) effectiveBackendDSN(snap snapshot) string {
	if !snap.enabled {
		return ""
	}
	if snap.backendDSN != "" {
		return snap.backendDSN
	}
	return s.opts.FallbackDSN
}

func (s *Service) backendSource(snap snapshot) string {
	switch {
	case !snap.enabled:
		return SourceOff
	case snap.backendDSN != "":
		return SourcePanel
	case s.opts.FallbackDSN != "":
		return SourceEnv
	default:
		return SourceOff
	}
}

func (s *Service) settings(row db.ErrorTrackingSetting, snap snapshot) Settings {
	return Settings{
		Enabled:               row.Enabled,
		BackendDSNConfigured:  snap.backendDSN != "",
		BackendDSNHint:        MaskDSN(snap.backendDSN),
		FrontendDSNConfigured: snap.frontendDSN != "",
		FrontendDSNHint:       MaskDSN(snap.frontendDSN),
		EnvFallbackConfigured: s.opts.FallbackDSN != "",
		BackendSource:         s.backendSource(snap),
		BackendActive:         errreport.Enabled(),
		UpdatedAt:             row.UpdatedAt.Time,
	}
}

// Get returns the masked setting, read from the database.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	row, err := s.q.GetErrorTrackingSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	snap, err := s.decrypt(row)
	if err != nil {
		return Settings{}, err
	}
	return s.settings(row, snap), nil
}

// Patch validates and stores the change, applies it here, and tells the other processes.
func (s *Service) Patch(ctx context.Context, in PatchInput) (Settings, error) {
	row, err := s.q.GetErrorTrackingSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	params := db.UpdateErrorTrackingSettingsParams{
		Enabled:        row.Enabled,
		BackendDsnEnc:  row.BackendDsnEnc,
		FrontendDsnEnc: row.FrontendDsnEnc,
	}
	if in.Enabled != nil {
		params.Enabled = *in.Enabled
	}
	if in.BackendDSN != nil {
		if params.BackendDsnEnc, err = s.seal("backend_dsn", *in.BackendDSN); err != nil {
			return Settings{}, err
		}
	}
	if in.FrontendDSN != nil {
		if params.FrontendDsnEnc, err = s.seal("frontend_dsn", *in.FrontendDSN); err != nil {
			return Settings{}, err
		}
	}
	row, err = s.q.UpdateErrorTrackingSettings(ctx, params)
	if err != nil {
		return Settings{}, err
	}
	if err := s.Reload(ctx); err != nil {
		s.log.Warn("error_tracking_settings_reload_failed", "error", err)
	}
	s.publish(ctx)
	snap, err := s.decrypt(row)
	if err != nil {
		return Settings{}, err
	}
	return s.settings(row, snap), nil
}

func (s *Service) seal(field, dsn string) (pgtype.Text, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return pgtype.Text{}, nil
	}
	if err := errreport.ValidateDSN(dsn); err != nil {
		return pgtype.Text{}, fmt.Errorf("%w: %s", ErrInvalidDSN, field)
	}
	enc, err := s.box.Encrypt(dsn)
	if err != nil {
		return pgtype.Text{}, err
	}
	return pgtype.Text{String: enc, Valid: true}, nil
}

func (s *Service) publish(ctx context.Context) {
	if s.rdb == nil || s.opts.Channel == "" {
		return
	}
	pctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := s.rdb.Publish(pctx, s.opts.Channel, "1").Err(); err != nil {
		// Other processes still pick the change up on their next periodic reload.
		s.log.Warn("error_tracking_settings_publish_failed", "error", err)
	}
}

// FrontendConfig returns the browser SDK config from memory (no database read).
func (s *Service) FrontendConfig() FrontendConfig {
	snap := s.cur.Load()
	if snap == nil || !snap.enabled || snap.frontendDSN == "" {
		return FrontendConfig{Enabled: false}
	}
	return FrontendConfig{
		Enabled:     true,
		DSN:         snap.frontendDSN,
		Environment: s.opts.Environment,
		Release:     s.opts.Release,
	}
}

// SendTest sends one test error through the active backend DSN and returns its event id.
func (s *Service) SendTest(ctx context.Context) (string, error) {
	id, ok := errreport.CaptureTest(ctx, "ComCode test error (Hata izleme)", map[string]string{
		"source": "platform_test",
	})
	if !ok {
		return "", ErrReportingOff
	}
	return id, nil
}

// MaskDSN keeps the scheme, host and project of a DSN and hides the key.
func MaskDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return "***"
	}
	return fmt.Sprintf("%s://***@%s%s", u.Scheme, u.Host, u.Path)
}
