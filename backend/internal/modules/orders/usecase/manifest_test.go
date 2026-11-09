package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
)

type manifestRepo struct {
	repository.Repository
	order *model.OrderDetail
	cards []model.ManifestCard
}

func (r manifestRepo) GetByUUID(context.Context, uuid.UUID) (*model.OrderDetail, error) {
	if r.order == nil {
		return nil, errors.New("no rows")
	}
	return r.order, nil
}

func (r manifestRepo) ListManifestCards(context.Context, uuid.UUID) ([]model.ManifestCard, error) {
	return r.cards, nil
}

func TestManifestQRZip(t *testing.T) {
	repo := manifestRepo{
		order: &model.OrderDetail{OrderNumber: "ORD-1"},
		cards: []model.ManifestCard{
			{OrderNumber: "ORD-1", SmartCode: "abc123"},
			{OrderNumber: "ORD-1", SmartCode: "x/y"},
		},
	}
	svc := NewService(repo, nil, nil, nil, "https://comcode.test/")
	data, filename, err := svc.ManifestQRZip(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if filename != "ORD-1-qr.zip" {
		t.Fatalf("filename = %q", filename)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 2 || zr.File[0].Name != "001-abc123.png" || zr.File[1].Name != "002-x_y.png" {
		t.Fatalf("unexpected entries: %v", zr.File)
	}
	if got := svc.cardPayloadURL("x/y"); got != "https://comcode.test/c/x%2Fy" {
		t.Fatalf("payload = %q", got)
	}
}

func TestManifestQRZipErrors(t *testing.T) {
	svc := NewService(manifestRepo{}, nil, nil, nil, "https://comcode.test")
	if _, _, err := svc.ManifestQRZip(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing order: %v", err)
	}
	svc = NewService(manifestRepo{order: &model.OrderDetail{}}, nil, nil, nil, "https://comcode.test")
	if _, _, err := svc.ManifestQRZip(context.Background(), uuid.New()); !errors.Is(err, ErrNoProvisionedCards) {
		t.Fatalf("no cards: %v", err)
	}
}
