package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/opengraph"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
)

var (
	ErrNotFound      = errors.New("social platform not found")
	ErrSlugTaken     = errors.New("slug is already taken")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInvalidImage  = errors.New("invalid image file")
)

type Service struct {
	repo     repository.Repository
	storage  storage.Driver
	ogClient *opengraph.Client
	httpCl   *http.Client
}

func New(repo repository.Repository, store storage.Driver, ogClient *opengraph.Client) *Service {
	if ogClient == nil {
		ogClient = opengraph.NewClient()
	}
	return &Service{
		repo:     repo,
		storage:  store,
		ogClient: ogClient,
		httpCl: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (s *Service) List(ctx context.Context, q *string, category *string, isActive *bool, limit, offset int32) ([]model.SocialPlatformSummary, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	items, err := s.repo.List(ctx, q, category, isActive, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx, q, category, isActive)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Service) ListActive(ctx context.Context) ([]model.SocialPlatformSummary, error) {
	return s.repo.ListActive(ctx)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.SocialPlatformDetail, error) {
	platform, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if platform == nil {
		return nil, ErrNotFound
	}
	return platform, nil
}

func (s *Service) Create(ctx context.Context, input model.UpsertSocialPlatformInput) (*model.SocialPlatformDetail, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	slug := sanitizeSlug(input.Slug)
	if slug == "" {
		slug = sanitizeSlug(input.Name)
	}
	if slug == "" {
		slug = "platform-" + uuid.NewString()[:8]
	}

	exists, err := s.repo.CheckSlugExists(ctx, slug, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		slug = fmt.Sprintf("%s-%s", slug, uuid.NewString()[:6])
	}

	return s.repo.Create(ctx, input, slug)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input model.UpsertSocialPlatformInput) (*model.SocialPlatformDetail, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}

	existing, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrNotFound
	}

	slug := sanitizeSlug(input.Slug)
	if slug == "" {
		slug = existing.Slug
	}

	if slug != existing.Slug {
		exists, err := s.repo.CheckSlugExists(ctx, slug, &id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrSlugTaken
		}
	}

	return s.repo.Update(ctx, id, input, slug)
}

func (s *Service) UploadIcon(ctx context.Context, id uuid.UUID, file io.Reader, filename, contentType string, size int64) (*model.SocialPlatformDetail, error) {
	platform, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if platform == nil {
		return nil, ErrNotFound
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		exts, _ := mime.ExtensionsByType(contentType)
		if len(exts) > 0 {
			ext = exts[0]
		} else {
			ext = ".png"
		}
	}
	ext = strings.TrimPrefix(ext, ".")

	key := storage.SocialPlatformIconObjectKey(platform.Slug, ext)
	var iconURL string
	if s.storage != nil {
		err = s.storage.Upload(ctx, storage.File{
			Body:        file,
			Size:        size,
			ContentType: contentType,
			Filename:    filename,
		}, key)
		if err != nil {
			return nil, fmt.Errorf("failed to upload icon: %w", err)
		}
		iconURL = s.storage.PublicURL(key)
	} else {
		iconURL = fmt.Sprintf("/api/v1/storage/stream/%s", key)
	}

	return s.repo.UpdateIcon(ctx, id, iconURL)
}

// FetchIcon resolves platform icon via OpenGraph / Favicon scraper and uploads to S3 or saves URL.
func (s *Service) FetchIcon(ctx context.Context, id uuid.UUID) (*model.SocialPlatformDetail, error) {
	platform, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if platform == nil {
		return nil, ErrNotFound
	}

	targetURL := resolvePlatformTargetURL(platform.URLPattern, platform.Slug)
	meta, err := s.ogClient.Fetch(ctx, targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch icon metadata: %w", err)
	}

	iconSource := meta.Favicon
	if iconSource == "" {
		iconSource = meta.ImageURL
	}
	if iconSource == "" {
		iconSource = fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=128", platform.Slug+".com")
	}

	// Try downloading icon image to persist to S3
	if s.storage != nil && iconSource != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, iconSource, nil)
		if err == nil {
			req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ComCode-Bot/1.0)")
			resp, err := s.httpCl.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer func() { _ = resp.Body.Close() }()
				data, err := io.ReadAll(resp.Body)
				if err == nil && len(data) > 0 {
					ext := "png"
					ct := resp.Header.Get("Content-Type")
					if strings.Contains(ct, "svg") {
						ext = "svg"
					} else if strings.Contains(ct, "webp") {
						ext = "webp"
					} else if strings.Contains(ct, "ico") {
						ext = "ico"
					}

					key := storage.SocialPlatformIconObjectKey(platform.Slug, ext)
					err = s.storage.Upload(ctx, storage.File{
						Body:        bytes.NewReader(data),
						Size:        int64(len(data)),
						ContentType: ct,
						Filename:    fmt.Sprintf("%s-icon.%s", platform.Slug, ext),
					}, key)
					if err == nil {
						savedURL := s.storage.PublicURL(key)
						return s.repo.UpdateIcon(ctx, id, savedURL)
					}
				}
			}
		}
	}

	// Fallback to storing direct remote icon URL
	return s.repo.UpdateIcon(ctx, id, iconSource)
}

// BulkFetchIcons iterates platforms and resolves their icons via OpenGraph scraper.
func (s *Service) BulkFetchIcons(ctx context.Context, uuids []uuid.UUID) (int, error) {
	var targetIDs []uuid.UUID

	if len(uuids) > 0 {
		targetIDs = uuids
	} else {
		// List all active platforms
		all, err := s.repo.List(ctx, nil, nil, nil, 200, 0)
		if err != nil {
			return 0, err
		}
		for _, p := range all {
			id, err := uuid.Parse(p.UUID)
			if err == nil {
				targetIDs = append(targetIDs, id)
			}
		}
	}

	count := 0
	for _, id := range targetIDs {
		if _, err := s.FetchIcon(ctx, id); err == nil {
			count++
		}
	}

	return count, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	existing, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrNotFound
	}
	return s.repo.Delete(ctx, id)
}

func resolvePlatformTargetURL(urlPattern, slug string) string {
	if urlPattern != "" {
		cleaned := strings.ReplaceAll(urlPattern, "{value}", "")
		cleaned = strings.TrimRight(cleaned, "/")
		if u, err := url.Parse(cleaned); err == nil && u.Host != "" {
			return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		}
		if strings.HasPrefix(cleaned, "http://") || strings.HasPrefix(cleaned, "https://") {
			return cleaned
		}
	}
	return "https://" + slug + ".com"
}

func sanitizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(
		"ç", "c", "ğ", "g", "ı", "i", "ö", "o", "ş", "s", "ü", "u",
		" ", "-", "_", "-", ".", "-", "/", "-",
	)
	s = replacer.Replace(s)
	reg := regexp.MustCompile("[^a-z0-9-]+")
	s = reg.ReplaceAllString(s, "")
	s = regexp.MustCompile("-+").ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
