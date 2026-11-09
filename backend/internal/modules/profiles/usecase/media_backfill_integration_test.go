package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
)

func (d memoryDriver) Exists(ctx context.Context, path string) (bool, error) {
	return d.mem.Exists(ctx, path)
}

// backfillFixture: two profiles soft-deleted before TEC-1176 (their media still in storage) and
// one live profile. Other tests share the package database, so the backfill starts after the
// highest profile id that existed before the fixture.
type backfillFixture struct {
	afterID                      int64
	victimA, victimB, live       uuid.UUID
	gone, kept                   []string
	shared, companyMedia, staged string
}

func newBackfillFixture(e *cleanupEnv) backfillFixture {
	e.t.Helper()
	var f backfillFixture
	if err := e.d.Pool.QueryRow(e.ctx, `SELECT COALESCE(MAX(id), 0) FROM profiles`).Scan(&f.afterID); err != nil {
		e.t.Fatal(err)
	}
	f.victimA = e.createProfile()
	f.victimB = e.createProfile()
	f.live = e.createProfile()

	avatarA := storage.ProfileAvatarObjectKey(f.victimA.String(), "png")
	coverA := storage.ProfileCoverObjectKey(f.victimA.String(), "jpg")
	f.shared = "profiles/" + f.victimA.String() + "/shared.png" // the live profile's avatar
	f.staged = storage.ProfileTempAvatarObjectKey(uuid.NewString(), "jpg")
	f.companyMedia = storage.CompanyMediaObjectKey(uuid.NewString(), uuid.NewString(), "png")
	liveCover := storage.ProfileCoverObjectKey(f.live.String(), "png")
	for _, k := range []string{avatarA, coverA, f.shared, f.staged, f.companyMedia, liveCover} {
		e.put(k)
	}
	e.setMedia(f.victimA, e.store.PublicURL(avatarA), "/api/v1/storage/stream/"+coverA)
	e.setMedia(f.victimB, e.store.PublicURL(f.staged), e.store.PublicURL(f.companyMedia))
	e.setMedia(f.live, e.store.PublicURL(f.shared), e.store.PublicURL(liveCover))
	// Soft delete the old way: no cleanup ran.
	if _, err := e.d.Pool.Exec(e.ctx, `UPDATE profiles SET deleted_at = NOW() WHERE uuid = ANY($1)`,
		[]uuid.UUID{f.victimA, f.victimB}); err != nil {
		e.t.Fatal(err)
	}

	f.gone = []string{avatarA, coverA, f.staged,
		storage.ProfileVCardObjectKey(f.victimA.String(), "tr"), storage.ProfileVCardObjectKey(f.victimB.String(), "tr")}
	f.kept = []string{f.shared, f.companyMedia, liveCover, storage.ProfileVCardObjectKey(f.live.String(), "tr")}
	return f
}

func (f backfillFixture) assert(e *cleanupEnv, stage string, cleaned bool) {
	e.t.Helper()
	for _, k := range f.gone {
		if e.exists(k) == cleaned {
			e.t.Errorf("%s: %s exists=%v, want %v", stage, k, !cleaned, !cleaned)
		}
	}
	for _, k := range f.kept {
		if !e.exists(k) {
			e.t.Errorf("%s: %s should be kept", stage, k)
		}
	}
}

// TEC-1177: the dry run counts and deletes nothing; -apply without a queue cleans up inline, keeping
// shared objects, company media and live profiles' media; a second run is a no-op.
func TestMediaBackfillDryRunThenInlineApply(t *testing.T) {
	e := newCleanupEnv(t)
	f := newBackfillFixture(e)
	b := NewMediaBackfill(e.store, e.d.Queries, nil, nil)

	batches := 0
	sum, err := b.Run(e.ctx, BackfillOptions{BatchSize: 1, AfterID: f.afterID, OnBatch: func(BackfillSummary) { batches++ }})
	if err != nil {
		t.Fatal(err)
	}
	// A: avatar, cover, vCard tr, shared (referenced); B: staged avatar, vCard tr. The company
	// media file B pointed at is never a candidate.
	want := BackfillSummary{DeletedProfiles: 2, WithMedia: 2, Candidates: 6, Deletable: 5, Skipped: 1, LastID: sum.LastID}
	if sum != want {
		t.Fatalf("dry run summary = %+v, want %+v", sum, want)
	}
	if sum.LastID <= f.afterID || batches != 2 {
		t.Fatalf("cursor/batches: last_id=%d after=%d batches=%d", sum.LastID, f.afterID, batches)
	}
	f.assert(e, "dry run", false)

	sum, err = b.Run(e.ctx, BackfillOptions{Apply: true, BatchSize: 1, AfterID: f.afterID})
	if err != nil {
		t.Fatal(err)
	}
	if sum.ProcessedInline != 2 || sum.DeletedObjects != 5 || sum.Failed != 0 || sum.Enqueued != 0 {
		t.Fatalf("apply summary = %+v", sum)
	}
	f.assert(e, "apply", true)

	sum, err = b.Run(e.ctx, BackfillOptions{Apply: true, AfterID: f.afterID})
	if err != nil {
		t.Fatal(err)
	}
	want = BackfillSummary{Apply: true, DeletedProfiles: 2, Candidates: 1, Skipped: 1, LastID: sum.LastID}
	if sum != want {
		t.Fatalf("second run summary = %+v, want %+v (no-op)", sum, want)
	}
	f.assert(e, "second run", true)
}

// TEC-1177: with the queue on, -apply enqueues one cleanup task per deleted profile with media;
// the worker's Process removes the objects; a resumed run (-after-id, -limit) and a second run
// behave.
func TestMediaBackfillApplyEnqueuesCleanupTasks(t *testing.T) {
	e := newCleanupEnv(t)
	f := newBackfillFixture(e)
	q := &recordingQueue{}
	b := NewMediaBackfill(e.store, e.d.Queries, q, nil)

	// Resumable: the first run stops after one profile, the next resumes from its last_id.
	first, err := b.Run(e.ctx, BackfillOptions{Apply: true, AfterID: f.afterID, Limit: 1})
	if err != nil || first.DeletedProfiles != 1 || first.Enqueued != 1 {
		t.Fatalf("first run: %+v %v", first, err)
	}
	rest, err := b.Run(e.ctx, BackfillOptions{Apply: true, AfterID: first.LastID})
	if err != nil || rest.DeletedProfiles != 1 || rest.Enqueued != 1 {
		t.Fatalf("resumed run: %+v %v", rest, err)
	}
	f.assert(e, "enqueued, not processed", false)

	if len(q.tasks) != 2 {
		t.Fatalf("want 2 tasks, got %d", len(q.tasks))
	}
	cleaner := NewMediaCleaner(e.store, e.d.Queries, nil, nil)
	seen := map[string]bool{}
	for _, task := range q.tasks {
		if task.Type() != queue.TaskProfileMediaCleanup {
			t.Fatalf("task type %s", task.Type())
		}
		payload, err := queue.ParseProfileMediaCleanupPayload(task.Payload())
		if err != nil {
			t.Fatal(err)
		}
		seen[payload.ProfileUUID] = true
		if err := cleaner.Process(e.ctx, payload); err != nil {
			t.Fatalf("Process: %v", err)
		}
	}
	if !seen[f.victimA.String()] || !seen[f.victimB.String()] || seen[f.live.String()] {
		t.Fatalf("tasks for %v", seen)
	}
	f.assert(e, "processed", true)

	again, err := b.Run(e.ctx, BackfillOptions{Apply: true, AfterID: f.afterID})
	if err != nil || again.Enqueued != 0 || again.WithMedia != 0 || len(q.tasks) != 2 {
		t.Fatalf("second run must enqueue nothing: %+v %v tasks=%d", again, err, len(q.tasks))
	}
}
