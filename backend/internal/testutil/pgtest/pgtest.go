// Package pgtest is the Postgres integration test harness (TEC-536).
//
// Integration tests run only when TEST_DATABASE_URL points at a Postgres server the tests may
// create databases on (for example postgres://test:test@127.0.0.1:63432/test?sslmode=disable).
// Without it every test that calls Open is skipped, so `go test ./...` stays hermetic.
//
// Each test binary (one per Go package) gets its own fresh database with every migration in
// backend/migrations applied through golang-migrate. Tests of one package share that database,
// so they must create their own rows (see the fixture helpers) instead of assuming empty tables.
// Wire Main into the package's TestMain to drop the database when the package is done:
//
//	func TestMain(m *testing.M) { pgtest.Main(m) }
package pgtest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // pgx5:// driver
	_ "github.com/golang-migrate/migrate/v4/source/file"     // file:// source
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
)

// EnvVar names the admin connection string. Unset means integration tests are skipped.
const EnvVar = "TEST_DATABASE_URL"

// DB is the migrated per-package database.
type DB struct {
	Pool    *pgxpool.Pool
	Queries *db.Queries
	Name    string
}

var (
	once    sync.Once
	shared  *DB
	initErr error
	adminDS string
)

// Open returns the package database, creating and migrating it on first use. It skips the test
// when TEST_DATABASE_URL is unset and fails it when the database cannot be prepared.
func Open(t testing.TB) *DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(EnvVar))
	if dsn == "" {
		t.Skipf("%s not set; skipping Postgres integration test", EnvVar)
	}
	once.Do(func() {
		adminDS = dsn
		shared, initErr = setup(context.Background(), dsn)
	})
	if initErr != nil {
		t.Fatalf("pgtest: %v", initErr)
	}
	return shared
}

// Main runs the package tests and drops the package database afterwards.
func Main(m *testing.M) {
	code := m.Run()
	if shared != nil {
		shared.Pool.Close()
		if err := dropDatabase(context.Background(), adminDS, shared.Name); err != nil {
			fmt.Fprintf(os.Stderr, "pgtest: drop %s: %v\n", shared.Name, err)
		}
	}
	os.Exit(code)
}

func setup(ctx context.Context, adminDSN string) (*DB, error) {
	name, err := databaseName()
	if err != nil {
		return nil, err
	}
	if err := createFromTemplate(ctx, adminDSN, name); err != nil {
		return nil, err
	}
	dsn, err := withDatabase(adminDSN, name)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &DB{Pool: pool, Queries: db.New(pool), Name: name}, nil
}

// createFromTemplate creates name as a copy of a migrated template database. Migrating takes
// tens of seconds on Docker for Mac, so the template is built once per migration set (its name
// carries a hash of backend/migrations) and every package clones it. An advisory lock keeps
// parallel test binaries from building it twice. Templates stay on the server; the CI container
// is thrown away after the job.
func createFromTemplate(ctx context.Context, adminDSN, name string) error {
	tmpl, err := templateName()
	if err != nil {
		return err
	}
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return fmt.Errorf("connect admin: %w", err)
	}
	defer func() { _ = admin.Close(ctx) }()

	const lockKey = 536_536_536
	if _, err := admin.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	defer func() { _, _ = admin.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockKey) }()

	var exists bool
	if err := admin.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", tmpl).Scan(&exists); err != nil {
		return fmt.Errorf("check template: %w", err)
	}
	if !exists {
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{tmpl}.Sanitize()); err != nil {
			return fmt.Errorf("create template %s: %w", tmpl, err)
		}
		tmplDSN, err := withDatabase(adminDSN, tmpl)
		if err != nil {
			return err
		}
		if err := migrateUp(tmplDSN); err != nil {
			_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{tmpl}.Sanitize()+" WITH (FORCE)")
			return err
		}
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE "+pgx.Identifier{tmpl}.Sanitize()); err != nil {
		return fmt.Errorf("create database %s: %w", name, err)
	}
	return nil
}

// templateName hashes the migration files so a changed migration set gets a fresh template.
func templateName() (string, error) {
	dir, err := MigrationsDir()
	if err != nil {
		return "", err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		h.Write([]byte(filepath.Base(f)))
		h.Write(b)
	}
	return "pgtest_tmpl_" + hex.EncodeToString(h.Sum(nil))[:16], nil
}

// migrateUp applies every migration with golang-migrate, the same tool `make migrate-up` uses.
func migrateUp(dsn string) error {
	dir, err := MigrationsDir()
	if err != nil {
		return err
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	u.Scheme = "pgx5"
	m, err := migrate.New("file://"+filepath.ToSlash(dir), u.String())
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// MigrationsDir is the absolute path of backend/migrations, resolved from this source file so
// tests work from any package directory.
func MigrationsDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("cannot locate pgtest source file")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("migrations dir: %w", err)
	}
	return dir, nil
}

func dropDatabase(ctx context.Context, adminDSN, name string) error {
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return err
	}
	defer func() { _ = admin.Close(ctx) }()
	_, err = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	return err
}

func withDatabase(dsn, name string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", EnvVar, err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", fmt.Errorf("%s must be a postgres:// URL", EnvVar)
	}
	u.Path = "/" + name
	return u.String(), nil
}

func databaseName() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	pkg := "pkg"
	if wd, err := os.Getwd(); err == nil {
		pkg = sanitize(filepath.Base(filepath.Dir(wd)) + "_" + filepath.Base(wd))
	}
	name := fmt.Sprintf("test_%s_%s", pkg, hex.EncodeToString(b))
	if len(name) > 63 {
		name = name[len(name)-63:]
	}
	return name, nil
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
