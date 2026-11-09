// Package usecase manages the company media library and company branding (TEC-946).
//
// Both belong to the active company account: reads need app.company_branding.read, writes
// app.company_branding.write and an owner/admin account role. The branding feeds the public
// page theme inheritance of employee profiles (profiles.SetAccountThemes, TEC-601) and the
// bulk logo action of employee tools (TEC-573).
package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/model"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Store is the subset of db.Querier the service uses.
type Store interface {
	ListCompanyMedia(context.Context, db.ListCompanyMediaParams) ([]db.ListCompanyMediaRow, error)
	CountCompanyMedia(context.Context, db.CountCompanyMediaParams) (int64, error)
	GetCompanyMedia(context.Context, db.GetCompanyMediaParams) (db.GetCompanyMediaRow, error)
	CreateCompanyMedia(context.Context, db.CreateCompanyMediaParams) (db.CreateCompanyMediaRow, error)
	DeleteCompanyMedia(context.Context, db.DeleteCompanyMediaParams) (int64, error)
	CompanyMediaBrandingUse(context.Context, db.CompanyMediaBrandingUseParams) ([]db.CompanyMediaBrandingUseRow, error)
	CompanyMediaProfileUse(context.Context, db.CompanyMediaProfileUseParams) ([]db.CompanyMediaProfileUseRow, error)
	GetAccountBranding(context.Context, int64) (db.GetAccountBrandingRow, error)
	UpsertAccountBranding(context.Context, db.UpsertAccountBrandingParams) error
}

// Media is the object storage surface of the library (storage.Driver).
type Media interface {
	Upload(ctx context.Context, file storage.File, path string) error
	Delete(ctx context.Context, path string) error
	PublicURL(path string) string
}

type Service struct {
	q     Store
	media Media
	// features gates premium themes (design.premium, TEC-947); nil refuses them.
	features entitlement.Features
}

func New(q Store, media Media) *Service { return &Service{q: q, media: media} }

// SetFeatures wires the design.premium check for the company theme.
func (s *Service) SetFeatures(f entitlement.Features) { s.features = f }

// access checks the active account is a company and the caller holds permission; write also
// needs an owner/admin account role.
func access(p authctx.Principal, permission string, write bool) error {
	if p.Account.ID <= 0 || p.Account.Type != authctx.AccountTypeCompany || !p.HasPermission(permission) {
		return model.ErrForbidden
	}
	if write && !p.Account.CanWrite() {
		return model.ErrForbidden
	}
	return nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", model.ErrInvalid, fmt.Sprintf(format, args...))
}

func (s *Service) url(key string) string {
	if s.media == nil {
		return "/api/v1/storage/stream/" + key
	}
	return s.media.PublicURL(key)
}

func uploader(raw string) *uuid.UUID {
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

// List returns library files of the active account, newest first, each with its usages.
func (s *Service) List(ctx context.Context, p authctx.Principal, f model.Filter) ([]model.Media, int64, error) {
	if err := access(p, rbac.PermAppCompanyBrandingRead, false); err != nil {
		return nil, 0, err
	}
	if f.Kind != "" && !slices.Contains(model.Kinds, f.Kind) {
		return nil, 0, invalid("unknown kind %q", f.Kind)
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	f.Limit = min(f.Limit, 100)
	f.Offset = max(f.Offset, 0)
	rows, err := s.q.ListCompanyMedia(ctx, db.ListCompanyMediaParams{
		AccountID: p.Account.ID, Kind: f.Kind, LimitCount: f.Limit, OffsetCount: f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountCompanyMedia(ctx, db.CountCompanyMediaParams{AccountID: p.Account.ID, Kind: f.Kind})
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.Media, 0, len(rows))
	refs := make([]mediaRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.Media{
			UUID: r.Uuid, Kind: r.Kind, Name: r.Name, ContentType: r.ContentType, SizeBytes: r.SizeBytes,
			URL: s.url(r.ObjectKey), UploadedByUUID: uploader(r.UploadedByUuid), CreatedAt: r.CreatedAt.Time,
		})
		refs = append(refs, mediaRef{id: r.ID, key: r.ObjectKey})
	}
	usage, err := s.usage(ctx, p.Account.ID, refs)
	if err != nil {
		return nil, 0, err
	}
	for i := range out {
		out[i].Usage = usage[refs[i].id]
	}
	return out, total, nil
}

type mediaRef struct {
	id  int64
	key string
}

// usage maps media ids to where they are used: the branding slots and live profiles of the
// account whose avatar or cover points at the file. Every id gets a non-nil slice.
func (s *Service) usage(ctx context.Context, accountID int64, refs []mediaRef) (map[int64][]model.Usage, error) {
	out := make(map[int64][]model.Usage, len(refs))
	if len(refs) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(refs))
	keys := make([]string, 0, len(refs))
	byKey := make(map[string]int64, len(refs))
	for _, r := range refs {
		ids = append(ids, r.id)
		keys = append(keys, r.key)
		byKey[r.key] = r.id
		out[r.id] = []model.Usage{}
	}
	slots, err := s.q.CompanyMediaBrandingUse(ctx, db.CompanyMediaBrandingUseParams{AccountID: accountID, MediaIds: ids})
	if err != nil {
		return nil, err
	}
	for _, b := range slots {
		t := model.UsageBrandingLogo
		if b.Slot == "cover" {
			t = model.UsageBrandingCover
		}
		out[b.MediaID.Int64] = append(out[b.MediaID.Int64], model.Usage{Type: t})
	}
	profiles, err := s.q.CompanyMediaProfileUse(ctx, db.CompanyMediaProfileUseParams{AccountID: accountID, ObjectKeys: keys})
	if err != nil {
		return nil, err
	}
	for _, pr := range profiles {
		id := byKey[pr.ObjectKey]
		pu := pr.ProfileUuid
		if pr.AsAvatar {
			out[id] = append(out[id], model.Usage{Type: model.UsageProfileAvatar, ProfileUUID: &pu, ProfileName: pr.DisplayName})
		}
		if pr.AsCover {
			out[id] = append(out[id], model.Usage{Type: model.UsageProfileCover, ProfileUUID: &pu, ProfileName: pr.DisplayName})
		}
	}
	return out, nil
}

var imageExt = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}

// detect sniffs the content type from the file head; the declared type is never trusted.
// Images (logo, cover, image) must be JPEG, PNG or WebP; documents must be PDF.
func detect(kind string, head []byte, size int64) (contentType, ext string, err error) {
	ct := http.DetectContentType(head)
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = ct[:i]
	}
	if kind == model.KindDocument {
		if ct != "application/pdf" {
			return "", "", invalid("documents must be PDF files")
		}
		if size > model.MaxDocumentBytes {
			return "", "", invalid("documents must be at most 10 MiB")
		}
		return ct, "pdf", nil
	}
	ext, ok := imageExt[ct]
	if !ok {
		return "", "", invalid("images must be JPEG, PNG or WebP")
	}
	if size > model.MaxImageBytes {
		return "", "", invalid("images must be at most 5 MiB")
	}
	return ct, ext, nil
}

func cleanName(name, filename string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.TrimSpace(filename)
	}
	if name == "" {
		name = "file"
	}
	if utf8.RuneCountInString(name) > 255 || !utf8.ValidString(name) {
		return "", invalid("name must be at most 255 characters")
	}
	return name, nil
}

// Upload stores a file in the active account's library (owner/admin).
func (s *Service) Upload(ctx context.Context, p authctx.Principal, in model.UploadInput, body io.Reader) (*model.Media, error) {
	if err := access(p, rbac.PermAppCompanyBrandingWrite, true); err != nil {
		return nil, err
	}
	if s.media == nil {
		return nil, invalid("media storage is not available")
	}
	if !slices.Contains(model.Kinds, in.Kind) {
		return nil, invalid("kind must be one of %s", strings.Join(model.Kinds, ", "))
	}
	if in.Size <= 0 {
		return nil, invalid("file is required")
	}
	name, err := cleanName(in.Name, in.Filename)
	if err != nil {
		return nil, err
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(body, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	head = head[:n]
	contentType, ext, err := detect(in.Kind, head, in.Size)
	if err != nil {
		return nil, err
	}
	count, err := s.q.CountCompanyMedia(ctx, db.CountCompanyMediaParams{AccountID: p.Account.ID})
	if err != nil {
		return nil, err
	}
	if count >= model.MaxMediaPerAccount {
		return nil, model.ErrLimitReached
	}
	id := uuid.New()
	key := storage.CompanyMediaObjectKey(p.Account.UUID.String(), id.String(), ext)
	if err := s.media.Upload(ctx, storage.File{
		Body: io.MultiReader(bytes.NewReader(head), body), Size: in.Size, ContentType: contentType,
		Filename: name,
	}, key); err != nil {
		return nil, fmt.Errorf("upload company media: %w", err)
	}
	row, err := s.q.CreateCompanyMedia(ctx, db.CreateCompanyMediaParams{
		Uuid: id, AccountID: p.Account.ID, Kind: in.Kind, Name: name, ObjectKey: key,
		ContentType: contentType, SizeBytes: in.Size,
		UploadedByUserID: pgtype.Int8{Int64: p.UserInternal, Valid: p.UserInternal > 0},
	})
	if err != nil {
		_ = s.media.Delete(ctx, key)
		return nil, err
	}
	return &model.Media{
		UUID: row.Uuid, Kind: row.Kind, Name: row.Name, ContentType: row.ContentType, SizeBytes: row.SizeBytes,
		URL: s.url(row.ObjectKey), UploadedByUUID: &p.UserID, CreatedAt: row.CreatedAt.Time, Usage: []model.Usage{},
	}, nil
}

// Delete removes a library file (owner/admin). A file still used by the branding or a profile
// is refused with an *model.InUseError listing the usages.
func (s *Service) Delete(ctx context.Context, p authctx.Principal, id uuid.UUID) error {
	if err := access(p, rbac.PermAppCompanyBrandingWrite, true); err != nil {
		return err
	}
	row, err := s.q.GetCompanyMedia(ctx, db.GetCompanyMediaParams{AccountID: p.Account.ID, Uuid: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return err
	}
	usage, err := s.usage(ctx, p.Account.ID, []mediaRef{{id: row.ID, key: row.ObjectKey}})
	if err != nil {
		return err
	}
	if u := usage[row.ID]; len(u) > 0 {
		return &model.InUseError{Usage: u}
	}
	n, err := s.q.DeleteCompanyMedia(ctx, db.DeleteCompanyMediaParams{AccountID: p.Account.ID, ID: row.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrNotFound
	}
	if s.media != nil {
		_ = s.media.Delete(ctx, row.ObjectKey) // the row is gone; a stray object is harmless
	}
	return nil
}

// GetBranding returns the active account's branding (all fields unset when never saved).
func (s *Service) GetBranding(ctx context.Context, p authctx.Principal) (*model.Branding, error) {
	if err := access(p, rbac.PermAppCompanyBrandingRead, false); err != nil {
		return nil, err
	}
	return s.branding(ctx, p.Account.ID)
}

func (s *Service) branding(ctx context.Context, accountID int64) (*model.Branding, error) {
	row, err := s.q.GetAccountBranding(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return &model.Branding{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := &model.Branding{
		Theme: row.Theme, LayoutMode: row.LayoutMode, PrimaryColor: row.PrimaryColor, Locked: row.Locked,
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	if row.LogoUuid.Valid {
		out.Logo = &model.MediaRef{UUID: uuid.UUID(row.LogoUuid.Bytes), URL: s.url(row.LogoKey.String)}
	}
	if row.CoverUuid.Valid {
		out.Cover = &model.MediaRef{UUID: uuid.UUID(row.CoverUuid.Bytes), URL: s.url(row.CoverKey.String)}
	}
	return out, nil
}

var colorRE = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// pickImage resolves a branding logo/cover pick: an image file (not a document) of the account.
func (s *Service) pickImage(ctx context.Context, accountID int64, field string, id *uuid.UUID) (pgtype.Int8, error) {
	if id == nil {
		return pgtype.Int8{}, nil
	}
	row, err := s.q.GetCompanyMedia(ctx, db.GetCompanyMediaParams{AccountID: accountID, Uuid: *id})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.Int8{}, invalid("%s: file %s is not in this company's library", field, *id)
	}
	if err != nil {
		return pgtype.Int8{}, err
	}
	if row.Kind == model.KindDocument {
		return pgtype.Int8{}, invalid("%s must be an image", field)
	}
	return pgtype.Int8{Int64: row.ID, Valid: true}, nil
}

// UpdateBranding replaces the active account's branding (owner/admin). Theme and layout use
// the public page allow-lists; "" means unset.
func (s *Service) UpdateBranding(ctx context.Context, p authctx.Principal, in model.UpdateBrandingInput) (*model.Branding, error) {
	if err := access(p, rbac.PermAppCompanyBrandingWrite, true); err != nil {
		return nil, err
	}
	in.Theme = strings.TrimSpace(in.Theme)
	in.LayoutMode = strings.TrimSpace(in.LayoutMode)
	in.PrimaryColor = strings.ToLower(strings.TrimSpace(in.PrimaryColor))
	if in.Theme != "" && !slices.Contains(pmodel.PublicThemes, in.Theme) {
		return nil, invalid("unknown theme %q", in.Theme)
	}
	if in.LayoutMode != "" && !slices.Contains(pmodel.PublicLayoutModes, in.LayoutMode) {
		return nil, invalid("unknown layout_mode %q", in.LayoutMode)
	}
	if _, premium := pmodel.PremiumThemes[in.Theme]; premium {
		// Only a new pick needs design.premium: a theme kept from a paid period stays savable
		// (public pages render its basic stand-in until an upgrade).
		current, err := s.AccountBranding(ctx, p.Account.ID)
		if err != nil {
			return nil, err
		}
		if current == nil || current.Theme != in.Theme {
			if err := entitlement.RequireFeature(ctx, s.features, p.Account.ID, entitlement.FeatureDesignPremium); err != nil {
				return nil, err
			}
		}
	}
	if in.PrimaryColor != "" && !colorRE.MatchString(in.PrimaryColor) {
		return nil, invalid("primary_color must be a #rrggbb color")
	}
	logo, err := s.pickImage(ctx, p.Account.ID, "logo_media_uuid", in.LogoMediaUUID)
	if err != nil {
		return nil, err
	}
	cover, err := s.pickImage(ctx, p.Account.ID, "cover_media_uuid", in.CoverMediaUUID)
	if err != nil {
		return nil, err
	}
	if err := s.q.UpsertAccountBranding(ctx, db.UpsertAccountBrandingParams{
		AccountID: p.Account.ID, Theme: in.Theme, LayoutMode: in.LayoutMode, PrimaryColor: in.PrimaryColor,
		LogoMediaID: logo, CoverMediaID: cover, Locked: in.Locked,
		UpdatedByUserID: pgtype.Int8{Int64: p.UserInternal, Valid: p.UserInternal > 0},
	}); err != nil {
		return nil, err
	}
	return s.branding(ctx, p.Account.ID)
}

// AccountBranding is the public page source of company theme inheritance
// (profiles.AccountThemeSource). nil when the account has no branding.
func (s *Service) AccountBranding(ctx context.Context, accountID int64) (*pmodel.AccountBranding, error) {
	b, err := s.branding(ctx, accountID)
	if err != nil || b.UpdatedAt == nil {
		return nil, err
	}
	out := &pmodel.AccountBranding{
		Theme: b.Theme, LayoutMode: b.LayoutMode, PrimaryColor: b.PrimaryColor, Locked: b.Locked,
	}
	if b.Logo != nil {
		out.LogoURL = b.Logo.URL
	}
	if b.Cover != nil {
		out.CoverURL = b.Cover.URL
	}
	return out, nil
}

// BrandingMediaURL returns the media URL of the branding logo ("logo") or cover ("cover") of an
// account, or "" when unset. The bulk logo action references it instead of copying files.
func (s *Service) BrandingMediaURL(ctx context.Context, accountID int64, slot string) (string, error) {
	b, err := s.branding(ctx, accountID)
	if err != nil {
		return "", err
	}
	ref := b.Logo
	if slot == "cover" {
		ref = b.Cover
	}
	if ref == nil {
		return "", nil
	}
	return ref.URL, nil
}
