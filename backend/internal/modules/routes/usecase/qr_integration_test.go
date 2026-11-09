package usecase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/png"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	profileshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/handler"
	profilesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/ingestion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/registry"
	analyticsstorage "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/qr"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/google/uuid"
	"image"
	"io"
)

func TestIntegrationQRAccountIsolationAndStablePayload(t *testing.T) {
	d, repo, svc := newRouteService(t)
	svc.SetQR(repo, nil)
	ctx := context.Background()
	owner, other := d.CreateUser(t), d.CreateUser(t)
	route := createSmartLink(t, repo, owner, "s/qr-"+uuid.NewString(), "https://example.com/a")
	account := authctx.Account{ID: owner.AccountID, Role: "owner"}
	foreign := authctx.Account{ID: other.AccountID, Role: "owner"}
	member := authctx.Account{ID: owner.AccountID, Role: "member"}
	initial, err := svc.GetQR(ctx, route.UUID, account)
	if err != nil {
		t.Fatal(err)
	}
	if initial.Style != qr.DefaultStyle() || initial.ErrorCorrection != "M" {
		t.Fatalf("defaults: %+v", initial)
	}
	style := qr.DefaultStyle()
	style.Module = "rounded"
	style.Eye = "circle"
	if _, err := svc.PutQR(ctx, route.UUID, member, style); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("member write: %v", err)
	}
	if _, err := svc.GetQR(ctx, route.UUID, foreign); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("foreign read: %v", err)
	}
	if _, err := svc.PutQR(ctx, route.UUID, foreign, style); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("foreign write: %v", err)
	}
	if _, _, err := svc.RenderQR(ctx, route.UUID, foreign, "png", 512); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("foreign render: %v", err)
	}
	saved, err := svc.PutQR(ctx, route.UUID, account, style)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Payload != initial.Payload {
		t.Fatal("appearance changed identity")
	}
	data, _, err := svc.RenderQR(ctx, route.UUID, account, "png", 512)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	url := "https://example.com/b"
	if _, err := svc.Update(ctx, route.UUID, account, model.UpdateRouteInput{DirectURL: &url}); err != nil {
		t.Fatal(err)
	}
	after, _, err := svc.RenderQR(ctx, route.UUID, account, "png", 512)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, after) {
		t.Fatal("target update changed QR")
	}
	// Missing and foreign logo references are rejected even with a valid style.
	style.LogoProfileUUID = &other.UUID
	style.LogoField = "avatar"
	if _, err := svc.PutQR(ctx, route.UUID, account, style); !errors.Is(err, qr.ErrInvalid) {
		t.Fatalf("foreign logo: %v", err)
	}
}

type qrEventQueue struct {
	events []domain.Event
	fail   bool
	store  ingestion.DirectEventStore
}

func (q *qrEventQueue) PublishEvent(_ context.Context, event domain.Event) error {
	if event.Type != "card.qr.scan" {
		return nil
	}
	if q.fail {
		return errors.New("queue unavailable")
	}
	if q.store != nil {
		if err := q.store.StoreEvent(context.Background(), event); err != nil {
			return err
		}
	}
	q.events = append(q.events, event)
	return nil
}

func TestIntegrationQRScanResolution(t *testing.T) {
	d, repo, _ := newRouteService(t)
	queue := &qrEventQueue{store: analyticsstorage.NewPostgresStore(d.Pool, d.Queries)}
	collector := ingestion.NewCollector(
		domain.NewValidator(registry.NewCachedStore(d.Queries, 0)), nil, nil, nil, queue, nil, nil,
	)
	svc := usecase.NewService(repo, "https://comcode.test", collector)
	owner := d.CreateUser(t)
	route := createSmartLink(t, repo, owner, "s/qr-"+uuid.NewString(), "https://example.com/private-target")
	h := handler.New(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/r/{path...}", h.Resolve)
	mux.HandleFunc("GET /v1/c/{code}", h.ResolveCard)
	mux.HandleFunc("GET /v1/s/{slug}", h.ResolveSmartLink)
	request := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/v1/"+path, nil)
		r.Header.Set("X-Request-ID", "qr-request")
		mux.ServeHTTP(w, r)
		return w
	}
	resolution := func(path string) model.RouteResolution {
		t.Helper()
		w := request(path)
		if w.Code != http.StatusOK {
			t.Fatalf("resolve: %d %s", w.Code, w.Body.String())
		}
		var body struct {
			Data model.RouteResolution `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Data
	}
	ordinary := resolution(route.Path)
	resolution(route.Path + "?src=nfc")
	if len(queue.events) != 0 {
		t.Fatal("unmarked visit counted as QR")
	}
	marked := resolution(route.Path + "?src=qr")
	if !reflect.DeepEqual(ordinary, marked) {
		t.Fatal("marker changed resolution")
	}
	if len(queue.events) != 1 {
		t.Fatalf("events: %d", len(queue.events))
	}
	event := queue.events[0]
	if event.Type != "card.qr.scan" || event.Subject.Type != "route" || event.Subject.ID != route.UUID.String() || event.RequestID != "qr-request" {
		t.Fatalf("event: %+v", event)
	}
	if event.Properties["source"] != "qr" {
		t.Fatal(event.Properties)
	}
	var count int
	if err := d.Pool.QueryRow(context.Background(), `SELECT count(*) FROM events WHERE subject_id = $1 AND event_type = 'card.qr.scan' AND request_id = 'qr-request'`, route.UUID.String()).Scan(&count); err != nil || count != 1 {
		t.Fatalf("persisted scan: %d %v", count, err)
	}
	queue.fail = true
	if got := resolution(route.Path + "?src=qr"); !reflect.DeepEqual(ordinary, got) {
		t.Fatal("queue outage changed visit")
	}
	if w := request("r/missing-" + uuid.NewString() + "?src=qr"); w.Code != 404 {
		t.Fatal(w.Code)
	}
	queue.fail = false
	card := d.CreateCard(t, owner)
	cr, err := svc.CreateReservedCardRoute(context.Background(), card.ID, card.SmartCode, &owner.ID, &owner.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"reserved", "active", "locked", "retired"} {
		if _, err := d.Pool.Exec(context.Background(), `UPDATE routes SET status=$1 WHERE id=$2`, state, cr.ID); err != nil {
			t.Fatal(err)
		}
		before := resolution(cr.Path)
		after := resolution(cr.Path + "?src=qr")
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("marker changed %s card", state)
		}
		last := queue.events[len(queue.events)-1]
		if last.Subject.Type != "card" || last.Subject.ID != card.UUID.String() {
			t.Fatalf("card subject: %+v", last.Subject)
		}
	}
	if _, err := d.Pool.Exec(context.Background(), `UPDATE nfc_cards SET passcode='secret' WHERE id=$1`, card.ID); err != nil {
		t.Fatal(err)
	}
	beforePasscode := resolution(cr.Path)
	afterPasscode := resolution(cr.Path + "?src=qr")
	if !beforePasscode.RequiresPasscode || !reflect.DeepEqual(beforePasscode, afterPasscode) {
		t.Fatal("marker changed passcode requirement")
	}

	// The universal resolver covers canonical and legacy identities with the same marker.
	for _, rt := range []string{model.RouteTypeCanonicalProfile, model.RouteTypeLegacyRoute} {
		r, err := repo.CreateRoute(context.Background(), model.CreateReservedRouteInput{
			Path: "qr-" + uuid.NewString(), RouteType: rt, Status: model.RouteStatusActive,
			TargetType: model.TargetTypeDirectURL, DirectURL: "https://example.com", AccountID: &owner.AccountID,
		})
		if err != nil {
			t.Fatal(err)
		}
		before := resolution("r/" + r.Path)
		after := resolution("r/" + r.Path + "?src=qr")
		if !reflect.DeepEqual(before, after) {
			t.Fatal("marker changed universal resolution")
		}
		if queue.events[len(queue.events)-1].Subject.ID != r.UUID.String() {
			t.Fatal("wrong route subject")
		}
	}
}

func TestIntegrationQRScanPublicProfile(t *testing.T) {
	d, repo, _ := newRouteService(t)
	queue := &qrEventQueue{store: analyticsstorage.NewPostgresStore(d.Pool, d.Queries)}
	collector := ingestion.NewCollector(
		domain.NewValidator(registry.NewCachedStore(d.Queries, 0)), nil, nil, nil, queue, nil, nil,
	)
	svc := usecase.NewService(repo, "https://comcode.test", collector)
	owner := d.CreateUser(t)
	var profileID int64
	if err := d.Pool.QueryRow(context.Background(), `INSERT INTO profiles(user_id,account_id) VALUES($1,$2) RETURNING id`, owner.ID, owner.AccountID).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	route, err := repo.CreateRoute(context.Background(), model.CreateReservedRouteInput{
		Path: "qr-" + uuid.NewString()[:8], RouteType: model.RouteTypeCanonicalProfile,
		Status: model.RouteStatusActive, TargetType: model.TargetTypeProfile,
		ProfileID: &profileID, AccountID: &owner.AccountID,
	})
	if err != nil {
		t.Fatal(err)
	}
	pr := profilesrepo.NewPostgres(d.Pool, d.Queries)
	ps := profilesusecase.NewService(pr, nil)
	ps.SetPublicPages(pr)
	h := profileshandler.New(ps)
	h.SetQRTracker(svc)
	h.SetLegacyResolver(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/public/p/{username}", h.PublicProfile)
	request := func(mark string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/v1/public/p/"+route.Path+mark, nil)
		r.Header.Set("X-Request-ID", "qr-profile-request")
		mux.ServeHTTP(w, r)
		return w
	}
	for _, state := range []string{"active", "locked", "retired"} {
		if _, err := d.Pool.Exec(context.Background(), `UPDATE routes SET status=$1 WHERE id=$2`, state, route.ID); err != nil {
			t.Fatal(err)
		}
		beforeCount := len(queue.events)
		ordinary := request("")
		if len(queue.events) != beforeCount {
			t.Fatal("unmarked profile recorded QR")
		}
		marked := request("?src=qr")
		if ordinary.Code != marked.Code || ordinary.Body.String() != marked.Body.String() {
			t.Fatal("marker changed visitor response")
		}
		if state == "active" && marked.Code != 200 {
			t.Fatal(marked.Body.String())
		}
		if len(queue.events) != beforeCount+1 {
			t.Fatal("missing visitor scan")
		}
		if last := queue.events[len(queue.events)-1]; last.Subject.ID != route.UUID.String() || last.RequestID != "qr-profile-request" {
			t.Fatal(last)
		}
		queue.fail = true
		if failed := request("?src=qr"); failed.Code != ordinary.Code || failed.Body.String() != ordinary.Body.String() {
			t.Fatal("analytics outage changed visitor response")
		}
		queue.fail = false
	}
}

func TestIntegrationQRScanLegacyPublicProfile(t *testing.T) {
	d, repo, _ := newRouteService(t)
	queue := &qrEventQueue{store: analyticsstorage.NewPostgresStore(d.Pool, d.Queries)}
	collector := ingestion.NewCollector(
		domain.NewValidator(registry.NewCachedStore(d.Queries, 0)), nil, nil, nil, queue, nil, nil,
	)
	svc := usecase.NewService(repo, "https://comcode.test", collector)
	owner := d.CreateUser(t)
	h := profileshandler.New(nil)
	h.SetQRTracker(svc)
	h.SetLegacyResolver(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/public/p/{username}", h.PublicProfile)
	for _, name := range []string{"bare_hash", "collision_alias"} {
		t.Run(name, func(t *testing.T) {
			hash := "qr-legacy-" + uuid.NewString()[:8]
			path := hash
			if name == "collision_alias" {
				// An existing canonical identity must lose to the printed legacy alias.
				if _, err := repo.CreateRoute(t.Context(), model.CreateReservedRouteInput{
					Path: hash, RouteType: model.RouteTypeCanonicalProfile, Status: model.RouteStatusActive,
					TargetType: model.TargetTypeDirectURL, DirectURL: "https://example.com/profile",
					AccountID: &owner.AccountID,
				}); err != nil {
					t.Fatal(err)
				}
				path = "r/legacy/" + hash
			}
			route, err := repo.CreateRoute(t.Context(), model.CreateReservedRouteInput{
				Path: path, RouteType: model.RouteTypeLegacyRoute, Status: model.RouteStatusActive,
				TargetType: model.TargetTypeDirectURL, DirectURL: "https://example.com/legacy",
				AccountID: &owner.AccountID,
			})
			if err != nil {
				t.Fatal(err)
			}
			before := len(queue.events)
			for _, marker := range []string{"", "?src=nfc", "?src=qr"} {
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/v1/public/p/"+hash+marker, nil)
				r.Header.Set("X-Request-ID", "qr-legacy-request")
				mux.ServeHTTP(w, r)
				if w.Code != http.StatusTemporaryRedirect || w.Header().Get("Location") != route.DirectURL {
					t.Fatalf("legacy visit %q: %d %v %s", marker, w.Code, w.Header(), w.Body.String())
				}
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("legacy redirect must not be cached")
				}
				want := before
				if marker == "?src=qr" {
					want++
				}
				if len(queue.events) != want {
					t.Fatalf("legacy visit %q: got %d scans, want %d", marker, len(queue.events), want)
				}
			}
			event := queue.events[before]
			if event.Subject.Type != "route" || event.Subject.ID != route.UUID.String() ||
				event.RequestID != "qr-legacy-request" || event.Properties["source"] != "qr" {
				t.Fatalf("legacy scan: %+v", event)
			}
			var count int
			err = d.Pool.QueryRow(t.Context(),
				`SELECT count(*) FROM events WHERE subject_id = $1 AND event_type = 'card.qr.scan'`,
				route.UUID.String(),
			).Scan(&count)
			if err != nil || count != 1 {
				t.Fatalf("persisted legacy scans: %d %v", count, err)
			}
		})
	}
}

func TestIntegrationQRLogoOwnershipAndMedia(t *testing.T) {
	d, repo, svc := newRouteService(t)
	store := storage.NewMemory()
	svc.SetQR(repo, qrMemoryDriver{mem: store})
	ctx := context.Background()
	owner, other := d.CreateUser(t), d.CreateUser(t)
	route := createSmartLink(t, repo, owner, "s/qr-"+uuid.NewString(), "https://example.com")
	account := authctx.Account{ID: owner.AccountID, Role: "owner"}
	logoID := uuid.New()
	key := storage.ProfileAvatarObjectKey(logoID.String(), "png")
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}
	if err := store.Upload(ctx, storage.File{Body: bytes.NewReader(encoded.Bytes())}, key); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `INSERT INTO profiles(uuid,user_id,account_id,avatar_url) VALUES($1,$2,$3,$4)`, logoID, owner.ID, owner.AccountID, store.PublicURL(key)); err != nil {
		t.Fatal(err)
	}
	style := qr.DefaultStyle()
	style.LogoProfileUUID = &logoID
	style.LogoField = "avatar"
	saved, err := svc.PutQR(ctx, route.UUID, account, style)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ErrorCorrection != "H" {
		t.Fatal("logo did not raise error correction")
	}
	if _, _, err := svc.RenderQR(ctx, route.UUID, account, "png", 512); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE profiles SET account_id=$1 WHERE uuid=$2`, other.AccountID, logoID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RenderQR(ctx, route.UUID, account, "png", 512); !errors.Is(err, qr.ErrInvalid) {
		t.Fatalf("moved logo render: %v", err)
	}
	if _, err := svc.PutQR(ctx, route.UUID, account, style); !errors.Is(err, qr.ErrInvalid) {
		t.Fatalf("foreign existing logo: %v", err)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE profiles SET account_id=$1,avatar_url=$2 WHERE uuid=$3`, owner.AccountID, "https://example.com/"+key, logoID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PutQR(ctx, route.UUID, account, style); !errors.Is(err, qr.ErrInvalid) {
		t.Fatalf("foreign URL: %v", err)
	}
	if _, err := svc.PutQR(ctx, route.UUID, account, qr.DefaultStyle()); err != nil {
		t.Fatal(err)
	}
	// Styles are allowed on immutable card identities without changing their target or path.
	card := d.CreateCard(t, owner)
	cr, err := svc.CreateReservedCardRoute(ctx, card.ID, card.SmartCode, &owner.ID, &owner.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PutQR(ctx, cr.UUID, account, qr.DefaultStyle()); err != nil {
		t.Fatal(err)
	}
}

// Only these driver methods are used by the QR service.
type qrMemoryDriver struct {
	storage.Driver
	mem *storage.Memory
}

func (m qrMemoryDriver) PublicURL(key string) string { return m.mem.PublicURL(key) }
func (m qrMemoryDriver) Download(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	return m.mem.Download(ctx, key)
}
