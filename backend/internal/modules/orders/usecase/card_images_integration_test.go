package usecase

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	ordersRepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	productsRepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

// memStore serves uploads from memory; other storage calls are not used here.
type memStore struct {
	storage.Driver
	m *storage.Memory
}

func (s memStore) Upload(ctx context.Context, f storage.File, key string) error {
	return s.m.Upload(ctx, f, key)
}
func (s memStore) PublicURL(key string) string { return s.m.PublicURL(key) }

func testPNG(t *testing.T) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

func TestIntegrationOrderCardImagesReachTheCards(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	user := d.CreateUser(t)
	product := d.CreateProduct(t, 100)
	svc := NewService(ordersRepo.NewPostgres(d.Pool, d.Queries), productsRepo.NewPostgres(d.Queries), nil, memStore{m: storage.NewMemory()}, "https://comcode.test")

	orderFront, err := svc.UploadCardImage(ctx, testPNG(t), "front.png", 0)
	if err != nil {
		t.Fatal(err)
	}
	itemBack, err := svc.UploadCardImage(ctx, testPNG(t), "back.png", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UploadCardImage(ctx, bytes.NewReader([]byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")), "x.gif", 0); err == nil {
		t.Fatal("gif accepted")
	}

	// Foreign URLs are refused.
	_, err = svc.CreateOnBehalf(ctx, model.CreateOrderInput{
		FrontImageURL: "https://evil.example/x.png",
		Items:         []model.CreateOrderItemInput{{ProductUUID: product.String(), Quantity: 1, RecipientName: "A"}},
	}, user.ID, user.AccountID)
	if !errors.Is(err, ErrInvalidCardImage) {
		t.Fatalf("foreign url err = %v", err)
	}

	ord, err := svc.CreateOnBehalf(ctx, model.CreateOrderInput{
		FrontImageURL: orderFront,
		Items: []model.CreateOrderItemInput{
			{ProductUUID: product.String(), Quantity: 1, RecipientName: "Own back", BackImageURL: itemBack},
			{ProductUUID: product.String(), Quantity: 1, RecipientName: "Order only"},
		},
	}, user.ID, user.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if ord.FrontImageURL != orderFront || ord.Items[0].BackImageURL != itemBack || ord.Items[1].BackImageURL != "" {
		t.Fatalf("stored images: order=%q items=%q/%q", ord.FrontImageURL, ord.Items[0].BackImageURL, ord.Items[1].BackImageURL)
	}

	provisioned, err := svc.ProvisionOrder(ctx, uuid.MustParse(ord.UUID))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]string{
		"Own back":   {orderFront, itemBack},
		"Order only": {orderFront, ""},
	}
	for _, it := range provisioned.Items {
		var front, back *string
		if err := d.Pool.QueryRow(ctx, `SELECT front_image_url, back_image_url FROM nfc_cards WHERE uuid = $1`, *it.CardUUID).Scan(&front, &back); err != nil {
			t.Fatal(err)
		}
		got := [2]string{deref(front), deref(back)}
		if got != want[it.RecipientName] {
			t.Fatalf("%s card images = %v, want %v", it.RecipientName, got, want[it.RecipientName])
		}
	}

	// Customers cannot set card images.
	mine, err := svc.CreateForAccount(ctx, authctx.Account{ID: user.AccountID, Role: authctx.AccountRoleOwner}, user.ID, model.CreateOrderInput{
		FrontImageURL: orderFront,
		Items:         []model.CreateOrderItemInput{{ProductUUID: product.String(), Quantity: 1, RecipientName: "Me", BackImageURL: itemBack}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mine.FrontImageURL != "" || mine.Items[0].BackImageURL != "" {
		t.Fatal("customer order kept card images")
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
