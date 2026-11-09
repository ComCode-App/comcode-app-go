package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/qr"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/google/uuid"
)

// fakeQRStore scopes routes and profile logos by account, like the Postgres queries.
type fakeQRStore struct {
	routes map[uuid.UUID]int64 // route uuid -> account id
	saved  map[uuid.UUID]qr.Style
	logos  map[uuid.UUID]struct {
		account int64
		url     string
	}
	puts int
}

func newFakeQRStore() *fakeQRStore {
	return &fakeQRStore{routes: map[uuid.UUID]int64{}, saved: map[uuid.UUID]qr.Style{}, logos: map[uuid.UUID]struct {
		account int64
		url     string
	}{}}
}

func (f *fakeQRStore) GetQR(_ context.Context, id uuid.UUID, accountID int64) (*repository.RouteQR, error) {
	if owner, ok := f.routes[id]; !ok || owner != accountID {
		return nil, repository.ErrQRNotFound
	}
	style, ok := f.saved[id]
	if !ok {
		style = qr.DefaultStyle()
	}
	return &repository.RouteQR{Path: "s/preview", Style: style}, nil
}

func (f *fakeQRStore) PutQR(_ context.Context, id uuid.UUID, _ int64, style qr.Style) error {
	f.puts++
	f.saved[id] = style
	return nil
}

func (f *fakeQRStore) GetLogoURL(_ context.Context, id uuid.UUID, accountID int64, _ string) (string, error) {
	l, ok := f.logos[id]
	if !ok || l.account != accountID {
		return "", repository.ErrQRNotFound
	}
	return l.url, nil
}

type previewFixture struct {
	svc     *usecase.Service
	store   *fakeQRStore
	route   uuid.UUID
	account authctx.Account
	logo    uuid.UUID
}

func newPreviewFixture(t *testing.T) previewFixture {
	t.Helper()
	mem := storage.NewMemory()
	store := newFakeQRStore()
	svc := usecase.NewService(newMockRepo(), "https://comcode.test", nil)
	svc.SetQR(store, qrMemoryDriver{mem: mem})
	route, logo := uuid.New(), uuid.New()
	account := authctx.Account{ID: 7, Role: authctx.AccountRoleOwner}
	store.routes[route] = account.ID
	key := storage.ProfileAvatarObjectKey(logo.String(), "png")
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}
	if err := mem.Upload(context.Background(), storage.File{Body: bytes.NewReader(encoded.Bytes())}, key); err != nil {
		t.Fatal(err)
	}
	store.logos[logo] = struct {
		account int64
		url     string
	}{account.ID, mem.PublicURL(key)}
	return previewFixture{svc: svc, store: store, route: route, account: account, logo: logo}
}

func TestPreviewQRMatchesSavedRender(t *testing.T) {
	f := newPreviewFixture(t)
	ctx := context.Background()
	style := qr.DefaultStyle()
	style.Foreground = "#123456"
	style.Module = "dots"
	style.Eye = "circle"
	style.QuietZone = 6
	style.LogoProfileUUID = &f.logo
	style.LogoField = "avatar"
	for _, format := range []string{"png", "svg"} {
		preview, ct, err := f.svc.PreviewQR(ctx, f.route, f.account, style, format, 512)
		if err != nil {
			t.Fatalf("%s preview: %v", format, err)
		}
		if f.store.puts != 0 {
			t.Fatal("preview persisted the style")
		}
		// Saving the same style and downloading must give byte-identical output.
		if _, err := f.svc.PutQR(ctx, f.route, f.account, style); err != nil {
			t.Fatal(err)
		}
		rendered, renderCT, err := f.svc.RenderQR(ctx, f.route, f.account, format, 512)
		if err != nil {
			t.Fatal(err)
		}
		if ct != renderCT || !bytes.Equal(preview, rendered) {
			t.Fatalf("%s preview differs from download", format)
		}
		f.store.puts = 0
		delete(f.store.saved, f.route)
	}
}

func TestPreviewQRRejectsOutOfScopeAndInvalid(t *testing.T) {
	f := newPreviewFixture(t)
	ctx := context.Background()
	ok := qr.DefaultStyle()
	foreign := authctx.Account{ID: 99, Role: authctx.AccountRoleOwner}
	if _, _, err := f.svc.PreviewQR(ctx, f.route, foreign, ok, "png", 512); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("foreign route: %v", err)
	}
	// Previews need no write role (nothing is stored) but stay account-scoped.
	member := authctx.Account{ID: f.account.ID, Role: "member"}
	if _, _, err := f.svc.PreviewQR(ctx, f.route, member, ok, "svg", 512); err != nil {
		t.Fatalf("member preview: %v", err)
	}
	lowContrast := ok
	lowContrast.Foreground = "#eeeeee"
	foreignLogo := ok
	other := uuid.New()
	f.store.logos[other] = struct {
		account int64
		url     string
	}{99, "/api/v1/storage/stream/profiles/" + other.String() + "/avatar.png"}
	foreignLogo.LogoProfileUUID = &other
	foreignLogo.LogoField = "avatar"
	cases := []struct {
		name   string
		style  qr.Style
		format string
		size   int
	}{
		{"pdf not offered", ok, "pdf", 512},
		{"unknown format", ok, "gif", 512},
		{"too small", ok, "png", usecase.PreviewMinSize - 1},
		{"too large", ok, "png", usecase.PreviewMaxSize + 1},
		{"low contrast", lowContrast, "png", 512},
		{"foreign logo", foreignLogo, "png", 512},
	}
	for _, tc := range cases {
		if _, _, err := f.svc.PreviewQR(ctx, f.route, f.account, tc.style, tc.format, tc.size); !errors.Is(err, qr.ErrInvalid) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	svg, _, err := f.svc.PreviewQR(ctx, f.route, f.account, ok, "svg", usecase.PreviewMaxSize)
	if err != nil || !strings.HasPrefix(string(svg), "<svg") {
		t.Fatalf("max size svg: %v", err)
	}
}

