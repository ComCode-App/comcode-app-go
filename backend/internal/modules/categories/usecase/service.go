package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/repository"
	"github.com/google/uuid"
)

var (
	ErrNotFound     = errors.New("category not found")
	ErrSlugTaken    = errors.New("category slug already exists")
	ErrNameRequired = errors.New("name is required")
)

type Service struct {
	repo repository.Repository
}

func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, q *string, isActive *bool, limit, offset int32) ([]model.CategorySummary, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	items, err := s.repo.List(ctx, q, isActive, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.repo.Count(ctx, q, isActive)
	if err != nil {
		return nil, 0, err
	}

	return items, count, nil
}

func (s *Service) ListActive(ctx context.Context) ([]model.CategorySummary, error) {
	return s.repo.ListActive(ctx)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.CategoryDetail, error) {
	cat, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, ErrNotFound
	}
	return cat, nil
}

func (s *Service) Create(ctx context.Context, input model.UpsertCategoryInput) (*model.CategoryDetail, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, ErrNameRequired
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

func (s *Service) Update(ctx context.Context, id uuid.UUID, input model.UpsertCategoryInput) (*model.CategoryDetail, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, ErrNameRequired
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

	cat, err := s.repo.Update(ctx, id, input, slug)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, ErrNotFound
	}
	return cat, nil
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
		s = "category"
	}
	return s
}

func sanitizeSlug(slug string) string {
	return generateSlug(slug)
}
