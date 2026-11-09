package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
)

func TestSystemService_GetInfo(t *testing.T) {
	cfg := config.Config{
		App: config.AppConfig{
			Name: "comcode-test",
			Env:  "test",
		},
		Log: config.LogConfig{
			Level: "info",
		},
	}

	svc := NewSystemService(cfg, nil, nil, nil, nil)
	info := svc.GetInfo(context.Background())

	if info.Environment.AppName != "comcode-test" {
		t.Errorf("expected app_name 'comcode-test', got %s", info.Environment.AppName)
	}
	if info.Runtime.GoVersion == "" {
		t.Errorf("expected GoVersion to be set")
	}
	if info.Runtime.NumCPU <= 0 {
		t.Errorf("expected NumCPU > 0")
	}
	if info.Services.Database.Status != "disabled" {
		t.Errorf("expected Database.Status 'disabled' without db, got %s", info.Services.Database.Status)
	}
}

// pingStore is a storage.Driver whose only implemented method is Ping.
type pingStore struct {
	storage.Driver
	err error
}

func (p pingStore) Ping(context.Context) error { return p.err }

func TestSystemService_StorageReportsConfiguredDriver(t *testing.T) {
	for _, tc := range []struct {
		configured string
		pingErr    error
		want       string
		status     string
	}{
		{configured: "s3", want: "s3", status: "healthy"},
		{configured: " MinIO ", want: "minio", status: "healthy"},
		{configured: "s3", pingErr: errors.New("unreachable"), want: "s3", status: "down"},
		{configured: "", want: "unknown", status: "healthy"},
	} {
		cfg := config.Config{Storage: config.StorageConfig{Driver: tc.configured}}
		info := NewSystemService(cfg, nil, nil, pingStore{err: tc.pingErr}, nil).GetInfo(context.Background())
		if got := info.Services.Storage.Driver; got != tc.want {
			t.Errorf("STORAGE_DRIVER=%q: driver = %q, want %q", tc.configured, got, tc.want)
		}
		if got := info.Services.Storage.Status; got != tc.status {
			t.Errorf("STORAGE_DRIVER=%q: status = %q, want %q", tc.configured, got, tc.status)
		}
	}
}

func TestSystemService_StorageNotInitialized(t *testing.T) {
	cfg := config.Config{Storage: config.StorageConfig{Driver: "s3"}}
	info := NewSystemService(cfg, nil, nil, nil, nil).GetInfo(context.Background())
	if info.Services.Storage.Driver != "none" || info.Services.Storage.Status != "disabled" {
		t.Errorf("nil storage: got driver %q status %q, want none/disabled", info.Services.Storage.Driver, info.Services.Storage.Status)
	}
}
