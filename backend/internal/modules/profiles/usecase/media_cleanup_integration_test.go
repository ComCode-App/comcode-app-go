package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

// recordingQueue captures enqueued tasks instead of sending them to Redis.
type recordingQueue struct {
	mu    sync.Mutex
	tasks []*asynq.Task
}

func (q *recordingQueue) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, task)
	return &asynq.TaskInfo{}, nil
}

type cleanupEnv struct {
	t     *testing.T
	ctx   context.Context
	d     *pgtest.DB
	mem   *storage.Memory
	store memoryDriver
	svc   *Service
	owner authctx.Principal
}

func newCleanupEnv(t *testing.T) *cleanupEnv {
	d := pgtest.Open(t)
	mem := storage.NewMemory()
	store := memoryDriver{mem: mem}
	svc := NewService(repository.NewPostgres(d.Pool, d.Queries), store)
	svc.SetQuotas(&stubQuotas{})
	u := d.CreateUser(t)
	owner := authctx.Principal{
		UserInternal: u.ID, UserID: u.UUID,
		Account: authctx.Account{ID: u.AccountID, UUID: uuid.New(), Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner},
	}
	return &cleanupEnv{t: t, ctx: authctx.WithPrincipal(context.Background(), owner), d: d, mem: mem, store: store, svc: svc, owner: owner}
}

// createProfile creates a profile and waits for its background vCard cache write, so the
// goroutine cannot race the cleanup under test.
func (e *cleanupEnv) createProfile() uuid.UUID {
	e.t.Helper()
	p, err := e.svc.Create(e.ctx, e.owner, sampleProfileInput())
	if err != nil {
		e.t.Fatal(err)
	}
	vcard := storage.ProfileVCardObjectKey(p.UUID.String(), "tr")
	deadline := time.Now().Add(5 * time.Second)
	for !e.exists(vcard) {
		if time.Now().After(deadline) {
			e.t.Fatalf("vCard cache %s was never written", vcard)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return p.UUID
}

func (e *cleanupEnv) put(key string) {
	e.t.Helper()
	if err := e.mem.Upload(e.ctx, storage.File{Body: strings.NewReader("x"), ContentType: "image/png"}, key); err != nil {
		e.t.Fatal(err)
	}
}

func (e *cleanupEnv) exists(key string) bool {
	ok, _ := e.mem.Exists(e.ctx, key)
	return ok
}

func (e *cleanupEnv) setMedia(id uuid.UUID, avatarURL, coverURL string) {
	e.t.Helper()
	if _, err := e.d.Pool.Exec(e.ctx, `UPDATE profiles SET avatar_url = $2, cover_url = $3 WHERE uuid = $1`, id, avatarURL, coverURL); err != nil {
		e.t.Fatal(err)
	}
}

func (e *cleanupEnv) requireDeleted(id uuid.UUID) {
	e.t.Helper()
	live, err := e.d.Queries.ProfileIsLive(e.ctx, id)
	if err != nil || live {
		e.t.Fatalf("profile %s should be soft-deleted: live=%v err=%v", id, live, err)
	}
}

// TEC-1176: deleting a profile removes its own objects from storage (avatar, cover, vCard cache,
// copied company logo, anything else under profiles/{uuid}/) through the queued job, while
// objects still referenced elsewhere and company media stay. The job is idempotent.
func TestDeleteProfileRemovesOwnedMediaViaQueue(t *testing.T) {
	e := newCleanupEnv(t)
	q := &recordingQueue{}
	cleaner := NewMediaCleaner(e.store, e.d.Queries, nil, nil).WithEnqueuer(q)
	e.svc.SetMediaCleaner(cleaner)

	victim := e.createProfile()
	other := e.createProfile()
	prefix := "profiles/" + victim.String() + "/"

	avatar := storage.ProfileAvatarObjectKey(victim.String(), "png")
	cover := storage.ProfileCoverObjectKey(victim.String(), "jpg")
	oldAvatar := storage.ProfileAvatarObjectKey(victim.String(), "webp") // left behind by an earlier upload
	companyCopy := storage.ProfileCompanyMediaObjectKey(victim.String(), "logo", "abc", "png")
	vcardEN := storage.ProfileVCardObjectKey(victim.String(), "en")
	shared := prefix + "shared.png" // referenced by another live profile
	companyMedia := storage.CompanyMediaObjectKey(uuid.NewString(), uuid.NewString(), "png")
	otherAvatar := storage.ProfileAvatarObjectKey(other.String(), "png")
	for _, k := range []string{avatar, cover, oldAvatar, companyCopy, vcardEN, shared, companyMedia, otherAvatar} {
		e.put(k)
	}
	e.setMedia(victim, e.store.PublicURL(avatar), "/api/v1/storage/stream/"+cover)
	e.setMedia(other, e.store.PublicURL(shared), e.store.PublicURL(companyMedia))

	if err := e.svc.Delete(e.ctx, e.owner, victim); err != nil {
		t.Fatal(err)
	}
	e.requireDeleted(victim)

	if len(q.tasks) != 1 || q.tasks[0].Type() != queue.TaskProfileMediaCleanup {
		t.Fatalf("want one %s task, got %d", queue.TaskProfileMediaCleanup, len(q.tasks))
	}
	payload, err := queue.ParseProfileMediaCleanupPayload(q.tasks[0].Payload())
	if err != nil || payload.ProfileUUID != victim.String() {
		t.Fatalf("payload: %+v %v", payload, err)
	}
	for _, k := range []string{avatar, cover} {
		if !containsKey(payload.Keys, k) {
			t.Fatalf("payload keys %v miss %s (collected before the delete)", payload.Keys, k)
		}
	}
	if !e.exists(avatar) {
		t.Fatal("objects must stay until the job runs")
	}

	if err := cleaner.Process(e.ctx, payload); err != nil {
		t.Fatalf("Process: %v", err)
	}
	gone := []string{avatar, cover, oldAvatar, companyCopy, vcardEN, storage.ProfileVCardObjectKey(victim.String(), "tr")}
	kept := []string{shared, companyMedia, otherAvatar, storage.ProfileVCardObjectKey(other.String(), "tr")}
	assertState := func(stage string) {
		t.Helper()
		for _, k := range gone {
			if e.exists(k) {
				t.Errorf("%s: %s should be deleted", stage, k)
			}
		}
		for _, k := range kept {
			if !e.exists(k) {
				t.Errorf("%s: %s should be kept", stage, k)
			}
		}
	}
	assertState("first run")

	// Idempotent: a retry or a duplicate run succeeds and changes nothing.
	if err := cleaner.Process(e.ctx, payload); err != nil {
		t.Fatalf("second Process: %v", err)
	}
	assertState("second run")

	// Once the last reference is gone, a later run removes the shared object too.
	e.setMedia(other, "", e.store.PublicURL(companyMedia))
	if err := cleaner.Process(e.ctx, payload); err != nil {
		t.Fatal(err)
	}
	if e.exists(shared) {
		t.Fatal("unreferenced object under the deleted profile's prefix should be deleted")
	}
	if !e.exists(companyMedia) {
		t.Fatal("company media is never deleted by profile cleanup")
	}

	// A task for a live profile deletes nothing.
	if err := cleaner.Process(e.ctx, queue.ProfileMediaCleanupPayload{ProfileUUID: other.String(), Keys: []string{otherAvatar}}); err != nil {
		t.Fatal(err)
	}
	if !e.exists(otherAvatar) {
		t.Fatal("live profile's media must not be deleted")
	}
}

// TEC-1176: with the queue off (QUEUE_ENABLED=false) the cleanup runs inline right after the delete.
func TestDeleteProfileRemovesMediaInlineWithoutQueue(t *testing.T) {
	e := newCleanupEnv(t)
	e.svc.SetMediaCleaner(NewMediaCleaner(e.store, e.d.Queries, nil, nil))

	id := e.createProfile()
	avatar := storage.ProfileAvatarObjectKey(id.String(), "png")
	staged := storage.ProfileTempAvatarObjectKey(uuid.NewString(), "jpg") // copy to the permanent key failed
	e.put(avatar)
	e.put(staged)
	e.setMedia(id, e.store.PublicURL(staged), e.store.PublicURL(avatar))

	if err := e.svc.Delete(e.ctx, e.owner, id); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{avatar, staged, storage.ProfileVCardObjectKey(id.String(), "tr")} {
		if e.exists(k) {
			t.Errorf("%s should be deleted inline", k)
		}
	}
}

// failingStore refuses every delete.
type failingStore struct{ memoryDriver }

func (failingStore) Delete(context.Context, string) error { return errors.New("storage down") }

// TEC-1176: a storage failure is logged and never rolls back or fails the profile delete; the
// job reports it so Asynq retries.
func TestDeleteProfileSurvivesStorageFailure(t *testing.T) {
	e := newCleanupEnv(t)
	broken := failingStore{e.store}
	cleaner := NewMediaCleaner(broken, e.d.Queries, nil, nil)
	e.svc.SetMediaCleaner(cleaner)

	id := e.createProfile()
	avatar := storage.ProfileAvatarObjectKey(id.String(), "png")
	e.put(avatar)
	e.setMedia(id, e.store.PublicURL(avatar), "")

	if err := e.svc.Delete(e.ctx, e.owner, id); err != nil {
		t.Fatalf("delete must succeed when storage fails: %v", err)
	}
	e.requireDeleted(id)
	if err := cleaner.Process(e.ctx, queue.ProfileMediaCleanupPayload{ProfileUUID: id.String()}); err == nil {
		t.Fatal("Process should return the delete error so the task is retried")
	}
	if _, err := e.svc.Get(e.ctx, e.owner, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted profile must stay deleted: %v", err)
	}
}

func containsKey(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}
