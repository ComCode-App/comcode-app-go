package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	"io"
	"regexp"
	"strings"

	categoriesRepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("product not found")
	ErrVariantNotFound = errors.New("product variant not found")
	ErrSlugTaken       = errors.New("product slug already exists")
	ErrNameRequired    = errors.New("name is required")
)

type Service struct {
	repo         repository.Repository
	categoryRepo categoriesRepo.Repository
	storage      storage.Driver
	search       searchengine.Enqueuer
}

func NewService(repo repository.Repository, categoryRepo categoriesRepo.Repository, store storage.Driver) *Service {
	return &Service{
		repo:         repo,
		categoryRepo: categoryRepo,
		storage:      store,
	}
}

func (s *Service) List(ctx context.Context, q *string, categoryID *int64, status *string, productType *string, limit, offset int32) ([]model.ProductSummary, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	items, err := s.repo.List(ctx, q, categoryID, status, productType, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.repo.Count(ctx, q, categoryID, status, productType)
	if err != nil {
		return nil, 0, err
	}

	return items, count, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.ProductDetail, error) {
	prod, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if prod == nil {
		return nil, ErrNotFound
	}
	return prod, nil
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (*model.ProductDetail, error) {
	prod, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if prod == nil {
		return nil, ErrNotFound
	}
	return prod, nil
}

func (s *Service) Create(ctx context.Context, input model.UpsertProductInput) (*model.ProductDetail, error) {
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

	var categoryID *int64
	if input.CategoryUUID != nil && *input.CategoryUUID != "" {
		catUUID, err := uuid.Parse(*input.CategoryUUID)
		if err == nil && s.categoryRepo != nil {
			cat, _ := s.categoryRepo.GetByUUID(ctx, catUUID)
			if cat != nil {
				categoryID = &cat.ID
			}
		}
	}

	return s.indexed(s.repo.Create(ctx, input, categoryID, slug))
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input model.UpsertProductInput) (*model.ProductDetail, error) {
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

	var categoryID *int64
	if input.CategoryUUID != nil && *input.CategoryUUID != "" {
		catUUID, err := uuid.Parse(*input.CategoryUUID)
		if err == nil && s.categoryRepo != nil {
			cat, _ := s.categoryRepo.GetByUUID(ctx, catUUID)
			if cat != nil {
				categoryID = &cat.ID
			}
		}
	}

	prod, err := s.repo.Update(ctx, id, input, categoryID, slug)
	if err != nil {
		return nil, err
	}
	if prod == nil {
		return nil, ErrNotFound
	}
	return prod, nil
}

func (s *Service) Update3DViewbox(ctx context.Context, id uuid.UUID, input model.Update3DViewboxInput) (*model.ProductDetail, error) {
	prod, err := s.repo.Update3DViewbox(ctx, id, input.Model3DViewbox, input.Model3DFileURL, input.Model3DUSDZURL)
	if err != nil {
		return nil, err
	}
	if prod == nil {
		return nil, ErrNotFound
	}
	return prod, nil
}

func (s *Service) Upload3DModel(ctx context.Context, id uuid.UUID, file io.Reader, filename, contentType string, size int64, isUSDZ bool) (*model.ProductDetail, error) {
	prod, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if prod == nil {
		return nil, ErrNotFound
	}

	// Only binary glTF (GLB) and USDZ are stored; the format comes from the
	// bytes and the content type is server-chosen (TEC-1097).
	head := make([]byte, 4)
	n, _ := io.ReadFull(file, head)
	head = head[:n]
	var ext, modelType string
	switch {
	case bytes.Equal(head, []byte("glTF")):
		ext, modelType = "glb", "model/gltf-binary"
	case bytes.Equal(head, []byte("PK\x03\x04")):
		ext, modelType = "usdz", "model/vnd.usdz+zip"
	default:
		return nil, fmt.Errorf("%w: 3D models must be GLB or USDZ", upload.ErrUnsupportedType)
	}
	if isUSDZ && ext != "usdz" {
		return nil, fmt.Errorf("%w: format=usdz needs a USDZ file", upload.ErrUnsupportedType)
	}
	file = io.MultiReader(bytes.NewReader(head), file)

	modelID := uuid.New().String()[:8]
	key := storage.Product3DModelObjectKey(prod.Slug, modelID, ext)

	var fileURL string
	if s.storage != nil {
		err = s.storage.Upload(ctx, storage.File{
			Body:        file,
			Size:        size,
			ContentType: modelType,
			Filename:    filename,
		}, key)
		if err != nil {
			return nil, fmt.Errorf("failed to upload 3d model: %w", err)
		}
		fileURL = s.storage.PublicURL(key)
	} else {
		fileURL = fmt.Sprintf("/api/v1/storage/stream/%s", key)
	}

	var updateInput model.Update3DViewboxInput
	updateInput.Model3DViewbox = prod.Model3DViewbox
	if isUSDZ || ext == "usdz" {
		updateInput.Model3DUSDZURL = fileURL
		updateInput.Model3DFileURL = prod.Model3DFileURL
	} else {
		updateInput.Model3DFileURL = fileURL
		updateInput.Model3DUSDZURL = prod.Model3DUSDZURL
	}

	return s.repo.Update3DViewbox(ctx, id, updateInput.Model3DViewbox, updateInput.Model3DFileURL, updateInput.Model3DUSDZURL)
}

func (s *Service) UploadImage(ctx context.Context, id uuid.UUID, file io.Reader, filename, contentType string, size int64) (*model.ProductDetail, error) {
	prod, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if prod == nil {
		return nil, ErrNotFound
	}

	// Type and extension come from the bytes, never from the client
	// filename or Content-Type (TEC-1097).
	sniffed, err := upload.SniffImage(file)
	if err != nil {
		return nil, err
	}
	ext := sniffed.Ext

	imgID := uuid.New().String()[:8]
	key := storage.ProductImageObjectKey(prod.Slug, imgID, ext)

	var imageURL string
	if s.storage != nil {
		err = s.storage.Upload(ctx, storage.File{
			Body:        sniffed.Body,
			Size:        size,
			ContentType: sniffed.ContentType,
			Filename:    filename,
		}, key)
		if err != nil {
			return nil, fmt.Errorf("failed to upload product image: %w", err)
		}
		imageURL = s.storage.PublicURL(key)
	} else {
		imageURL = fmt.Sprintf("/api/v1/storage/stream/%s", key)
	}

	newImages := append(prod.Images, imageURL)
	updateInput := model.UpsertProductInput{
		Name:             prod.Name,
		Slug:             prod.Slug,
		ShortDescription: prod.ShortDescription,
		Description:      prod.Description,
		Status:           prod.Status,
		ProductType:      prod.ProductType,
		BasePrice:        prod.BasePrice,
		Currency:         prod.Currency,
		TaxRate:          prod.TaxRate,
		StockQuantity:    prod.StockQuantity,
		SKU:              prod.SKU,
		AllowCustomPrint: prod.AllowCustomPrint,
		Model3DFileURL:   prod.Model3DFileURL,
		Model3DUSDZURL:   prod.Model3DUSDZURL,
		Model3DViewbox:   &prod.Model3DViewbox,
		Images:           newImages,
		MetaTitle:        prod.MetaTitle,
		MetaDescription:  prod.MetaDescription,
		MetaKeywords:     prod.MetaKeywords,
		CanonicalURL:     prod.CanonicalURL,
		OGImageURL:       prod.OGImageURL,
		DisplayOrder:     prod.DisplayOrder,
	}

	return s.indexed(s.repo.Update(ctx, id, updateInput, prod.CategoryID, prod.Slug))
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if s.search != nil {
		s.search.EnqueueDelete(ctx, "products", id.String())
	}
	return nil
}

// SetSearchIndexer keeps products in the Cmd+K index (TEC-595).
func (s *Service) SetSearchIndexer(e searchengine.Enqueuer) { s.search = e }

func (s *Service) indexed(prod *model.ProductDetail, err error) (*model.ProductDetail, error) {
	if err == nil && prod != nil && s.search != nil {
		s.search.EnqueueUpsert(context.Background(), "products", prod.UUID)
	}
	return prod, err
}

// Variants
func (s *Service) ListVariants(ctx context.Context, productUUID uuid.UUID) ([]model.ProductVariantSummary, error) {
	prod, err := s.repo.GetByUUID(ctx, productUUID)
	if err != nil {
		return nil, err
	}
	if prod == nil {
		return nil, ErrNotFound
	}
	return s.repo.ListVariants(ctx, prod.ID)
}

func (s *Service) CreateVariant(ctx context.Context, productUUID uuid.UUID, input model.UpsertVariantInput) (*model.ProductVariantSummary, error) {
	prod, err := s.repo.GetByUUID(ctx, productUUID)
	if err != nil {
		return nil, err
	}
	if prod == nil {
		return nil, ErrNotFound
	}

	if strings.TrimSpace(input.Name) == "" {
		return nil, ErrNameRequired
	}

	return s.repo.CreateVariant(ctx, prod.ID, input)
}

func (s *Service) UpdateVariant(ctx context.Context, variantUUID uuid.UUID, input model.UpsertVariantInput) (*model.ProductVariantSummary, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, ErrNameRequired
	}
	v, err := s.repo.UpdateVariant(ctx, variantUUID, input)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrVariantNotFound
	}
	return v, nil
}

func (s *Service) DeleteVariant(ctx context.Context, variantUUID uuid.UUID) error {
	return s.repo.DeleteVariant(ctx, variantUUID)
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
		s = "product"
	}
	return s
}

func sanitizeSlug(slug string) string {
	return generateSlug(slug)
}
