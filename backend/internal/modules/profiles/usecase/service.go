package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/completion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/vcard"
	"github.com/google/uuid"
)

var (
	ErrNotFound       = errors.New("profile not found")
	ErrForbidden      = errors.New("forbidden")
	ErrInvalidRequest = errors.New("invalid request")
)

type Service struct {
	repo        repository.Repository
	storage     storage.Driver
	vcardGen    *vcard.Generator
	vcardParser *vcard.Parser
	quotas      entitlement.Quotas
	pages       repository.PublicPageStore
	backgrounds BackgroundCatalog
	// features gates premium themes and backgrounds (design.premium, TEC-947).
	features   entitlement.Features
	routeCache *routecache.Cache
	activity   *activity.Recorder

	// linkPresentations applies smart link presentation to public pages (TEC-556); nil = off.
	linkPresentations LinkPresentationSource

	// accountThemes resolves company theme inheritance (TEC-601); nil = no inheritance.
	accountThemes AccountThemeSource

	// verifications is the verified badge (TEC-549); nil = every profile reads "none".
	verifications Verifications

	// search keeps profiles in the account-scoped Cmd+K index (TEC-951); nil = not indexed.
	search searchengine.Enqueuer

	// mediaCleaner deletes a deleted profile's storage objects (TEC-1176); nil = vCards only.
	mediaCleaner *MediaCleaner
}

func NewService(repo repository.Repository, store storage.Driver) *Service {
	return &Service{
		repo:        repo,
		storage:     store,
		vcardGen:    vcard.NewGenerator(),
		vcardParser: vcard.NewParser(),
	}
}

// SetQuotas wires plan quota checks. Without it, quota-guarded creates are denied.
func (s *Service) SetQuotas(q entitlement.Quotas) { s.quotas = q }

func (s *Service) List(ctx context.Context, p authctx.Principal, limit, offset int32, q, status string) ([]model.ProfileSummary, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return s.repo.ListByAccountID(ctx, p.Account.ID, limit, offset, q, status)
}

// get loads a profile of the caller's active account; profiles of other accounts are not found.
func (s *Service) get(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) (*model.ProfileDetail, error) {
	detail, err := s.repo.GetByUUID(ctx, p.Account.ID, profileUUID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return detail, nil
}

// getWritable is get plus the account-role write check (owner/admin).
func (s *Service) getWritable(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) (*model.ProfileDetail, error) {
	detail, err := s.get(ctx, p, profileUUID)
	if err != nil {
		return nil, err
	}
	if !p.Account.CanWrite() {
		return nil, ErrForbidden
	}
	return detail, nil
}

func (s *Service) Get(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) (*model.ProfileDetail, error) {
	detail, err := s.get(ctx, p, profileUUID)
	if err != nil {
		return nil, err
	}
	s.decorate(ctx, p.Account.ID, detail)
	if s.pages == nil {
		return detail, nil
	}
	if profileID, _, err := s.pages.ProfileRef(ctx, p.Account.ID, profileUUID); err == nil {
		if route, err := s.pages.LiveRoute(ctx, profileID); err == nil && route != nil {
			detail.Username = route.Path
		}
	}
	return detail, nil
}

func (s *Service) Create(ctx context.Context, p authctx.Principal, in model.UpsertProfileInput) (*model.ProfileDetail, error) {
	if !p.Account.CanWrite() {
		return nil, ErrForbidden
	}
	if err := entitlement.Guard(ctx, s.quotas, p.Account.ID, entitlement.FeatureProfilesMax); err != nil {
		return nil, err
	}
	if err := s.validateMediaURL("avatar_url", in.AvatarURL, p.Account.UUID, uuid.Nil); err != nil {
		return nil, err
	}
	if err := s.validateMediaURL("cover_url", in.CoverURL, p.Account.UUID, uuid.Nil); err != nil {
		return nil, err
	}
	if in.PrimaryLocale == "" {
		in.PrimaryLocale = "tr"
	}
	if len(in.Translations) == 0 {
		return nil, fmt.Errorf("%w: at least one language translation is required", ErrInvalidRequest)
	}

	// Validate at least primary translation has a name
	hasPrimaryName := false
	for _, tr := range in.Translations {
		if tr.Locale == in.PrimaryLocale || tr.IsDefault {
			if strings.TrimSpace(tr.FirstName) != "" || strings.TrimSpace(tr.LastName) != "" {
				hasPrimaryName = true
				break
			}
		}
	}
	if !hasPrimaryName && len(in.Translations) > 0 {
		if strings.TrimSpace(in.Translations[0].FirstName) != "" || strings.TrimSpace(in.Translations[0].LastName) != "" {
			hasPrimaryName = true
		}
	}
	if !hasPrimaryName {
		return nil, fmt.Errorf("%w: first name or last name is required in profile translation", ErrInvalidRequest)
	}

	compRate := calculateCompletionRate(in)
	detail, err := s.repo.Create(ctx, p.Account.ID, p.UserInternal, p.UserID, in, compRate)
	if err != nil {
		return nil, err
	}

	// If avatar was staged in tmp/, promote to permanent profile avatar in S3
	if s.storage != nil && in.AvatarURL != "" {
		if tmpKey := s.mediaKey(in.AvatarURL); strings.HasPrefix(tmpKey, tempAvatarPrefix) {
			ext := filepath.Ext(tmpKey)
			if ext == "" {
				ext = ".jpg"
			}
			permanentKey := storage.ProfileAvatarObjectKey(detail.UUID.String(), ext)
			if copyErr := s.storage.Copy(ctx, tmpKey, permanentKey); copyErr == nil {
				_ = s.storage.Delete(ctx, tmpKey)
				permanentURL := s.storage.PublicURL(permanentKey)
				if withAvatar, err := s.repo.UpdateAvatar(ctx, p.Account.ID, detail.UUID, permanentURL); err == nil {
					detail = withAvatar
				}
			}
		}
	}

	s.decorate(ctx, p.Account.ID, detail)
	// Invalidate & pre-cache vCard for all supported locales in storage
	go s.preCacheAllVCards(context.Background(), detail)

	// Every profile gets a public username; failures are retried by GetPublicPage.
	if s.pages != nil {
		if err := s.ensureUsername(ctx, p.Account.ID, detail); err == nil {
			if page, err := s.GetPublicPage(ctx, p, detail.UUID); err == nil {
				detail.Username = page.Username
			}
		}
	}
	s.indexProfile(ctx, detail.UUID)

	return detail, nil
}

func (s *Service) Update(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID, in model.UpsertProfileInput) (*model.ProfileDetail, error) {
	detail, err := s.getWritable(ctx, p, profileUUID)
	if err != nil {
		return nil, err
	}

	if err := s.validateMediaURL("avatar_url", in.AvatarURL, p.Account.UUID, profileUUID); err != nil {
		return nil, err
	}
	if err := s.validateMediaURL("cover_url", in.CoverURL, p.Account.UUID, profileUUID); err != nil {
		return nil, err
	}
	if in.PrimaryLocale == "" {
		in.PrimaryLocale = detail.PrimaryLocale
	}
	if len(in.Translations) == 0 {
		return nil, fmt.Errorf("%w: at least one language translation is required", ErrInvalidRequest)
	}

	// If avatar was staged in tmp/, promote to permanent profile avatar in S3
	if s.storage != nil && in.AvatarURL != "" {
		if tmpKey := s.mediaKey(in.AvatarURL); strings.HasPrefix(tmpKey, tempAvatarPrefix) {
			ext := filepath.Ext(tmpKey)
			if ext == "" {
				ext = ".jpg"
			}
			permanentKey := storage.ProfileAvatarObjectKey(profileUUID.String(), ext)
			if copyErr := s.storage.Copy(ctx, tmpKey, permanentKey); copyErr == nil {
				_ = s.storage.Delete(ctx, tmpKey)
				in.AvatarURL = s.storage.PublicURL(permanentKey)
			}
		}
	}

	// A verified profile that changes its display name or organization loses the badge and goes
	// back to review (TEC-549). This runs before the write so a failure leaves nothing changed;
	// if the write then fails the badge stays revoked, which is the safe side for a trust mark.
	if s.verifications != nil {
		if reasons := identityChanges(detail, in); len(reasons) > 0 {
			if err := s.verifications.RevokeForIdentityChange(ctx, p.Account.ID, profileUUID, p.UserInternal, reasons); err != nil {
				return nil, fmt.Errorf("revoke verification: %w", err)
			}
		}
	}

	compRate := calculateCompletionRate(in)
	updated, err := s.repo.Update(ctx, p.Account.ID, profileUUID, p.UserID, in, compRate)
	if err != nil {
		return nil, err
	}
	s.routeCache.InvalidateProfile(ctx, profileUUID)

	s.decorate(ctx, p.Account.ID, updated)
	// Invalidate & pre-cache vCard for all supported locales in storage
	go s.preCacheAllVCards(context.Background(), updated)
	s.indexProfile(ctx, updated.UUID)

	return updated, nil
}

func (s *Service) UploadAvatar(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID, file io.Reader, filename, contentType string, size int64) (*model.ProfileDetail, error) {
	if _, err := s.getWritable(ctx, p, profileUUID); err != nil {
		return nil, err
	}
	// Type and extension come from the bytes, never from the client
	// filename or Content-Type: avatar.html must not become text/html.
	sniffed, err := upload.SniffImage(file)
	if err != nil {
		return nil, err
	}
	ext := sniffed.Ext

	key := storage.ProfileAvatarObjectKey(profileUUID.String(), ext)
	var avatarURL string
	if s.storage != nil {
		err = s.storage.Upload(ctx, storage.File{
			Body:        sniffed.Body,
			Size:        size,
			ContentType: sniffed.ContentType,
			Filename:    filename,
		}, key)
		if err != nil {
			return nil, fmt.Errorf("failed to upload avatar: %w", err)
		}
		avatarURL = s.storage.PublicURL(key)
	} else {
		avatarURL = fmt.Sprintf("/api/v1/storage/stream/%s", key)
	}

	res, err := s.repo.UpdateAvatar(ctx, p.Account.ID, profileUUID, avatarURL)
	if err != nil {
		return nil, err
	}
	s.routeCache.InvalidateProfile(ctx, profileUUID)
	stored := res.CompletionRate
	s.decorate(ctx, p.Account.ID, res)
	s.syncCompletionRate(ctx, p.Account.ID, res, stored)

	go s.preCacheAllVCards(context.Background(), res)
	return res, nil
}

func (s *Service) UploadCover(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID, file io.Reader, filename, contentType string, size int64) (*model.ProfileDetail, error) {
	if _, err := s.getWritable(ctx, p, profileUUID); err != nil {
		return nil, err
	}
	// Type and extension come from the bytes, never from the client
	// filename or Content-Type: avatar.html must not become text/html.
	sniffed, err := upload.SniffImage(file)
	if err != nil {
		return nil, err
	}
	ext := sniffed.Ext

	key := storage.ProfileCoverObjectKey(profileUUID.String(), ext)
	var coverURL string
	if s.storage != nil {
		err = s.storage.Upload(ctx, storage.File{
			Body:        sniffed.Body,
			Size:        size,
			ContentType: sniffed.ContentType,
			Filename:    filename,
		}, key)
		if err != nil {
			return nil, fmt.Errorf("failed to upload cover: %w", err)
		}
		coverURL = s.storage.PublicURL(key)
	} else {
		coverURL = fmt.Sprintf("/api/v1/storage/stream/%s", key)
	}

	res, err := s.repo.UpdateCover(ctx, p.Account.ID, profileUUID, coverURL)
	if err != nil {
		return nil, err
	}
	s.decorate(ctx, p.Account.ID, res)
	return res, nil
}

func (s *Service) Delete(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) error {
	detail, err := s.getWritable(ctx, p, profileUUID)
	if err != nil {
		return err
	}

	// Collect the object keys the row points at before it is deleted (TEC-1176).
	locales := []string{detail.PrimaryLocale}
	for _, tr := range detail.Translations {
		locales = append(locales, tr.Locale)
	}
	keys := s.profileMediaKeys(profileUUID, detail.AvatarURL, detail.CoverURL, locales)

	if err := s.repo.Delete(ctx, p.Account.ID, profileUUID); err != nil {
		return err
	}
	s.routeCache.InvalidateProfile(ctx, profileUUID)
	if s.search != nil {
		s.search.EnqueueDelete(ctx, "profiles", profileUUID.String())
	}

	// The delete has committed: remove the profile's media from object storage. Failures are
	// logged by the cleaner and never undo the delete.
	if s.mediaCleaner != nil {
		s.mediaCleaner.Schedule(ctx, profileUUID, keys)
	} else if s.storage != nil {
		for _, loc := range locales {
			_ = s.storage.Delete(ctx, storage.ProfileVCardObjectKey(profileUUID.String(), loc))
		}
	}
	return nil
}

// SetSearchIndexer keeps profiles in the account-scoped Cmd+K index (TEC-951).
func (s *Service) SetSearchIndexer(e searchengine.Enqueuer) { s.search = e }

func (s *Service) indexProfile(ctx context.Context, id uuid.UUID) {
	if s.search != nil {
		s.search.EnqueueUpsert(ctx, "profiles", id.String())
	}
}

func (s *Service) SetPrimary(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) error {
	if _, err := s.getWritable(ctx, p, profileUUID); err != nil {
		return err
	}

	return s.repo.SetPrimary(ctx, p.Account.ID, profileUUID)
}

func (s *Service) GenerateVCard(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID, requestedLocale, requestedVersion string) (string, string, error) {
	detail, err := s.get(ctx, p, profileUUID)
	if err != nil {
		return "", "", err
	}

	localeKey := requestedLocale
	if localeKey == "" {
		localeKey = detail.PrimaryLocale
	}
	if localeKey == "" {
		localeKey = "tr"
	}

	// 1. Fast path: check pre-rendered cached vCard in storage
	if s.storage != nil {
		cacheKey := storage.ProfileVCardObjectKey(detail.UUID.String(), localeKey)
		rc, _, err := s.storage.Download(ctx, cacheKey)
		if err == nil {
			defer func() { _ = rc.Close() }()
			buf, err := io.ReadAll(rc)
			if err == nil && len(buf) > 0 {
				filename := s.buildVCardFilename(detail, localeKey)
				return string(buf), filename, nil
			}
		}
	}

	// 2. Slow path / Cache miss: Generate, save to cache and return
	vcardStr, filename := s.buildContactVCard(ctx, detail, localeKey)
	if s.storage != nil {
		go func(content, profUUID, loc string) {
			cacheKey := storage.ProfileVCardObjectKey(profUUID, loc)
			_ = s.storage.Upload(context.Background(), storage.File{
				Body:        strings.NewReader(content),
				Size:        int64(len(content)),
				ContentType: "text/vcard; charset=utf-8",
				Filename:    fmt.Sprintf("vcard_%s.vcf", loc),
			}, cacheKey)
		}(vcardStr, detail.UUID.String(), localeKey)
	}

	return vcardStr, filename, nil
}

func (s *Service) buildVCardFilename(detail *model.ProfileDetail, locale string) string {
	var targetTr *model.ProfileTranslationDTO
	for i := range detail.Translations {
		if detail.Translations[i].Locale == locale {
			targetTr = &detail.Translations[i]
			break
		}
	}
	if targetTr == nil && len(detail.Translations) > 0 {
		targetTr = &detail.Translations[0]
	}

	firstName := ""
	lastName := ""
	if targetTr != nil {
		firstName = targetTr.FirstName
		lastName = targetTr.LastName
	}

	filename := fmt.Sprintf("%s_%s.vcf", sanitizeFilename(firstName), sanitizeFilename(lastName))
	if filename == "_.vcf" {
		filename = fmt.Sprintf("profile_%s.vcf", detail.UUID.String()[:8])
	}
	return filename
}

func (s *Service) buildContactVCard(ctx context.Context, detail *model.ProfileDetail, locale string) (string, string) {
	var targetTr *model.ProfileTranslationDTO
	for i := range detail.Translations {
		tr := &detail.Translations[i]
		if locale != "" && tr.Locale == locale {
			targetTr = tr
			break
		}
	}
	if targetTr == nil {
		for i := range detail.Translations {
			tr := &detail.Translations[i]
			if tr.Locale == detail.PrimaryLocale || tr.IsDefault {
				targetTr = tr
				break
			}
		}
	}
	if targetTr == nil && len(detail.Translations) > 0 {
		targetTr = &detail.Translations[0]
	}

	var tr model.ProfileTranslationDTO
	if targetTr != nil {
		tr = *targetTr
	}

	var bday *time.Time
	if detail.Birthday != nil && *detail.Birthday != "" {
		t, err := time.Parse("2006-01-02", *detail.Birthday)
		if err == nil {
			bday = &t
		}
	}

	phones := make([]vcard.Phone, 0, len(detail.ContactDetails))
	emails := make([]vcard.Email, 0, len(detail.ContactDetails))
	for _, c := range detail.ContactDetails {
		switch c.Category {
		case "phone", "whatsapp":
			phones = append(phones, vcard.Phone{
				Type:      c.Type,
				Label:     c.Label,
				Number:    c.Value,
				IsPrimary: c.IsPrimary,
			})
		case "email":
			emails = append(emails, vcard.Email{
				Type:      c.Type,
				Label:     c.Label,
				Address:   c.Value,
				IsPrimary: c.IsPrimary,
			})
		}
	}

	addresses := make([]vcard.Address, 0, len(detail.Addresses))
	for _, a := range detail.Addresses {
		addresses = append(addresses, vcard.Address{
			Type:       a.Type,
			Label:      a.Label,
			Street:     a.Street,
			Extended:   a.Extended,
			City:       a.City,
			Region:     a.Region,
			PostalCode: a.PostalCode,
			Country:    a.Country,
			IsPrimary:  a.IsPrimary,
		})
	}

	socials := make([]vcard.SocialLink, 0, len(detail.SocialLinks))
	for _, sl := range detail.SocialLinks {
		if sl.IsActive && sl.URL != "" {
			socials = append(socials, vcard.SocialLink{
				Platform: sl.Platform,
				Label:    sl.Label,
				URL:      sl.URL,
			})
		}
	}

	avatarURL := detail.AvatarURL
	if avatarURL != "" {
		if strings.HasPrefix(avatarURL, "/api/v1/storage/stream/") {
			key := strings.TrimPrefix(avatarURL, "/api/v1/storage/stream/")
			if s.storage != nil {
				avatarURL = s.storage.PublicURL(key)
			}
		} else if strings.HasPrefix(avatarURL, "profiles/") {
			if s.storage != nil {
				avatarURL = s.storage.PublicURL(avatarURL)
			}
		}
	}

	photoBytes, photoMime := s.fetchPhoto(ctx, detail.UUID, detail.AvatarURL)

	contact := vcard.Contact{
		UID:           detail.UUID.String(),
		FirstName:     tr.FirstName,
		LastName:      tr.LastName,
		MiddleName:    tr.MiddleName,
		Prefix:        tr.Prefix,
		Suffix:        tr.Suffix,
		Nickname:      tr.Nickname,
		CompanyName:   tr.CompanyName,
		Department:    tr.Department,
		Title:         tr.Title,
		Role:          tr.Role,
		Bio:           tr.Bio,
		Note:          tr.Note,
		Birthday:      bday,
		Gender:        detail.Gender,
		AvatarURL:     avatarURL,
		PhotoBytes:    photoBytes,
		PhotoMimeType: photoMime,
		Phones:        phones,
		Emails:        emails,
		Addresses:     addresses,
		SocialLinks:   socials,
		Version:       vcard.Version40, // Standard RFC 6350 (vCard 4.0)
	}

	filename := fmt.Sprintf("%s_%s.vcf", sanitizeFilename(tr.FirstName), sanitizeFilename(tr.LastName))
	if filename == "_.vcf" {
		filename = fmt.Sprintf("profile_%s.vcf", detail.UUID.String()[:8])
	}

	vcardStr := s.vcardGen.Generate(contact)
	return vcardStr, filename
}

func (s *Service) preCacheAllVCards(ctx context.Context, detail *model.ProfileDetail) {
	if s.storage == nil || detail == nil {
		return
	}

	locales := make(map[string]bool)
	if detail.PrimaryLocale != "" {
		locales[detail.PrimaryLocale] = true
	} else {
		locales["tr"] = true
	}

	for _, tr := range detail.Translations {
		if tr.Locale != "" {
			locales[tr.Locale] = true
		}
	}

	for loc := range locales {
		vcfContent, _ := s.buildContactVCard(ctx, detail, loc)
		cacheKey := storage.ProfileVCardObjectKey(detail.UUID.String(), loc)
		_ = s.storage.Upload(ctx, storage.File{
			Body:        strings.NewReader(vcfContent),
			Size:        int64(len(vcfContent)),
			ContentType: "text/vcard; charset=utf-8",
			Filename:    fmt.Sprintf("vcard_%s.vcf", loc),
		}, cacheKey)
	}
}

// maxVCardPhotoBytes caps the avatar embedded into a vCard.
const maxVCardPhotoBytes = 2 << 20

// fetchPhoto reads the profile's own avatar from object storage for vCard embedding. It never
// fetches remote URLs: media is API-owned, and following user-supplied URLs server-side would be
// an SSRF primitive (the response is echoed back inside the vCard).
func (s *Service) fetchPhoto(ctx context.Context, profileUUID uuid.UUID, avatarURL string) ([]byte, string) {
	if s.storage == nil || avatarURL == "" {
		return nil, ""
	}
	key := s.mediaKey(avatarURL)
	if !strings.HasPrefix(key, profileMediaPrefix(profileUUID)) {
		return nil, ""
	}
	rc, _, err := s.storage.Download(ctx, key)
	if err != nil {
		return nil, ""
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(rc, maxVCardPhotoBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxVCardPhotoBytes {
		return nil, ""
	}
	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(key)))
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	return data, ct
}

const (
	storageStreamPrefix = "/api/v1/storage/stream/"
	tempAvatarPrefix    = "tmp/avatars/"
	// companyMediaRoot is the root of company media libraries (accounts/{uuid}/media/, TEC-946).
	companyMediaRoot = "accounts/"
)

func profileMediaPrefix(profileUUID uuid.UUID) string {
	return "profiles/" + profileUUID.String() + "/"
}

// mediaKey returns the object key behind an API-owned media URL (our storage public URL, the
// stream proxy path or a bare key), or "" when the URL was not produced by our storage.
func (s *Service) mediaKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var key string
	switch {
	case strings.HasPrefix(raw, storageStreamPrefix):
		key = strings.TrimPrefix(raw, storageStreamPrefix)
	case strings.HasPrefix(raw, "profiles/"), strings.HasPrefix(raw, tempAvatarPrefix), strings.HasPrefix(raw, companyMediaRoot):
		key = raw
	default:
		idx := -1
		for _, prefix := range []string{"profiles/", tempAvatarPrefix, companyMediaRoot} {
			if i := strings.Index(raw, prefix); i >= 0 && (idx < 0 || i < idx) {
				idx = i
			}
		}
		if idx < 0 || s.storage == nil {
			return ""
		}
		key = raw[idx:]
		if s.storage.PublicURL(key) != raw {
			return ""
		}
	}
	if strings.Contains(key, "..") || strings.Contains(key, "//") || strings.ContainsAny(key, "?#\\") {
		return ""
	}
	return key
}

// validateMediaURL accepts an empty value, a staged temp avatar, a file of the active account's
// company media library (TEC-946), or media stored under this profile. profileUUID is uuid.Nil
// on create, where profile media cannot exist yet.
func (s *Service) validateMediaURL(field, raw string, accountUUID, profileUUID uuid.UUID) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	key := s.mediaKey(raw)
	// Avatars and covers are raster-only; an SVG (e.g. a company logo from
	// the media library) is not accepted here (TEC-1097).
	if strings.HasSuffix(strings.ToLower(key), ".svg") {
		return fmt.Errorf("%w: %s must be a PNG, JPEG, WebP or GIF image", ErrInvalidRequest, field)
	}
	switch {
	case strings.HasPrefix(key, tempAvatarPrefix):
		return nil
	case accountUUID != uuid.Nil && strings.HasPrefix(key, storage.CompanyMediaPrefix(accountUUID.String())):
		return nil
	case profileUUID != uuid.Nil && strings.HasPrefix(key, profileMediaPrefix(profileUUID)):
		return nil
	default:
		return fmt.Errorf("%w: %s must be an uploaded media URL", ErrInvalidRequest, field)
	}
}

// calculateCompletionRate is the stored completion_rate of a payload (profiles/completion).
func calculateCompletionRate(in model.UpsertProfileInput) int {
	return completion.Rate(completion.FromInput(in))
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(s)
}

func (s *Service) ParseVCard(ctx context.Context, p authctx.Principal, r io.Reader) (*model.ParsedVCardProfile, error) {
	contact, err := s.vcardParser.ParseReader(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if len(contact) == 0 {
		return nil, fmt.Errorf("%w: no contact found in vcard", ErrInvalidRequest)
	}

	c := contact[0]
	res := &model.ParsedVCardProfile{
		DisplayName: c.DisplayName,
		Prefix:      c.Prefix,
		FirstName:   c.FirstName,
		LastName:    c.LastName,
		MiddleName:  c.MiddleName,
		Suffix:      c.Suffix,
		Nickname:    c.Nickname,
		CompanyName: c.CompanyName,
		Department:  c.Department,
		Title:       c.Title,
		Role:        c.Role,
		Bio:         c.Bio,
		Note:        c.Note,
		Gender:      c.Gender,
		AvatarURL:   c.AvatarURL,
	}

	if c.Birthday != nil && !c.Birthday.IsZero() {
		bdayStr := c.Birthday.Format("2006-01-02")
		res.Birthday = &bdayStr
	}

	// The vCard's declared photo type is not trusted: only sniffed raster
	// images are staged (TEC-1097).
	photoMIME := http.DetectContentType(c.PhotoBytes)
	if len(c.PhotoBytes) > 0 && upload.IsAllowedImageType(photoMIME) {
		mimeType := strings.SplitN(photoMIME, ";", 2)[0]

		// Upload photo directly to S3/MinIO tmp folder and provide S3 URL to frontend
		if s.storage != nil {
			hash := sha256.Sum256(c.PhotoBytes)
			tempID := fmt.Sprintf("%s_%d", hex.EncodeToString(hash[:8]), time.Now().Unix())
			ext, _ := upload.ImageExt(mimeType)
			tempKey := storage.ProfileTempAvatarObjectKey(tempID, ext)
			err := s.storage.Upload(ctx, storage.File{
				Body:        bytes.NewReader(c.PhotoBytes),
				Size:        int64(len(c.PhotoBytes)),
				ContentType: mimeType,
				Filename:    fmt.Sprintf("temp_avatar.%s", ext),
			}, tempKey)
			if err == nil {
				res.AvatarTempKey = tempKey
				res.AvatarURL = s.storage.PublicURL(tempKey)
			}
		} else {
			// Fallback only if no storage is configured at all
			b64 := base64.StdEncoding.EncodeToString(c.PhotoBytes)
			res.AvatarDataURL = fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
		}
	}

	// Contact details (Phones & Emails)
	var contactDetails []model.CreateContactDetailInput
	order := 0

	for _, ph := range c.Phones {
		if strings.TrimSpace(ph.Number) == "" {
			continue
		}
		category := "phone"
		phType := ph.Type
		if phType == "fax" {
			phType = "work"
		}
		contactDetails = append(contactDetails, model.CreateContactDetailInput{
			Category:     category,
			Type:         phType,
			Label:        ph.Label,
			Value:        ph.Number,
			IsPrimary:    ph.IsPrimary,
			DisplayOrder: order,
		})
		order++
	}

	for _, em := range c.Emails {
		if strings.TrimSpace(em.Address) == "" {
			continue
		}
		contactDetails = append(contactDetails, model.CreateContactDetailInput{
			Category:     "email",
			Type:         em.Type,
			Label:        em.Label,
			Value:        em.Address,
			IsPrimary:    em.IsPrimary,
			DisplayOrder: order,
		})
		order++
	}
	res.ContactDetails = contactDetails

	// Addresses
	var addresses []model.CreateAddressInput
	for idx, ad := range c.Addresses {
		addresses = append(addresses, model.CreateAddressInput{
			Type:         ad.Type,
			Label:        ad.Label,
			Street:       ad.Street,
			Extended:     ad.Extended,
			City:         ad.City,
			Region:       ad.Region,
			PostalCode:   ad.PostalCode,
			Country:      ad.Country,
			IsPrimary:    ad.IsPrimary,
			DisplayOrder: idx,
		})
	}
	res.Addresses = addresses

	// Social Links
	var socials []model.CreateSocialLinkInput
	for idx, sl := range c.SocialLinks {
		if strings.TrimSpace(sl.URL) == "" {
			continue
		}
		socials = append(socials, model.CreateSocialLinkInput{
			Platform:     sl.Platform,
			Label:        sl.Label,
			URL:          sl.URL,
			DisplayOrder: idx,
			IsActive:     true,
		})
	}
	res.SocialLinks = socials

	return res, nil
}
