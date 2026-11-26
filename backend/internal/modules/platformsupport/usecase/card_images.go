package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
)

var (
	// ErrInvalidCardSide: the side is neither front nor back.
	ErrInvalidCardSide = errors.New("side must be front or back")
	// ErrStorageUnavailable: card images need object storage.
	ErrStorageUnavailable = errors.New("object storage is not configured")
)

// cardImageTypes are the accepted design image formats (TEC-1320): PNG, JPEG or WebP.
var cardImageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}

// ImageStore is the slice of object storage card images use; storage.Driver satisfies it.
type ImageStore interface {
	Upload(ctx context.Context, file storage.File, objectPath string) error
	Delete(ctx context.Context, objectPath string) error
	PublicURL(objectPath string) string
}

// SetStorage wires object storage for card design images.
func (s *Service) SetStorage(d ImageStore) { s.storage = d }

func validSide(side string) bool { return side == "front" || side == "back" }

// UploadCardImage stores a design image for one face of a card. Any aspect ratio is accepted:
// previews crop it with object-fit: cover. The type comes from the bytes, never the client.
func (s *Service) UploadCardImage(ctx context.Context, actor Actor, id uuid.UUID, side string, file io.Reader, filename string, size int64) (*model.Card, error) {
	if !validSide(side) {
		return nil, ErrInvalidCardSide
	}
	if s.storage == nil {
		return nil, ErrStorageUnavailable
	}
	card, err := s.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}
	sniffed, err := upload.SniffImage(file)
	if err != nil {
		return nil, err
	}
	if !cardImageTypes[sniffed.ContentType] {
		return nil, fmt.Errorf("%w: card images must be PNG, JPEG or WebP", upload.ErrUnsupportedType)
	}
	key := storage.CardImageObjectKey(card.UUID.String(), side, strconv.FormatInt(time.Now().UnixMilli(), 36), sniffed.Ext)
	if err := s.storage.Upload(ctx, storage.File{Body: sniffed.Body, Size: size, ContentType: sniffed.ContentType, Filename: filename}, key); err != nil {
		return nil, fmt.Errorf("upload card image: %w", err)
	}
	url := s.storage.PublicURL(key)
	if err := s.repo.SetCardImage(ctx, card.ID, side, url); err != nil {
		_ = s.storage.Delete(context.WithoutCancel(ctx), key)
		return nil, err
	}
	s.deleteReplacedImage(ctx, card, side)
	s.record(ctx, actor, "card.image_updated", "cards", &card.UUID, map[string]any{
		"smart_code": card.SmartCode, "side": side,
	})
	return s.GetCard(ctx, id)
}

// RemoveCardImage clears one face's design image; the preview falls back to the default artwork.
func (s *Service) RemoveCardImage(ctx context.Context, actor Actor, id uuid.UUID, side string) (*model.Card, error) {
	if !validSide(side) {
		return nil, ErrInvalidCardSide
	}
	card, err := s.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}
	if current(card, side) == "" {
		return card, nil
	}
	if err := s.repo.SetCardImage(ctx, card.ID, side, ""); err != nil {
		return nil, err
	}
	s.deleteReplacedImage(ctx, card, side)
	s.record(ctx, actor, "card.image_removed", "cards", &card.UUID, map[string]any{
		"smart_code": card.SmartCode, "side": side,
	})
	return s.GetCard(ctx, id)
}

func current(card *model.Card, side string) string {
	if side == "front" {
		return card.FrontImageURL
	}
	return card.BackImageURL
}

// deleteReplacedImage removes the previous object of a face when it is one of this card's keys.
func (s *Service) deleteReplacedImage(ctx context.Context, card *model.Card, side string) {
	old := current(card, side)
	if s.storage == nil || old == "" {
		return
	}
	prefix := "cards/" + card.UUID.String() + "/" + side + "-"
	i := strings.Index(old, prefix)
	if i < 0 {
		return
	}
	// Only an object this storage serves at exactly that URL is ours to delete.
	if key := old[i:]; s.storage.PublicURL(key) == old {
		_ = s.storage.Delete(context.WithoutCancel(ctx), key)
	}
}
