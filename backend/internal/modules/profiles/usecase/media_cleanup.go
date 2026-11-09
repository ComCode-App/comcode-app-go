package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
)

// Deleted-profile media cleanup (TEC-1176, KVKK). A soft-deleted profile used to leave its avatar,
// cover, vCard cache and copied company logos in object storage, where the bucket serves anonymous
// GETs. After the delete commits, the profile service schedules this cleanup: an Asynq task on the
// maintenance queue (idempotent, retried), or an inline best-effort run when the queue is off.
//
// What is deleted: every object under profiles/{uuid}/ plus a staged tmp/avatars/ object the row
// still pointed at. Company media library files (accounts/{uuid}/media/) are company-owned and
// never touched. Each key is deleted only when no live row references it any more
// (CountStorageKeyReferences): another profile's avatar/cover, a company media file or a
// background thumbnail keeps it.

const (
	mediaCleanupMaxRetry = 10
	mediaCleanupTimeout  = 2 * time.Minute
	// mediaCleanupDelay lets in-flight vCard pre-cache goroutines of the last edit finish first,
	// so a late cache write does not survive the cleanup.
	mediaCleanupDelay = 30 * time.Second
)

// MediaReferences is the database side of the cleanup (*db.Queries).
type MediaReferences interface {
	ProfileIsLive(ctx context.Context, profileUUID uuid.UUID) (bool, error)
	CountStorageKeyReferences(ctx context.Context, objectKey string) (int64, error)
}

// MediaStore is the object storage surface of the cleanup (storage.Driver).
type MediaStore interface {
	Delete(ctx context.Context, path string) error
	List(ctx context.Context, input storage.ListObjectsInput) (storage.ListObjectsResult, error)
}

// MediaEnqueuer schedules the cleanup task (queue.Client).
type MediaEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// MediaCleaner deletes the storage objects of deleted profiles.
type MediaCleaner struct {
	store MediaStore
	refs  MediaReferences
	queue MediaEnqueuer
	log   *slog.Logger
}

// NewMediaCleaner builds the cleaner. A nil queue runs the cleanup inline (QUEUE_ENABLED=false).
func NewMediaCleaner(store MediaStore, refs MediaReferences, q *queue.Client, log *slog.Logger) *MediaCleaner {
	c := &MediaCleaner{store: store, refs: refs, log: log}
	if q != nil {
		c.queue = q
	}
	return c.withDefaults()
}

// WithEnqueuer replaces the queue (tests).
func (c *MediaCleaner) WithEnqueuer(q MediaEnqueuer) *MediaCleaner {
	c.queue = q
	return c
}

func (c *MediaCleaner) withDefaults() *MediaCleaner {
	if c.log == nil {
		c.log = slog.Default()
	}
	c.log = c.log.With("component", "profile_media_cleanup")
	return c
}

// SetMediaCleaner wires the deleted-profile media cleanup (TEC-1176). Without it Delete only drops
// the cached vCards, as before.
func (s *Service) SetMediaCleaner(c *MediaCleaner) { s.mediaCleaner = c }

// Schedule runs after the profile delete has committed. It never fails the delete: an enqueue
// error falls back to an inline best-effort run, and every failure is logged.
func (c *MediaCleaner) Schedule(ctx context.Context, profileUUID uuid.UUID, keys []string) {
	if c == nil || profileUUID == uuid.Nil {
		return
	}
	payload := queue.ProfileMediaCleanupPayload{ProfileUUID: profileUUID.String(), Keys: keys}
	if c.queue != nil {
		task, err := queue.NewProfileMediaCleanupTask(payload)
		if err == nil {
			_, err = c.queue.Enqueue(queue.WithRequestID(ctx, task),
				asynq.Queue(queue.QueueMaintenance),
				asynq.MaxRetry(mediaCleanupMaxRetry),
				asynq.Timeout(mediaCleanupTimeout),
				asynq.ProcessIn(mediaCleanupDelay),
				asynq.TaskID("profile-media-cleanup:"+payload.ProfileUUID))
		}
		if err == nil || errors.Is(err, asynq.ErrTaskIDConflict) {
			return
		}
		c.log.ErrorContext(ctx, "profile_media_cleanup_enqueue_failed",
			"profile_uuid", payload.ProfileUUID, "error", err)
	}
	// Queue off (or enqueue failed): best effort now, detached from the request's cancellation.
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mediaCleanupTimeout)
	defer cancel()
	if err := c.Process(runCtx, payload); err != nil {
		c.log.ErrorContext(ctx, "profile_media_cleanup_failed",
			"profile_uuid", payload.ProfileUUID, "inline", true, "error", err)
	}
}

// Process is the worker entry point. It is idempotent: objects already gone are skipped, so a
// retry or a second run deletes nothing new. It returns an error when any lookup or delete failed,
// and Asynq retries the whole task.
func (c *MediaCleaner) Process(ctx context.Context, payload queue.ProfileMediaCleanupPayload) error {
	profileUUID, err := uuid.Parse(payload.ProfileUUID)
	if err != nil || profileUUID == uuid.Nil {
		return fmt.Errorf("profile media cleanup: invalid profile uuid: %w", asynq.SkipRetry)
	}
	live, err := c.refs.ProfileIsLive(ctx, profileUUID)
	if err != nil {
		return fmt.Errorf("profile media cleanup: profile state: %w", err)
	}
	if live {
		// Never clean up a profile that exists: the task was stale or forged.
		c.log.WarnContext(ctx, "profile_media_cleanup_profile_live", "profile_uuid", payload.ProfileUUID)
		return nil
	}

	deleted, kept, err := c.cleanup(ctx, profileUUID, payload.Keys)
	c.log.InfoContext(ctx, "profile_media_cleanup_done",
		"profile_uuid", payload.ProfileUUID, "deleted", deleted, "kept_shared", kept, "failed", countErrors(err))
	return err
}

// cleanup deletes the unreferenced objects of one deleted profile and returns how many were
// deleted and how many were kept because a live row still references them.
func (c *MediaCleaner) cleanup(ctx context.Context, profileUUID uuid.UUID, keys []string) (deleted, kept int, err error) {
	candidates, errs := c.candidateKeys(ctx, profileUUID, keys)
	for _, key := range candidates {
		refs, err := c.refs.CountStorageKeyReferences(ctx, key)
		if err != nil {
			errs = append(errs, fmt.Errorf("reference check: %w", err))
			continue
		}
		if refs > 0 {
			kept++
			continue
		}
		if err := c.store.Delete(ctx, key); err != nil {
			errs = append(errs, fmt.Errorf("delete object: %w", err))
			continue
		}
		deleted++
	}
	return deleted, kept, errors.Join(errs...)
}

// candidateKeys returns, sorted, the keys the cleanup considers for a profile: the given column
// keys it may own plus everything listed under profiles/{uuid}/. A listing error is returned
// alongside the keys collected so far.
func (c *MediaCleaner) candidateKeys(ctx context.Context, profileUUID uuid.UUID, keys []string) ([]string, []error) {
	prefix := profileMediaPrefix(profileUUID)
	candidates := map[string]struct{}{}
	for _, key := range keys {
		if ownedByProfile(key, prefix) {
			candidates[key] = struct{}{}
		}
	}
	var errs []error
	listed, err := c.listPrefix(ctx, prefix)
	if err != nil {
		errs = append(errs, err)
	}
	for _, key := range listed {
		if ownedByProfile(key, prefix) {
			candidates[key] = struct{}{}
		}
	}
	out := make([]string, 0, len(candidates))
	for key := range candidates {
		out = append(out, key)
	}
	sort.Strings(out)
	return out, errs
}

// countErrors counts the errors behind a (possibly joined) error.
func countErrors(err error) int {
	if err == nil {
		return 0
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return len(joined.Unwrap())
	}
	return 1
}

// listPrefix returns every object key under prefix (all pages).
func (c *MediaCleaner) listPrefix(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	token := ""
	for page := 0; page < 100; page++ {
		res, err := c.store.List(ctx, storage.ListObjectsInput{Prefix: prefix, ContinuationToken: token, MaxKeys: 1000})
		if err != nil {
			return keys, fmt.Errorf("list objects: %w", err)
		}
		for _, obj := range res.Objects {
			keys = append(keys, obj.Key)
		}
		if !res.IsTruncated || res.NextContinuationToken == "" {
			return keys, nil
		}
		token = res.NextContinuationToken
	}
	return keys, fmt.Errorf("list objects: too many pages under %s", prefix)
}

// ownedByProfile accepts only keys a profile can own: its own prefix, or a staged vCard avatar
// (tmp/avatars/) the row still pointed at. Company media and anything else is never deleted here.
func ownedByProfile(key, prefix string) bool {
	if key == "" || strings.Contains(key, "..") || strings.Contains(key, "//") || strings.ContainsAny(key, "?#\\") {
		return false
	}
	return strings.HasPrefix(key, prefix) || strings.HasPrefix(key, tempAvatarPrefix)
}

// profileMediaKeys collects the object keys a profile row points at, before the row is deleted:
// avatar and cover (any extension, a staged temp avatar, a copied company logo) and the cached
// vCards of every locale. The prefix listing in Process catches objects no column names.
func (s *Service) profileMediaKeys(profileUUID uuid.UUID, avatarURL, coverURL string, locales []string) []string {
	return collectProfileMediaKeys(s.mediaKey, profileUUID, avatarURL, coverURL, locales)
}

// collectProfileMediaKeys is profileMediaKeys with the URL-to-key mapping passed in, so the
// deleted-profile backfill (TEC-1177) collects keys exactly like Delete does.
func collectProfileMediaKeys(mediaKey func(string) string, profileUUID uuid.UUID, avatarURL, coverURL string, locales []string) []string {
	prefix := profileMediaPrefix(profileUUID)
	seen := map[string]struct{}{}
	var keys []string
	add := func(key string) {
		if !ownedByProfile(key, prefix) {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	add(mediaKey(avatarURL))
	add(mediaKey(coverURL))
	for _, loc := range locales {
		add(storage.ProfileVCardObjectKey(profileUUID.String(), loc))
	}
	return keys
}
