package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/banks/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/banks/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
)

var (
	ErrNotFound      = errors.New("bank not found")
	ErrSlugTaken     = errors.New("bank slug already exists")
	ErrNameRequired  = errors.New("name is required")
	ErrCodeRequired  = errors.New("bank code is required")
)

type Service struct {
	repo    repository.Repository
	storage storage.Driver
}

func NewService(repo repository.Repository, store storage.Driver) *Service {
	return &Service{repo: repo, storage: store}
}

func (s *Service) List(ctx context.Context, q *string, countryCode *string, isActive *bool, limit, offset int32) ([]model.BankSummary, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	items, err := s.repo.List(ctx, q, countryCode, isActive, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.repo.Count(ctx, q, countryCode, isActive)
	if err != nil {
		return nil, 0, err
	}

	return items, count, nil
}

func (s *Service) ListActive(ctx context.Context) ([]model.BankSummary, error) {
	return s.repo.ListActive(ctx)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.BankDetail, error) {
	bank, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if bank == nil {
		return nil, ErrNotFound
	}
	return bank, nil
}

func (s *Service) GetByCode(ctx context.Context, code string) (*model.BankDetail, error) {
	bank, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if bank == nil {
		return nil, ErrNotFound
	}
	return bank, nil
}

func (s *Service) Create(ctx context.Context, input model.UpsertBankInput) (*model.BankDetail, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, ErrNameRequired
	}
	if strings.TrimSpace(input.Code) == "" {
		return nil, ErrCodeRequired
	}

	slug := input.Slug
	if slug == "" {
		slug = generateSlug(input.Name)
	} else {
		slug = sanitizeSlug(slug)
	}

	exists, err := s.repo.CheckSlugExists(ctx, slug, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		slug = fmt.Sprintf("%s-%s", slug, uuid.New().String()[:8])
	}

	return s.repo.Create(ctx, input, slug)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input model.UpsertBankInput) (*model.BankDetail, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, ErrNameRequired
	}
	if strings.TrimSpace(input.Code) == "" {
		return nil, ErrCodeRequired
	}

	slug := input.Slug
	if slug == "" {
		slug = generateSlug(input.Name)
	} else {
		slug = sanitizeSlug(slug)
	}

	exists, err := s.repo.CheckSlugExists(ctx, slug, &id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrSlugTaken
	}

	bank, err := s.repo.Update(ctx, id, input, slug)
	if err != nil {
		return nil, err
	}
	if bank == nil {
		return nil, ErrNotFound
	}
	return bank, nil
}

func (s *Service) UpdateIcon(ctx context.Context, id uuid.UUID, iconURL string) (*model.BankDetail, error) {
	bank, err := s.repo.UpdateIcon(ctx, id, iconURL)
	if err != nil {
		return nil, err
	}
	if bank == nil {
		return nil, ErrNotFound
	}
	return bank, nil
}

func (s *Service) UploadIcon(ctx context.Context, id uuid.UUID, file io.Reader, filename, contentType string, size int64) (*model.BankDetail, error) {
	bank, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if bank == nil {
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

	key := storage.BankIconObjectKey(bank.Slug, ext)
	var iconURL string
	if s.storage != nil {
		err = s.storage.Upload(ctx, storage.File{
			Body:        file,
			Size:        size,
			ContentType: contentType,
			Filename:    filename,
		}, key)
		if err != nil {
			return nil, fmt.Errorf("failed to upload bank icon: %w", err)
		}
		iconURL = s.storage.PublicURL(key)
	} else {
		iconURL = fmt.Sprintf("/api/v1/storage/stream/%s", key)
	}

	return s.repo.UpdateIcon(ctx, id, iconURL)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func generateSlug(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, "ğ", "g")
	s = strings.ReplaceAll(s, "ü", "u")
	s = strings.ReplaceAll(s, "ş", "s")
	s = strings.ReplaceAll(s, "ı", "i")
	s = strings.ReplaceAll(s, "ö", "o")
	s = strings.ReplaceAll(s, "ç", "c")

	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "bank"
	}
	return s
}

func sanitizeSlug(slug string) string {
	return generateSlug(slug)
}
