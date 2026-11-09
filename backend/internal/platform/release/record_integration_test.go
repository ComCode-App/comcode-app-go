package release_test

// Postgres integration test (TEC-1094). Skipped unless TEST_DATABASE_URL is set; see
// internal/testutil/pgtest.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/release"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func TestRecordIsIdempotentPerVersionAndCommit(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	commit := "c0ffee" + pgtest.Unique()
	info := release.Info{Version: "v0.42", Commit: commit, BuiltAt: "2026-11-09T10:00:00Z", Notes: "- first (abc1234)"}

	first, err := release.Record(ctx, d.Queries, info)
	if err != nil {
		t.Fatalf("first record: %v", err)
	}
	if first.BootCount != 1 || !first.BuiltAt.Valid || first.Notes != info.Notes {
		t.Fatalf("first row = %+v", first)
	}

	time.Sleep(10 * time.Millisecond)
	// A restart of a build without notes (or build time) must not wipe what the first boot stored.
	again, err := release.Record(ctx, d.Queries, release.Info{Version: info.Version, Commit: commit})
	if err != nil {
		t.Fatalf("second record: %v", err)
	}
	if again.ID != first.ID {
		t.Fatalf("second boot inserted a new row (%d != %d)", again.ID, first.ID)
	}
	if again.BootCount != 2 {
		t.Fatalf("boot_count = %d, want 2", again.BootCount)
	}
	if !again.FirstSeenAt.Time.Equal(first.FirstSeenAt.Time) {
		t.Fatalf("first_seen_at moved: %v -> %v", first.FirstSeenAt.Time, again.FirstSeenAt.Time)
	}
	if !again.LastSeenAt.Time.After(first.LastSeenAt.Time) {
		t.Fatalf("last_seen_at did not move: %v -> %v", first.LastSeenAt.Time, again.LastSeenAt.Time)
	}
	if again.Notes != info.Notes || !again.BuiltAt.Valid {
		t.Fatalf("notes/built_at lost on restart: %+v", again)
	}

	// Same version, different commit (a re-cut tag) is a separate row.
	other, err := release.Record(ctx, d.Queries, release.Info{Version: info.Version, Commit: commit + "x"})
	if err != nil {
		t.Fatalf("other commit: %v", err)
	}
	if other.ID == first.ID || other.BootCount != 1 {
		t.Fatalf("other commit row = %+v", other)
	}

	var n int
	if err := d.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM app_releases WHERE git_commit LIKE $1 || '%'`, commit).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}
}

func TestRecordBootRecordsDevBuild(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	release.RecordBoot(ctx, d.Queries, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rows, err := d.Queries.ListAppReleases(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Version == release.DevVersion {
			return
		}
	}
	t.Fatalf("no %q row after RecordBoot: %+v", release.DevVersion, rows)
}

type failingUpserter struct{}

func (failingUpserter) UpsertAppRelease(context.Context, db.UpsertAppReleaseParams) (db.AppRelease, error) {
	return db.AppRelease{}, errors.New("database is down")
}

// RecordBoot must never panic or exit when the database write fails.
func TestRecordBootIsFailSoft(t *testing.T) {
	release.RecordBoot(context.Background(), failingUpserter{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
