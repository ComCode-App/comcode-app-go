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
	return &repository.RouteQR{Path: "s/qr", Style: style}, nil
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

type qrFixture struct {
	svc     *usecase.Service
	store   *fakeQRStore
	route   uuid.UUID
	account authctx.Account
	logo    uuid.UUID
	logoPNG []byte
}

func newQRFixture(t *testing.T) qrFixture {
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
	return qrFixture{svc: svc, store: store, route: route, account: account, logo: logo, logoPNG: encoded.Bytes()}
}

func TestQRLogoReturnsTheStoredMedia(t *testing.T) {
	f := newQRFixture(t)
	data, ct, err := f.svc.QRLogo(context.Background(), f.route, f.account, f.logo, "avatar")
	if err != nil || ct != "image/png" {
		t.Fatalf("logo: %s %v", ct, err)
	}
	if !bytes.Equal(data, f.logoPNG) {
		t.Fatal("logo bytes differ from the stored avatar")
	}
	// Account members without a write role can read it: the editor shows logos to every reader.
	member := authctx.Account{ID: f.account.ID, Role: "member"}
	if _, _, err := f.svc.QRLogo(context.Background(), f.route, member, f.logo, "avatar"); err != nil {
		t.Fatalf("member: %v", err)
	}
}

// The browser draws the saved style with this logo; the server render embeds the same media.
func TestQRLogoIsTheMediaServerRendersUse(t *testing.T) {
	f := newQRFixture(t)
	ctx := context.Background()
	style := qr.DefaultStyle()
	style.LogoProfileUUID = &f.logo
	style.LogoField = "avatar"
	if _, err := f.svc.PutQR(ctx, f.route, f.account, style); err != nil {
		t.Fatal(err)
	}
	svg, _, err := f.svc.RenderQR(ctx, f.route, f.account, "svg", 512)
	if err != nil || !strings.Contains(string(svg), "data:image/png;base64,") {
		t.Fatalf("server render with logo: %v", err)
	}
}

func TestQRLogoRejectsOutOfScopeAndInvalid(t *testing.T) {
	f := newQRFixture(t)
	ctx := context.Background()
	foreign := authctx.Account{ID: 99, Role: authctx.AccountRoleOwner}
	if _, _, err := f.svc.QRLogo(ctx, f.route, foreign, f.logo, "avatar"); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("foreign route: %v", err)
	}
	other := uuid.New()
	f.store.logos[other] = struct {
		account int64
		url     string
	}{99, "/api/v1/storage/stream/profiles/" + other.String() + "/avatar.png"}
	cases := []struct {
		name    string
		profile uuid.UUID
		field   string
	}{
		{"foreign profile", other, "avatar"},
		{"unknown profile", uuid.New(), "avatar"},
		{"nil profile", uuid.Nil, "avatar"},
		{"unknown field", f.logo, "banner"},
		{"empty field", f.logo, ""},
		{"no cover uploaded", f.logo, "cover"},
	}
	for _, tc := range cases {
		if _, _, err := f.svc.QRLogo(ctx, f.route, f.account, tc.profile, tc.field); !errors.Is(err, qr.ErrInvalid) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}
