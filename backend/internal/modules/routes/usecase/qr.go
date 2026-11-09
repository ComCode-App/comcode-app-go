package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // Existing profile uploads support JPEG as well as PNG.
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/ingestion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/qr"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/google/uuid"
)

// SetQR wires scoped QR persistence and the existing object storage driver.
func (s *Service) SetQR(store repository.QRStore, driver storage.Driver) {
	s.qrStore = store
	s.qrStorage = driver
}

func (s *Service) GetQR(ctx context.Context, id uuid.UUID, account authctx.Account) (*repository.RouteQR, error) {
	result, err := s.qrStore.GetQR(ctx, id, account.ID)
	if errors.Is(err, repository.ErrQRNotFound) {
		return nil, ErrRouteNotFound
	}
	if err != nil {
		return nil, err
	}
	result.Payload = qr.PayloadURL(s.frontendURL, result.Path)
	result.ErrorCorrection = "M"
	if result.Style.LogoProfileUUID != nil {
		result.ErrorCorrection = "H"
	}
	return result, nil
}

func (s *Service) PutQR(ctx context.Context, id uuid.UUID, account authctx.Account, style qr.Style) (*repository.RouteQR, error) {
	result, err := s.GetQR(ctx, id, account)
	if err != nil {
		return nil, err
	}
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	if err := style.Validate(); err != nil {
		return nil, err
	}
	if style.LogoProfileUUID != nil {
		if _, err := s.qrLogo(ctx, account.ID, style); err != nil {
			return nil, err
		}
	}
	if err := s.qrStore.PutQR(ctx, id, account.ID, style); err != nil {
		if errors.Is(err, repository.ErrQRNotFound) {
			return nil, ErrRouteNotFound
		}
		return nil, err
	}
	result.Style = style
	result.ErrorCorrection = "M"
	if style.LogoProfileUUID != nil {
		result.ErrorCorrection = "H"
	}
	return result, nil
}

func (s *Service) RenderQR(ctx context.Context, id uuid.UUID, account authctx.Account, format string, size int) ([]byte, string, error) {
	result, err := s.GetQR(ctx, id, account)
	if err != nil {
		return nil, "", err
	}
	var logo image.Image
	if result.Style.LogoProfileUUID != nil {
		logo, err = s.qrLogo(ctx, account.ID, result.Style)
		if err != nil {
			return nil, "", err
		}
	}
	return qr.Render(result.Payload, format, size, result.Style, logo)
}

// Preview size cap: enough for a crisp on-screen preview, well below the 2048 download maximum.
const (
	PreviewMinSize = 256
	PreviewMaxSize = 1024
)

// PreviewQR renders an unsaved style for an owned route with the same renderer as RenderQR,
// so the editor shows exactly what downloads and the print pack will produce. Nothing is stored.
// Only PNG and SVG are offered; logos go through the same same-account media checks as PutQR.
func (s *Service) PreviewQR(ctx context.Context, id uuid.UUID, account authctx.Account, style qr.Style, format string, size int) ([]byte, string, error) {
	if format != "png" && format != "svg" {
		return nil, "", qr.ErrInvalid
	}
	if size < PreviewMinSize || size > PreviewMaxSize {
		return nil, "", qr.ErrInvalid
	}
	result, err := s.GetQR(ctx, id, account)
	if err != nil {
		return nil, "", err
	}
	if err := style.Validate(); err != nil {
		return nil, "", err
	}
	var logo image.Image
	if style.LogoProfileUUID != nil {
		logo, err = s.qrLogo(ctx, account.ID, style)
		if err != nil {
			return nil, "", err
		}
	}
	return qr.Render(result.Payload, format, size, style, logo)
}

// qrLogo accepts only a current, same-account profile avatar/cover and derives the
// key from an API-owned URL. No arbitrary URLs are fetched. Reads and decode sizes are bounded.
func (s *Service) qrLogo(ctx context.Context, accountID int64, style qr.Style) (image.Image, error) {
	raw, err := s.qrStore.GetLogoURL(ctx, *style.LogoProfileUUID, accountID, style.LogoField)
	if errors.Is(err, repository.ErrQRNotFound) {
		return nil, qr.ErrInvalid
	}
	if err != nil {
		return nil, err
	}
	if s.qrStorage == nil {
		return nil, qr.ErrInvalid
	}
	prefix := "profiles/" + style.LogoProfileUUID.String() + "/" + style.LogoField + "."
	idx := strings.Index(raw, prefix)
	if idx < 0 {
		return nil, qr.ErrInvalid
	}
	key := raw[idx:]
	validURL := raw == s.qrStorage.PublicURL(key) || raw == "/api/v1/storage/stream/"+key || raw == key
	if !validURL || strings.ContainsAny(strings.TrimPrefix(key, prefix), "/\\?#") || strings.Contains(key, "..") {
		return nil, qr.ErrInvalid
	}
	reader, _, err := s.qrStorage.Download(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("read QR logo: %w", err)
	}
	defer func() { _ = reader.Close() }()
	const maxBytes = 2 << 20
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, qr.ErrInvalid
	}
	cfg, kind, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (kind != "png" && kind != "jpeg") {
		return nil, qr.ErrInvalid
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 {
		return nil, qr.ErrInvalid
	}
	logo, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, qr.ErrInvalid
	}
	return logo, nil
}

// RecordQRScan attributes a marked public-profile visit without affecting its response.
func (s *Service) RecordQRScan(ctx context.Context, path string, hc ingestion.HTTPContext) {
	route, err := s.lookupPath(ctx, s.NormalizePath(path))
	if err != nil {
		slog.WarnContext(ctx, "qr_scan_lookup_failed", "error", err)
		return
	}
	if route != nil {
		s.recordQRScan(ctx, route, hc)
		// The public profile page records its own views; the QR scan of a profile's own code is
		// the one case a webhook would otherwise miss.
		s.emitScanned(ctx, route, "qr")
	}
}

func (s *Service) recordQRScan(ctx context.Context, route *model.RouteDetail, hc ingestion.HTTPContext) {
	if s.collector != nil {
		subject := domain.EntityReference{Type: "route", ID: route.UUID.String()}
		if route.CardUUID != nil {
			subject = domain.EntityReference{Type: "card", ID: route.CardUUID.String()}
		}
		eventCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		_, err := s.collector.Ingest(eventCtx, ingestion.IngestRequest{
			Type: "card.qr.scan", SchemaVersion: 1, Subject: subject,
			Properties: map[string]any{"source": "qr", "route_uuid": route.UUID.String(), "path": route.Path},
		}, hc, false, "async")
		if err != nil {
			slog.WarnContext(ctx, "qr_scan_ingest_failed", "route_uuid", route.UUID, "error", err)
		}
	}
}
