package release

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
)

// Upserter is the one query RecordBoot needs (db.Queries implements it).
type Upserter interface {
	UpsertAppRelease(ctx context.Context, arg db.UpsertAppReleaseParams) (db.AppRelease, error)
}

// recordTimeout bounds the boot write so a slow database cannot hold up startup.
const recordTimeout = 5 * time.Second

// Record upserts the app_releases row for info, keyed by (version, git_commit): the first boot
// of a build inserts it, later boots move last_seen_at and boot_count. Idempotent, so every
// replica and every restart can call it.
func Record(ctx context.Context, q Upserter, info Info) (db.AppRelease, error) {
	builtAt := pgtype.Timestamptz{}
	if t, ok := info.BuiltAtTime(); ok {
		builtAt = pgtype.Timestamptz{Time: t, Valid: true}
	}
	row, err := q.UpsertAppRelease(ctx, db.UpsertAppReleaseParams{
		Version:   info.Version,
		GitCommit: info.Commit,
		BuiltAt:   builtAt,
		Notes:     info.Notes,
	})
	if err != nil {
		return db.AppRelease{}, fmt.Errorf("upsert app_releases: %w", err)
	}
	return row, nil
}

// RecordBoot records the running build at server startup. It is fail-soft: an error is logged
// as a warning and boot continues, because provenance must never take the API down.
func RecordBoot(ctx context.Context, q Upserter, log *slog.Logger) {
	info := Current()
	ctx, cancel := context.WithTimeout(ctx, recordTimeout)
	defer cancel()
	row, err := Record(ctx, q, info)
	if err != nil {
		log.Warn("app_release_record_failed", "version", info.Version, "commit", info.ShortCommit(), "error", err)
		return
	}
	log.Info("app_release_recorded",
		"version", row.Version,
		"commit", info.ShortCommit(),
		"boot_count", row.BootCount,
		"first_seen_at", row.FirstSeenAt.Time)
}
