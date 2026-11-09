package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/errreport"
)

// Placeholder DSNs on a reserved test domain; nothing is ever sent to them.
const (
	panelDSN    = "https://panelkey@errors.example.test/11"
	frontendDSN = "https://browserkey@errors.example.test/12"
	envDSN      = "https://envkey@errors.example.test/10"
)

type memStore struct {
	mu  sync.Mutex
	row db.ErrorTrackingSetting
}

func newMemStore() *memStore {
	return &memStore{row: db.ErrorTrackingSetting{ID: 1, Enabled: true}}
}

func (m *memStore) GetErrorTrackingSettings(context.Context) (db.ErrorTrackingSetting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.row, nil
}

func (m *memStore) UpdateErrorTrackingSettings(_ context.Context, arg db.UpdateErrorTrackingSettingsParams) (db.ErrorTrackingSetting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.row.Enabled = arg.Enabled
	m.row.BackendDsnEnc = arg.BackendDsnEnc
	m.row.FrontendDsnEnc = arg.FrontendDsnEnc
	m.row.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return m.row, nil
}

func newBox(t *testing.T) *crypto.SecretBox {
	t.Helper()
	box, err := crypto.NewSecretBox("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func ptr[T any](v T) *T { return &v }

func TestBackendDSNPrecedence(t *testing.T) {
	t.Cleanup(func() { _ = errreport.SetDSN("") })
	ctx := context.Background()
	store := newMemStore()
	svc := New(store, newBox(t), nil, nil, Options{FallbackDSN: envDSN})

	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Get(ctx)
	if !errreport.Enabled() || got.BackendSource != SourceEnv {
		t.Fatalf("empty panel must fall back to SENTRY_DSN: %+v", got)
	}

	got, err := svc.Patch(ctx, PatchInput{BackendDSN: ptr(panelDSN)})
	if err != nil {
		t.Fatal(err)
	}
	if got.BackendSource != SourcePanel || !got.BackendDSNConfigured || !got.BackendActive {
		t.Fatalf("panel DSN must win: %+v", got)
	}
	if got.BackendDSNHint != "https://***@errors.example.test/11" {
		t.Fatalf("hint = %q", got.BackendDSNHint)
	}
	if store.row.BackendDsnEnc.String == panelDSN {
		t.Fatal("DSN stored in plain text")
	}

	got, _ = svc.Patch(ctx, PatchInput{Enabled: ptr(false)})
	if errreport.Enabled() || got.BackendSource != SourceOff || got.BackendActive {
		t.Fatalf("disabled toggle must turn reporting off, env included: %+v", got)
	}
	if _, err := svc.SendTest(ctx); !errors.Is(err, ErrReportingOff) {
		t.Fatalf("SendTest while off = %v", err)
	}

	got, _ = svc.Patch(ctx, PatchInput{Enabled: ptr(true), BackendDSN: ptr("")})
	if got.BackendDSNConfigured || got.BackendSource != SourceEnv {
		t.Fatalf("clearing the panel DSN must go back to env: %+v", got)
	}
}

func TestPatchRejectsInvalidDSN(t *testing.T) {
	svc := New(newMemStore(), newBox(t), nil, nil, Options{})
	_, err := svc.Patch(context.Background(), PatchInput{FrontendDSN: ptr("not-a-dsn")})
	if !errors.Is(err, ErrInvalidDSN) {
		t.Fatalf("err = %v", err)
	}
}

func TestFrontendConfig(t *testing.T) {
	t.Cleanup(func() { _ = errreport.SetDSN("") })
	ctx := context.Background()
	svc := New(newMemStore(), newBox(t), nil, nil, Options{Environment: "production", Release: "abc"})
	if svc.FrontendConfig().Enabled {
		t.Fatal("not loaded yet must be disabled")
	}
	if _, err := svc.Patch(ctx, PatchInput{FrontendDSN: ptr(frontendDSN)}); err != nil {
		t.Fatal(err)
	}
	cfg := svc.FrontendConfig()
	if !cfg.Enabled || cfg.DSN != frontendDSN || cfg.Environment != "production" || cfg.Release != "abc" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if _, err := svc.Patch(ctx, PatchInput{Enabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	if cfg := svc.FrontendConfig(); cfg.Enabled || cfg.DSN != "" {
		t.Fatalf("disabled must hide the DSN: %+v", cfg)
	}
}

// A change saved through one process reaches another one through Redis, without a restart.
func TestInvalidationReachesOtherProcess(t *testing.T) {
	t.Cleanup(func() { _ = errreport.SetDSN("") })
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := newMemStore()
	box := newBox(t)
	opts := Options{Channel: ChannelName("test"), RefreshInterval: time.Hour}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api := New(store, box, rdb, nil, opts)
	worker := New(store, box, rdb, nil, opts)
	worker.Start(ctx)

	// Wait until the subscription is registered, or the publish below is lost.
	deadline := time.Now().Add(5 * time.Second)
	for len(mr.PubSubChannels("")) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("worker never subscribed")
		}
		time.Sleep(5 * time.Millisecond)
	}

	if _, err := api.Patch(ctx, PatchInput{FrontendDSN: ptr(frontendDSN)}); err != nil {
		t.Fatal(err)
	}
	for !worker.FrontendConfig().Enabled {
		if time.Now().After(deadline) {
			t.Fatal("worker did not reload after the invalidation message")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestMaskDSN(t *testing.T) {
	cases := map[string]string{
		"":                                   "",
		"https://abc@errors.example.test/42": "https://***@errors.example.test/42",
		"http://k:s@host.test:9000/sub/7":    "http://***@host.test:9000/sub/7",
		"garbage":                            "***",
	}
	for in, want := range cases {
		if got := MaskDSN(in); got != want {
			t.Errorf("MaskDSN(%q) = %q, want %q", in, got, want)
		}
	}
}
