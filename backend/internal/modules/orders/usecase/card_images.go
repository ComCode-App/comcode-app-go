package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
)

// ErrInvalidCardImage: a card image URL that was not uploaded through UploadCardImage.
var ErrInvalidCardImage = errors.New("card image must be uploaded through the order card image route")

// cardImageTypes are the accepted card design formats (TEC-1320): PNG, JPEG or WebP.
var cardImageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}

// UploadCardImage stores a card design image for an order the platform is about to place and
// returns its API-owned URL (TEC-1320). PNG, JPEG or WebP; the type comes from the bytes.
func (s *Service) UploadCardImage(ctx context.Context, file io.Reader, filename string, size int64) (string, error) {
	if s.storage == nil {
		return "", ErrStorageUnavailable
	}
	sniffed, err := upload.SniffImage(file)
	if err != nil {
		return "", err
	}
	if !cardImageTypes[sniffed.ContentType] {
		return "", fmt.Errorf("%w: card images must be PNG, JPEG or WebP", upload.ErrUnsupportedType)
	}
	key := storage.OrderCardImageObjectKey(uuid.NewString(), sniffed.Ext)
	if err := s.storage.Upload(ctx, storage.File{Body: sniffed.Body, Size: size, ContentType: sniffed.ContentType, Filename: filename}, key); err != nil {
		return "", fmt.Errorf("upload card image: %w", err)
	}
	return s.storage.PublicURL(key), nil
}

// ErrStorageUnavailable: card images need object storage.
var ErrStorageUnavailable = errors.New("object storage is not configured")

// ownCardImage reports whether url is an order card image this storage serves.
func (s *Service) ownCardImage(url string) bool {
	if url == "" {
		return true
	}
	if s.storage == nil {
		return false
	}
	i := strings.Index(url, storage.OrderCardImagePrefix)
	return i >= 0 && s.storage.PublicURL(url[i:]) == url
}

// validateCardImages accepts only images uploaded through UploadCardImage, so an order cannot
// point cards at arbitrary URLs.
func (s *Service) validateCardImages(in model.CreateOrderInput) error {
	urls := []string{in.FrontImageURL, in.BackImageURL}
	for _, it := range in.Items {
		urls = append(urls, it.FrontImageURL, it.BackImageURL)
	}
	for _, u := range urls {
		if !s.ownCardImage(u) {
			return ErrInvalidCardImage
		}
	}
	return nil
}

// stripCardImages drops card images from customer orders: only the platform sets designs.
func stripCardImages(in *model.CreateOrderInput) {
	in.FrontImageURL, in.BackImageURL = "", ""
	for i := range in.Items {
		in.Items[i].FrontImageURL, in.Items[i].BackImageURL = "", ""
	}
}
