package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"regexp"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/qr"
	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

// qrImageSize is the PNG edge in pixels; large enough to print sharply on a card.
const qrImageSize = 1024

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// ManifestQRZip builds the print pack of an order (TEC-581): one QR PNG per provisioned card,
// encoding a QR-marked entry to the /c/{code} identity URL. Activation codes are not included;
// they only travel in the step-up protected manifest export.
func (s *Service) ManifestQRZip(ctx context.Context, orderUUID uuid.UUID) ([]byte, string, error) {
	if _, err := s.repo.GetByUUID(ctx, orderUUID); err != nil {
		return nil, "", ErrNotFound
	}
	cards, err := s.repo.ListManifestCards(ctx, orderUUID)
	if err != nil {
		return nil, "", err
	}
	if len(cards) == 0 {
		return nil, "", ErrNoProvisionedCards
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, c := range cards {
		png, err := qrcode.Encode(qr.CardPayloadURL(s.frontendURL, c.SmartCode), qrcode.Medium, qrImageSize)
		if err != nil {
			return nil, "", fmt.Errorf("qr %s: %w", c.SmartCode, err)
		}
		name := fmt.Sprintf("%03d-%s.png", i+1, unsafeFileChars.ReplaceAllString(c.SmartCode, "_"))
		f, err := zw.Create(name)
		if err != nil {
			return nil, "", err
		}
		if _, err := f.Write(png); err != nil {
			return nil, "", err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	filename := unsafeFileChars.ReplaceAllString(cards[0].OrderNumber, "_") + "-qr.zip"
	return buf.Bytes(), filename, nil
}

// cardPayloadURL is the existing NFC identity URL (unmarked for compatibility).
func (s *Service) cardPayloadURL(smartCode string) string {
	return qr.CardIdentityURL(s.frontendURL, smartCode)
}
