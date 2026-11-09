package pgtest

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) { Main(m) }

// TestMigrationsApplied checks the harness ran every migration up to the newest file.
func TestMigrationsApplied(t *testing.T) {
	d := Open(t)
	dir, err := MigrationsDir()
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	var latest int64
	for _, f := range files {
		n, err := strconv.ParseInt(strings.SplitN(filepath.Base(f), "_", 2)[0], 10, 64)
		if err == nil && n > latest {
			latest = n
		}
	}

	var version int64
	var dirty bool
	if err := d.Pool.QueryRow(context.Background(), "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if dirty || version != latest {
		t.Fatalf("schema_migrations = %d (dirty=%v), want %d", version, dirty, latest)
	}
}

func TestWithDatabase(t *testing.T) {
	got, err := withDatabase("postgres://u:p@127.0.0.1:63432/test?sslmode=disable", "other")
	if err != nil || got != "postgres://u:p@127.0.0.1:63432/other?sslmode=disable" {
		t.Fatalf("withDatabase = %q, %v", got, err)
	}
	if _, err := withDatabase("mysql://x", "y"); err == nil {
		t.Fatal("non-postgres URL must be rejected")
	}
}

func TestOpenSkipsWithoutEnv(t *testing.T) {
	if os.Getenv(EnvVar) != "" {
		t.Skip("env set; skip behaviour not observable")
	}
	ran := false
	t.Run("inner", func(t *testing.T) {
		Open(t)
		ran = true
	})
	if ran {
		t.Fatal("Open must skip when the env var is unset")
	}
}
