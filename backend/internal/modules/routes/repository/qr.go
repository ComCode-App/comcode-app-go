package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/qr"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrQRNotFound = errors.New("QR route or logo not found")

type RouteQR struct {
	Path            string   `json:"-"`
	Style           qr.Style `json:"style"`
	Payload         string   `json:"payload"`
	ErrorCorrection string   `json:"error_correction"`
}

type QRStore interface {
	GetQR(context.Context, uuid.UUID, int64) (*RouteQR, error)
	PutQR(context.Context, uuid.UUID, int64, qr.Style) error
	GetLogoURL(context.Context, uuid.UUID, int64, string) (string, error)
}

func (r *PostgresRepository) GetQR(ctx context.Context, id uuid.UUID, accountID int64) (*RouteQR, error) {
	row, err := r.q.GetRouteQR(ctx, db.GetRouteQRParams{Uuid: id, AccountID: pgtype.Int8{Int64: accountID, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQRNotFound
	}
	if err != nil {
		return nil, err
	}
	style := qr.DefaultStyle()
	if err := json.Unmarshal(row.QrStyle, &style); err != nil {
		return nil, err
	}
	return &RouteQR{Path: row.Path, Style: style}, nil
}

func (r *PostgresRepository) PutQR(ctx context.Context, id uuid.UUID, accountID int64, style qr.Style) error {
	raw, err := json.Marshal(style)
	if err != nil {
		return err
	}
	count, err := r.q.PutRouteQR(ctx, db.PutRouteQRParams{Uuid: id, AccountID: pgtype.Int8{Int64: accountID, Valid: true}, QrStyle: raw})
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrQRNotFound
	}
	return nil
}

func (r *PostgresRepository) GetLogoURL(ctx context.Context, id uuid.UUID, accountID int64, field string) (string, error) {
	row, err := r.q.GetQRLogo(ctx, db.GetQRLogoParams{Uuid: id, AccountID: accountID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrQRNotFound
	}
	if err != nil {
		return "", err
	}
	if field == "cover" {
		return row.CoverUrl, nil
	}
	return row.AvatarUrl, nil
}
