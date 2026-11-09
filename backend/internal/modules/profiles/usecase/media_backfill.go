package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
)

// Deleted-profile media backfill (TEC-1177, KVKK). Profiles soft-deleted before TEC-1176 still
// have their media in object storage. The backfill walks every soft-deleted profile by id, collects
// its keys exactly like Service.Delete does (columns + vCard locales) plus the profiles/{uuid}/
// listing, and applies the same reference check as the cleanup task. The default is a dry run;
// with Apply it enqueues app:profiles:media_cleanup per profile (or runs it inline without a
// queue). Only profiles that still have something to delete are scheduled, so a second run after
// the cleanup finished is a no-op.

const (
	// DefaultBackfillBatchSize is the number of deleted profiles read per query.
	DefaultBackfillBatchSize = 200
	maxBackfillBatchSize     = 5000
)

// BackfillQueries is the database side of the backfill (*db.Queries).
type BackfillQueries interface {
	MediaReferences
	ListDeletedProfilesForMediaBackfill(ctx context.Context, arg db.ListDeletedProfilesForMediaBackfillParams) ([]db.ListDeletedProfilesForMediaBackfillRow, error)
}

// BackfillStore is the object storage surface of the backfill (storage.Driver).
type BackfillStore interface {
	MediaStore
	Exists(ctx context.Context, path string) (bool, error)
	PublicURL(path string) string
}

// BackfillOptions controls one backfill run.
type BackfillOptions struct {
	// Apply schedules (or runs inline) the cleanup; false is a dry run that deletes nothing.
	Apply bool
	// BatchSize is the number of deleted profiles read per query (default 200).
	BatchSize int
	// AfterID resumes after this profiles.id (the last_id a previous run printed).
	AfterID int64
	// Limit stops after this many deleted profiles (0 = all).
	Limit int
	// OnBatch, when set, is called after every batch with the running totals.
	OnBatch func(BackfillSummary)
}

// BackfillSummary holds the counts of a backfill run. It carries no personal data.
type BackfillSummary struct {
	Apply           bool
	DeletedProfiles int   // soft-deleted profiles scanned
	WithMedia       int   // of those, profiles with at least one deletable object
	Candidates      int   // existing objects the cleanup would consider
	Deletable       int   // candidates no live row references
	Skipped         int   // candidates kept by the reference check (shared)
	Enqueued        int   // cleanup tasks enqueued (apply, queue on)
	AlreadyQueued   int   // TaskID conflict: a cleanup task for the profile is already queued
	ProcessedInline int   // profiles cleaned inline (apply, queue off)
	DeletedObjects  int   // objects deleted inline
	Failed          int   // profiles whose planning, enqueue or inline cleanup failed
	LastID          int64 // last profiles.id scanned: pass it as AfterID to resume
}

// MediaBackfill runs the deleted-profile media backfill.
type MediaBackfill struct {
	store   BackfillStore
	queries BackfillQueries
	cleaner *MediaCleaner
	log     *slog.Logger
}

// NewMediaBackfill builds the backfill. A nil queue cleans up inline instead of enqueuing.
func NewMediaBackfill(store BackfillStore, queries BackfillQueries, q MediaEnqueuer, log *slog.Logger) *MediaBackfill {
	cleaner := NewMediaCleaner(store, queries, nil, log)
	if q != nil {
		cleaner.WithEnqueuer(q)
	}
	return &MediaBackfill{store: store, queries: queries, cleaner: cleaner, log: cleaner.log}
}

// Run walks the soft-deleted profiles in id order. It returns the summary and an error only when
// listing the profiles fails; per-profile failures are counted in Failed.
func (b *MediaBackfill) Run(ctx context.Context, opts BackfillOptions) (BackfillSummary, error) {
	batch := opts.BatchSize
	if batch <= 0 {
		batch = DefaultBackfillBatchSize
	}
	if batch > maxBackfillBatchSize {
		batch = maxBackfillBatchSize
	}
	sum := BackfillSummary{Apply: opts.Apply, LastID: opts.AfterID}
	for {
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		size := batch
		if opts.Limit > 0 {
			left := opts.Limit - sum.DeletedProfiles
			if left <= 0 {
				return sum, nil
			}
			size = min(size, left)
		}
		rows, err := b.queries.ListDeletedProfilesForMediaBackfill(ctx, db.ListDeletedProfilesForMediaBackfillParams{
			AfterID:    sum.LastID,
			BatchLimit: int32(size),
		})
		if err != nil {
			return sum, fmt.Errorf("list deleted profiles: %w", err)
		}
		for _, row := range rows {
			b.profile(ctx, opts.Apply, row, &sum)
			sum.LastID = row.ID
		}
		if opts.OnBatch != nil && len(rows) > 0 {
			opts.OnBatch(sum)
		}
		if len(rows) < size {
			return sum, nil
		}
	}
}

// profile plans one deleted profile and, with apply, schedules or runs its cleanup.
func (b *MediaBackfill) profile(ctx context.Context, apply bool, row db.ListDeletedProfilesForMediaBackfillRow, sum *BackfillSummary) {
	sum.DeletedProfiles++
	locales := append([]string{row.PrimaryLocale}, row.Locales...)
	keys := collectProfileMediaKeys(func(raw string) string { return mediaObjectKey(b.store.PublicURL, raw) },
		row.Uuid, row.AvatarUrl, row.CoverUrl, locales)

	deletable, skipped, candidates, err := b.plan(ctx, row.Uuid, keys)
	sum.Candidates += candidates
	sum.Deletable += deletable
	sum.Skipped += skipped
	if err != nil {
		sum.Failed++
		b.log.ErrorContext(ctx, "profile_media_backfill_plan_failed", "profile_uuid", row.Uuid.String(), "error", err)
		return
	}
	if deletable == 0 {
		return
	}
	sum.WithMedia++
	if !apply {
		return
	}

	payload := queue.ProfileMediaCleanupPayload{ProfileUUID: row.Uuid.String(), Keys: keys}
	if b.cleaner.queue != nil {
		task, err := queue.NewProfileMediaCleanupTask(payload)
		if err == nil {
			_, err = b.cleaner.queue.Enqueue(task,
				asynq.Queue(queue.QueueMaintenance),
				asynq.MaxRetry(mediaCleanupMaxRetry),
				asynq.Timeout(mediaCleanupTimeout),
				asynq.TaskID("profile-media-cleanup:"+payload.ProfileUUID))
		}
		switch {
		case err == nil:
			sum.Enqueued++
		case errors.Is(err, asynq.ErrTaskIDConflict):
			sum.AlreadyQueued++
		default:
			sum.Failed++
			b.log.ErrorContext(ctx, "profile_media_backfill_enqueue_failed", "profile_uuid", payload.ProfileUUID, "error", err)
		}
		return
	}

	runCtx, cancel := context.WithTimeout(ctx, mediaCleanupTimeout)
	defer cancel()
	live, err := b.queries.ProfileIsLive(runCtx, row.Uuid)
	if err == nil && live {
		// Restored between the listing and now: never clean up a live profile.
		return
	}
	if err == nil {
		var deleted int
		deleted, _, err = b.cleaner.cleanup(runCtx, row.Uuid, keys)
		sum.DeletedObjects += deleted
	}
	sum.ProcessedInline++
	if err != nil {
		sum.Failed++
		b.log.ErrorContext(ctx, "profile_media_backfill_cleanup_failed", "profile_uuid", payload.ProfileUUID, "error", err)
	}
}

// plan counts, without deleting, the objects the cleanup would consider for a profile: column
// keys that exist plus the prefix listing, split by the reference check.
func (b *MediaBackfill) plan(ctx context.Context, profileUUID uuid.UUID, keys []string) (deletable, skipped, candidates int, err error) {
	listed, errs := b.cleaner.candidateKeys(ctx, profileUUID, nil)
	if len(errs) > 0 {
		return 0, 0, 0, errors.Join(errs...)
	}
	seen := make(map[string]struct{}, len(listed))
	existing := listed
	for _, key := range listed {
		seen[key] = struct{}{}
	}
	prefix := profileMediaPrefix(profileUUID)
	for _, key := range keys {
		if _, ok := seen[key]; ok || !ownedByProfile(key, prefix) {
			continue
		}
		seen[key] = struct{}{}
		ok, err := b.store.Exists(ctx, key)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("object exists: %w", err)
		}
		if ok {
			existing = append(existing, key)
		}
	}
	for _, key := range existing {
		refs, err := b.queries.CountStorageKeyReferences(ctx, key)
		if err != nil {
			return deletable, skipped, len(existing), fmt.Errorf("reference check: %w", err)
		}
		if refs > 0 {
			skipped++
		} else {
			deletable++
		}
	}
	return deletable, skipped, len(existing), nil
}
