// Package usecase manages Background Studio designs (TEC-599). App designs belong to the active
// account; presets (account_id NULL) are managed by the platform and visible to everyone.
package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/thumbnail"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound       = errors.New("background not found")
	ErrForbidden      = errors.New("forbidden")
	ErrInvalidRequest = errors.New("invalid request")
	ErrLimitReached   = errors.New("background limit reached")
)

// ThumbnailStore is the part of the object store thumbnails need (storage.Driver implements it).
type ThumbnailStore interface {
	Upload(ctx context.Context, file storage.File, path string) error
	Delete(ctx context.Context, path string) error
	PublicURL(path string) string
}

type Service struct {
	q        *db.Queries
	features entitlement.Features
	store    ThumbnailStore
}

func New(q *db.Queries) *Service { return &Service{q: q} }

// SetFeatures wires the plan check for premium designs (TEC-947). Without it premium designs
// cannot be saved and render as the basic background (fail-closed).
func (s *Service) SetFeatures(f entitlement.Features) { s.features = f }

// SetThumbnails wires the object store for static thumbnails (TEC-599). Without it no thumbnail
// is generated and pages keep the CSS stand-in.
func (s *Service) SetThumbnails(store ThumbnailStore) { s.store = store }

func (s *Service) thumbnailURL(key pgtype.Text) *string {
	if !key.Valid || key.String == "" || s.store == nil {
		return nil
	}
	u := s.store.PublicURL(key.String)
	return &u
}

func (s *Service) toModel(id uuid.UUID, accountID pgtype.Int8, name, kind string, doc []byte, thumb pgtype.Text, created, updated pgtype.Timestamptz) model.Background {
	return model.Background{
		UUID: id, Name: name, Kind: kind, Document: doc, IsPreset: !accountID.Valid,
		Premium: model.IsPremium(doc), ThumbnailURL: s.thumbnailURL(thumb),
		CreatedAt: created.Time, UpdatedAt: updated.Time,
	}
}

// requirePremium refuses a premium document unless the account's plan has design.premium.
func (s *Service) requirePremium(ctx context.Context, accountID int64, doc json.RawMessage) error {
	if !model.IsPremium(doc) {
		return nil
	}
	return entitlement.RequireFeature(ctx, s.features, accountID, entitlement.FeatureDesignPremium)
}

// refreshThumbnail renders the design's static stand-in, stores it and drops the previous one.
// Best effort: a failure leaves the old (or no) thumbnail and the page keeps its CSS stand-in.
// It returns the stored key.
func (s *Service) refreshThumbnail(ctx context.Context, id uuid.UUID, doc []byte, current pgtype.Text) pgtype.Text {
	if s.store == nil {
		return current
	}
	key, err := s.storeThumbnail(ctx, id, doc, current)
	if err != nil {
		slog.Default().Warn("background_thumbnail_failed", "background_uuid", id.String(), "error", err)
		return current
	}
	return key
}

func (s *Service) storeThumbnail(ctx context.Context, id uuid.UUID, doc []byte, current pgtype.Text) (pgtype.Text, error) {
	img, ok, err := thumbnail.Render(doc)
	if err != nil {
		return current, err
	}
	key := pgtype.Text{}
	if ok {
		sum := sha256.Sum256(img)
		key = pgtype.Text{String: storage.BackgroundThumbnailObjectKey(id.String(), hex.EncodeToString(sum[:6]), "jpg"), Valid: true}
		if key != current {
			if err := s.store.Upload(ctx, storage.File{
				Body: bytes.NewReader(img), Size: int64(len(img)), ContentType: thumbnail.ContentType,
				Filename: "thumbnail.jpg",
			}, key.String); err != nil {
				return current, err
			}
		}
	}
	if key == current {
		return current, nil
	}
	n, err := s.q.SetBackgroundThumbnail(ctx, db.SetBackgroundThumbnailParams{ThumbnailKey: key, Uuid: id, Document: doc})
	if err != nil {
		return current, err
	}
	if n == 0 {
		// Edited or deleted meanwhile: that write renders its own thumbnail.
		s.deleteObject(ctx, key)
		return current, nil
	}
	s.deleteObject(ctx, current)
	return key, nil
}

func (s *Service) deleteObject(ctx context.Context, key pgtype.Text) {
	if s.store == nil || !key.Valid || key.String == "" {
		return
	}
	if err := s.store.Delete(ctx, key.String); err != nil {
		slog.Default().Warn("background_thumbnail_delete_failed", "key", key.String, "error", err)
	}
}

// BackfillThumbnails renders thumbnails for designs saved before they existed. It runs at
// startup; the content-hash keys make concurrent runs on several instances harmless.
func (s *Service) BackfillThumbnails(ctx context.Context) {
	if s.store == nil {
		return
	}
	for ctx.Err() == nil {
		rows, err := s.q.ListBackgroundsWithoutThumbnail(ctx, 50)
		if err != nil || len(rows) == 0 {
			return
		}
		done := 0
		for _, r := range rows {
			if key := s.refreshThumbnail(ctx, r.Uuid, r.Document, pgtype.Text{}); key.Valid {
				done++
			}
		}
		if done == 0 {
			return // nothing renders (store down?): try again on the next start
		}
	}
}

func validate(in model.UpsertBackgroundInput) (string, string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 120 {
		return "", "", fmt.Errorf("%w: name is required (max 120 characters)", ErrInvalidRequest)
	}
	kind, err := model.ValidateDocument(in.Document)
	if err != nil {
		return "", "", fmt.Errorf("%w: %s", ErrInvalidRequest, err.Error())
	}
	return name, kind, nil
}

// List returns the active account's designs followed by platform presets.
func (s *Service) List(ctx context.Context, p authctx.Principal) ([]model.Background, error) {
	rows, err := s.q.ListBackgroundsForAccount(ctx, p.Account.ID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Background, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.toModel(r.Uuid, r.AccountID, r.Name, r.Kind, r.Document, r.ThumbnailKey, r.CreatedAt, r.UpdatedAt))
	}
	return out, nil
}

// Create saves a design in the active account (owner/admin).
func (s *Service) Create(ctx context.Context, p authctx.Principal, in model.UpsertBackgroundInput) (*model.Background, error) {
	if !p.Account.CanWrite() {
		return nil, ErrForbidden
	}
	name, kind, err := validate(in)
	if err != nil {
		return nil, err
	}
	if err := s.requirePremium(ctx, p.Account.ID, in.Document); err != nil {
		return nil, err
	}
	n, err := s.q.CountAccountBackgrounds(ctx, p.Account.ID)
	if err != nil {
		return nil, err
	}
	if n >= model.MaxPerAccount {
		return nil, ErrLimitReached
	}
	r, err := s.q.CreateBackground(ctx, db.CreateBackgroundParams{
		AccountID: pgtype.Int8{Int64: p.Account.ID, Valid: true},
		Name:      name, Kind: kind, Document: in.Document,
		CreatedBy: pgtype.Int8{Int64: p.UserInternal, Valid: p.UserInternal != 0},
	})
	if err != nil {
		return nil, err
	}
	r.ThumbnailKey = s.refreshThumbnail(ctx, r.Uuid, r.Document, r.ThumbnailKey)
	m := s.toModel(r.Uuid, r.AccountID, r.Name, r.Kind, r.Document, r.ThumbnailKey, r.CreatedAt, r.UpdatedAt)
	return &m, nil
}

// Update replaces one of the active account's designs. Presets are not editable here.
func (s *Service) Update(ctx context.Context, p authctx.Principal, id uuid.UUID, in model.UpsertBackgroundInput) (*model.Background, error) {
	if !p.Account.CanWrite() {
		return nil, ErrForbidden
	}
	name, kind, err := validate(in)
	if err != nil {
		return nil, err
	}
	if err := s.requirePremium(ctx, p.Account.ID, in.Document); err != nil {
		return nil, err
	}
	r, err := s.q.UpdateAccountBackground(ctx, db.UpdateAccountBackgroundParams{
		Name: name, Kind: kind, Document: in.Document, Uuid: id, AccountID: p.Account.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.ThumbnailKey = s.refreshThumbnail(ctx, r.Uuid, r.Document, r.ThumbnailKey)
	m := s.toModel(r.Uuid, r.AccountID, r.Name, r.Kind, r.Document, r.ThumbnailKey, r.CreatedAt, r.UpdatedAt)
	return &m, nil
}

// Delete removes one of the active account's designs. Profiles that used it fall back to the
// default background.
func (s *Service) Delete(ctx context.Context, p authctx.Principal, id uuid.UUID) error {
	if !p.Account.CanWrite() {
		return ErrForbidden
	}
	thumb, _ := s.q.GetBackgroundThumbnailKey(ctx, id)
	n, err := s.q.DeleteAccountBackground(ctx, db.DeleteAccountBackgroundParams{Uuid: id, AccountID: p.Account.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	s.deleteObject(ctx, thumb)
	return nil
}

// Selectable reports whether a design can be picked by the account (its own or a preset) and
// whether it is premium.
func (s *Service) Selectable(ctx context.Context, accountID int64, id uuid.UUID) (usable, premium bool, err error) {
	r, err := s.q.GetBackgroundVisibleToAccount(ctx, db.GetBackgroundVisibleToAccountParams{Uuid: id, AccountID: accountID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return true, model.IsPremium(r.Document), nil
}

// PublicBackground returns the design to render on a public page of the account, or nil.
func (s *Service) PublicBackground(ctx context.Context, accountID int64, id uuid.UUID) (*model.PublicBackground, error) {
	r, err := s.q.GetPublicBackgroundForAccount(ctx, db.GetPublicBackgroundForAccountParams{Uuid: id, AccountID: accountID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := &model.PublicBackground{Document: r.Document, Premium: model.IsPremium(r.Document)}
	if u := s.thumbnailURL(r.ThumbnailKey); u != nil {
		out.ThumbnailURL = *u
	}
	return out, nil
}

// ---- Platform presets ----

func (s *Service) ListPresets(ctx context.Context) ([]model.Background, error) {
	rows, err := s.q.ListBackgroundPresets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.Background, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.toModel(r.Uuid, r.AccountID, r.Name, r.Kind, r.Document, r.ThumbnailKey, r.CreatedAt, r.UpdatedAt))
	}
	return out, nil
}

func (s *Service) CreatePreset(ctx context.Context, p authctx.Principal, in model.UpsertBackgroundInput) (*model.Background, error) {
	name, kind, err := validate(in)
	if err != nil {
		return nil, err
	}
	r, err := s.q.CreateBackground(ctx, db.CreateBackgroundParams{
		Name: name, Kind: kind, Document: in.Document,
		CreatedBy: pgtype.Int8{Int64: p.UserInternal, Valid: p.UserInternal != 0},
	})
	if err != nil {
		return nil, err
	}
	r.ThumbnailKey = s.refreshThumbnail(ctx, r.Uuid, r.Document, r.ThumbnailKey)
	m := s.toModel(r.Uuid, r.AccountID, r.Name, r.Kind, r.Document, r.ThumbnailKey, r.CreatedAt, r.UpdatedAt)
	return &m, nil
}

func (s *Service) UpdatePreset(ctx context.Context, id uuid.UUID, in model.UpsertBackgroundInput) (*model.Background, error) {
	name, kind, err := validate(in)
	if err != nil {
		return nil, err
	}
	r, err := s.q.UpdateBackgroundPreset(ctx, db.UpdateBackgroundPresetParams{Name: name, Kind: kind, Document: in.Document, Uuid: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.ThumbnailKey = s.refreshThumbnail(ctx, r.Uuid, r.Document, r.ThumbnailKey)
	m := s.toModel(r.Uuid, r.AccountID, r.Name, r.Kind, r.Document, r.ThumbnailKey, r.CreatedAt, r.UpdatedAt)
	return &m, nil
}

func (s *Service) DeletePreset(ctx context.Context, id uuid.UUID) error {
	thumb, _ := s.q.GetBackgroundThumbnailKey(ctx, id)
	n, err := s.q.DeleteBackgroundPreset(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	s.deleteObject(ctx, thumb)
	return nil
}
