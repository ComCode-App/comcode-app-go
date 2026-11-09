package repository

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/model"
)

type PostgresRepository struct {
	q *db.Queries
}

func NewPostgres(q *db.Queries) *PostgresRepository {
	return &PostgresRepository{q: q}
}

func (r *PostgresRepository) List(ctx context.Context, q *string, categoryID *int64, status *string, productType *string, limit, offset int32) ([]model.ProductSummary, error) {
	rows, err := r.q.ListProducts(ctx, db.ListProductsParams{
		Q:           textToPgtype(q),
		CategoryID:  int64ToPgtype(categoryID),
		Status:      textToPgtype(status),
		ProductType: textToPgtype(productType),
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, err
	}

	result := make([]model.ProductSummary, 0, len(rows))
	for _, row := range rows {
		var images []string
		if len(row.Images) > 0 {
			_ = json.Unmarshal(row.Images, &images)
		}
		result = append(result, model.ProductSummary{
			UUID:             row.Uuid.String(),
			CategoryName:     pgtypeToString(row.CategoryName),
			CategorySlug:     pgtypeToString(row.CategorySlug),
			Name:             row.Name,
			Slug:             row.Slug,
			ShortDescription: row.ShortDescription,
			Status:           row.Status,
			ProductType:      row.ProductType,
			BasePrice:        numericToFloat(row.BasePrice),
			Currency:         row.Currency,
			StockQuantity:    int(row.StockQuantity),
			SKU:              row.Sku,
			AllowCustomPrint: row.AllowCustomPrint,
			Model3DFileURL:   row.Model3dFileUrl,
			Model3DUSDZURL:   row.Model3dUsdzUrl,
			Images:           images,
			DisplayOrder:     int(row.DisplayOrder),
			CreatedAt:        row.CreatedAt.Time,
		})
	}
	return result, nil
}

func (r *PostgresRepository) Count(ctx context.Context, q *string, categoryID *int64, status *string, productType *string) (int64, error) {
	return r.q.CountProducts(ctx, db.CountProductsParams{
		Q:           textToPgtype(q),
		CategoryID:  int64ToPgtype(categoryID),
		Status:      textToPgtype(status),
		ProductType: textToPgtype(productType),
	})
}

func (r *PostgresRepository) GetByUUID(ctx context.Context, id uuid.UUID) (*model.ProductDetail, error) {
	row, err := r.q.GetProductByUUID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	detail := toProductDetailFromGetRow(row)

	variants, err := r.ListVariants(ctx, row.ID)
	if err == nil {
		detail.Variants = variants
	}
	return detail, nil
}

func (r *PostgresRepository) GetBySlug(ctx context.Context, slug string) (*model.ProductDetail, error) {
	row, err := r.q.GetProductBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	detail := toProductDetailFromSlugRow(row)

	variants, err := r.ListVariants(ctx, row.ID)
	if err == nil {
		detail.Variants = variants
	}
	return detail, nil
}

func (r *PostgresRepository) Create(ctx context.Context, input model.UpsertProductInput, categoryID *int64, slug string) (*model.ProductDetail, error) {
	imagesJSON, _ := json.Marshal(input.Images)
	if len(input.Images) == 0 {
		imagesJSON = []byte("[]")
	}

	viewboxJSON := []byte("{}")
	if input.Model3DViewbox != nil {
		viewboxJSON, _ = json.Marshal(input.Model3DViewbox)
	}

	status := input.Status
	if status == "" {
		status = "draft"
	}
	prodType := input.ProductType
	if prodType == "" {
		prodType = "physical_card"
	}
	curr := input.Currency
	if curr == "" {
		curr = "TRY"
	}
	taxRate := input.TaxRate
	if taxRate <= 0 {
		taxRate = 20.00
	}

	row, err := r.q.CreateProduct(ctx, db.CreateProductParams{
		CategoryID:       int64ToPgtype(categoryID),
		Name:             input.Name,
		Slug:             slug,
		ShortDescription: input.ShortDescription,
		Description:      input.Description,
		Status:           status,
		ProductType:      prodType,
		BasePrice:        floatToNumeric(input.BasePrice),
		Currency:         curr,
		TaxRate:          floatToNumeric(taxRate),
		StockQuantity:    int32(input.StockQuantity),
		Sku:              input.SKU,
		AllowCustomPrint: input.AllowCustomPrint,
		Model3dFileUrl:   input.Model3DFileURL,
		Model3dUsdzUrl:   input.Model3DUSDZURL,
		Model3dViewbox:   viewboxJSON,
		Images:           imagesJSON,
		MetaTitle:        input.MetaTitle,
		MetaDescription:  input.MetaDescription,
		MetaKeywords:     input.MetaKeywords,
		CanonicalUrl:     input.CanonicalURL,
		OgImageUrl:       input.OGImageURL,
		DisplayOrder:     int32(input.DisplayOrder),
	})
	if err != nil {
		return nil, err
	}

	return toProductDetailFromProduct(row), nil
}

func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, input model.UpsertProductInput, categoryID *int64, slug string) (*model.ProductDetail, error) {
	var imagesJSON []byte
	if input.Images != nil {
		imagesJSON, _ = json.Marshal(input.Images)
	}

	var viewboxJSON []byte
	if input.Model3DViewbox != nil {
		viewboxJSON, _ = json.Marshal(input.Model3DViewbox)
	}

	row, err := r.q.UpdateProduct(ctx, db.UpdateProductParams{
		Uuid:             id,
		CategoryID:       int64ToPgtype(categoryID),
		Name:             textToPgtype(&input.Name),
		Slug:             textToPgtype(&slug),
		ShortDescription: pgtype.Text{String: input.ShortDescription, Valid: true},
		Description:      pgtype.Text{String: input.Description, Valid: true},
		Status:           textToPgtype(&input.Status),
		ProductType:      textToPgtype(&input.ProductType),
		BasePrice:        floatToNumeric(input.BasePrice),
		Currency:         textToPgtype(&input.Currency),
		TaxRate:          floatToNumeric(input.TaxRate),
		StockQuantity:    pgtype.Int4{Int32: int32(input.StockQuantity), Valid: true},
		Sku:              pgtype.Text{String: input.SKU, Valid: true},
		AllowCustomPrint: pgtype.Bool{Bool: input.AllowCustomPrint, Valid: true},
		Model3dFileUrl:   pgtype.Text{String: input.Model3DFileURL, Valid: true},
		Model3dUsdzUrl:   pgtype.Text{String: input.Model3DUSDZURL, Valid: true},
		Model3dViewbox:   viewboxJSON,
		Images:           imagesJSON,
		MetaTitle:        pgtype.Text{String: input.MetaTitle, Valid: true},
		MetaDescription:  pgtype.Text{String: input.MetaDescription, Valid: true},
		MetaKeywords:     pgtype.Text{String: input.MetaKeywords, Valid: true},
		CanonicalUrl:     pgtype.Text{String: input.CanonicalURL, Valid: true},
		OgImageUrl:       pgtype.Text{String: input.OGImageURL, Valid: true},
		DisplayOrder:     pgtype.Int4{Int32: int32(input.DisplayOrder), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toProductDetailFromProduct(row), nil
}

func (r *PostgresRepository) Update3DViewbox(ctx context.Context, id uuid.UUID, viewbox model.Model3DViewbox, fileURL, usdzURL string) (*model.ProductDetail, error) {
	viewboxJSON, err := json.Marshal(viewbox)
	if err != nil {
		return nil, err
	}

	row, err := r.q.UpdateProduct3DViewbox(ctx, db.UpdateProduct3DViewboxParams{
		Uuid:           id,
		Model3dViewbox: viewboxJSON,
		Model3dFileUrl: textToPgtype(&fileURL),
		Model3dUsdzUrl: textToPgtype(&usdzURL),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toProductDetailFromProduct(row), nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteProduct(ctx, id)
}

func (r *PostgresRepository) CheckSlugExists(ctx context.Context, slug string, excludeUUID *uuid.UUID) (bool, error) {
	var exc pgtype.UUID
	if excludeUUID != nil {
		exc = pgtype.UUID{Bytes: *excludeUUID, Valid: true}
	}
	return r.q.CheckProductSlugExists(ctx, db.CheckProductSlugExistsParams{
		Slug:        slug,
		ExcludeUuid: exc,
	})
}

// Variants
func (r *PostgresRepository) ListVariants(ctx context.Context, productID int64) ([]model.ProductVariantSummary, error) {
	rows, err := r.q.ListProductVariantsByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	result := make([]model.ProductVariantSummary, 0, len(rows))
	for _, row := range rows {
		var vb model.Model3DViewbox
		if len(row.Model3dViewbox) > 0 {
			_ = json.Unmarshal(row.Model3dViewbox, &vb)
		}
		result = append(result, model.ProductVariantSummary{
			UUID:           row.Uuid.String(),
			Name:           row.Name,
			SKU:            row.Sku,
			PriceDelta:     numericToFloat(row.PriceDelta),
			StockQuantity:  int(row.StockQuantity),
			Material:       row.Material,
			ColorCode:      row.ColorCode,
			ImageURL:       row.ImageUrl,
			Model3DFileURL: row.Model3dFileUrl,
			Model3DUSDZURL: row.Model3dUsdzUrl,
			Model3DViewbox: vb,
			IsActive:       row.IsActive,
			DisplayOrder:   int(row.DisplayOrder),
		})
	}
	return result, nil
}

func (r *PostgresRepository) GetVariantByUUID(ctx context.Context, id uuid.UUID) (*model.ProductVariantSummary, error) {
	row, err := r.q.GetProductVariantByUUID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	var vb model.Model3DViewbox
	if len(row.Model3dViewbox) > 0 {
		_ = json.Unmarshal(row.Model3dViewbox, &vb)
	}
	return &model.ProductVariantSummary{
		UUID:           row.Uuid.String(),
		Name:           row.Name,
		SKU:            row.Sku,
		PriceDelta:     numericToFloat(row.PriceDelta),
		StockQuantity:  int(row.StockQuantity),
		Material:       row.Material,
		ColorCode:      row.ColorCode,
		ImageURL:       row.ImageUrl,
		Model3DFileURL: row.Model3dFileUrl,
		Model3DUSDZURL: row.Model3dUsdzUrl,
		Model3DViewbox: vb,
		IsActive:       row.IsActive,
		DisplayOrder:   int(row.DisplayOrder),
	}, nil
}

func (r *PostgresRepository) CreateVariant(ctx context.Context, productID int64, input model.UpsertVariantInput) (*model.ProductVariantSummary, error) {
	vbJSON := []byte("{}")
	if input.Model3DViewbox != nil {
		vbJSON, _ = json.Marshal(input.Model3DViewbox)
	}

	row, err := r.q.CreateProductVariant(ctx, db.CreateProductVariantParams{
		ProductID:      productID,
		Name:           input.Name,
		Sku:            input.SKU,
		PriceDelta:     floatToNumeric(input.PriceDelta),
		StockQuantity:  int32(input.StockQuantity),
		Material:       input.Material,
		ColorCode:      input.ColorCode,
		ImageUrl:       input.ImageURL,
		Model3dFileUrl: input.Model3DFileURL,
		Model3dUsdzUrl: input.Model3DUSDZURL,
		Model3dViewbox: vbJSON,
		IsActive:       input.IsActive,
		DisplayOrder:   int32(input.DisplayOrder),
	})
	if err != nil {
		return nil, err
	}

	return &model.ProductVariantSummary{
		UUID:           row.Uuid.String(),
		Name:           row.Name,
		SKU:            row.Sku,
		PriceDelta:     numericToFloat(row.PriceDelta),
		StockQuantity:  int(row.StockQuantity),
		Material:       row.Material,
		ColorCode:      row.ColorCode,
		ImageURL:       row.ImageUrl,
		Model3DFileURL: row.Model3dFileUrl,
		Model3DUSDZURL: row.Model3dUsdzUrl,
		IsActive:       row.IsActive,
		DisplayOrder:   int(row.DisplayOrder),
	}, nil
}

func (r *PostgresRepository) UpdateVariant(ctx context.Context, id uuid.UUID, input model.UpsertVariantInput) (*model.ProductVariantSummary, error) {
	var vbJSON []byte
	if input.Model3DViewbox != nil {
		vbJSON, _ = json.Marshal(input.Model3DViewbox)
	}

	row, err := r.q.UpdateProductVariant(ctx, db.UpdateProductVariantParams{
		Uuid:           id,
		Name:           textToPgtype(&input.Name),
		Sku:            pgtype.Text{String: input.SKU, Valid: true},
		PriceDelta:     floatToNumeric(input.PriceDelta),
		StockQuantity:  pgtype.Int4{Int32: int32(input.StockQuantity), Valid: true},
		Material:       pgtype.Text{String: input.Material, Valid: true},
		ColorCode:      pgtype.Text{String: input.ColorCode, Valid: true},
		ImageUrl:       pgtype.Text{String: input.ImageURL, Valid: true},
		Model3dFileUrl: pgtype.Text{String: input.Model3DFileURL, Valid: true},
		Model3dUsdzUrl: pgtype.Text{String: input.Model3DUSDZURL, Valid: true},
		Model3dViewbox: vbJSON,
		IsActive:       pgtype.Bool{Bool: input.IsActive, Valid: true},
		DisplayOrder:   pgtype.Int4{Int32: int32(input.DisplayOrder), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &model.ProductVariantSummary{
		UUID:           row.Uuid.String(),
		Name:           row.Name,
		SKU:            row.Sku,
		PriceDelta:     numericToFloat(row.PriceDelta),
		StockQuantity:  int(row.StockQuantity),
		Material:       row.Material,
		ColorCode:      row.ColorCode,
		ImageURL:       row.ImageUrl,
		Model3DFileURL: row.Model3dFileUrl,
		Model3DUSDZURL: row.Model3dUsdzUrl,
		IsActive:       row.IsActive,
		DisplayOrder:   int(row.DisplayOrder),
	}, nil
}

func (r *PostgresRepository) DeleteVariant(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteProductVariant(ctx, id)
}

func toProductDetailFromGetRow(row db.GetProductByUUIDRow) *model.ProductDetail {
	var images []string
	if len(row.Images) > 0 {
		_ = json.Unmarshal(row.Images, &images)
	}
	var viewbox model.Model3DViewbox
	if len(row.Model3dViewbox) > 0 {
		_ = json.Unmarshal(row.Model3dViewbox, &viewbox)
	}

	var catID *int64
	if row.CategoryID.Valid {
		catID = &row.CategoryID.Int64
	}

	return &model.ProductDetail{
		ID:               row.ID,
		UUID:             row.Uuid.String(),
		CategoryID:       catID,
		CategoryName:     pgtypeToString(row.CategoryName),
		CategorySlug:     pgtypeToString(row.CategorySlug),
		Name:             row.Name,
		Slug:             row.Slug,
		ShortDescription: row.ShortDescription,
		Description:      row.Description,
		Status:           row.Status,
		ProductType:      row.ProductType,
		BasePrice:        numericToFloat(row.BasePrice),
		Currency:         row.Currency,
		TaxRate:          numericToFloat(row.TaxRate),
		StockQuantity:    int(row.StockQuantity),
		SKU:              row.Sku,
		AllowCustomPrint: row.AllowCustomPrint,
		Model3DFileURL:   row.Model3dFileUrl,
		Model3DUSDZURL:   row.Model3dUsdzUrl,
		Model3DViewbox:   viewbox,
		Images:           images,
		MetaTitle:        row.MetaTitle,
		MetaDescription:  row.MetaDescription,
		MetaKeywords:     row.MetaKeywords,
		CanonicalURL:     row.CanonicalUrl,
		OGImageURL:       row.OgImageUrl,
		DisplayOrder:     int(row.DisplayOrder),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

func toProductDetailFromSlugRow(row db.GetProductBySlugRow) *model.ProductDetail {
	var images []string
	if len(row.Images) > 0 {
		_ = json.Unmarshal(row.Images, &images)
	}
	var viewbox model.Model3DViewbox
	if len(row.Model3dViewbox) > 0 {
		_ = json.Unmarshal(row.Model3dViewbox, &viewbox)
	}

	var catID *int64
	if row.CategoryID.Valid {
		catID = &row.CategoryID.Int64
	}

	return &model.ProductDetail{
		ID:               row.ID,
		UUID:             row.Uuid.String(),
		CategoryID:       catID,
		CategoryName:     pgtypeToString(row.CategoryName),
		CategorySlug:     pgtypeToString(row.CategorySlug),
		Name:             row.Name,
		Slug:             row.Slug,
		ShortDescription: row.ShortDescription,
		Description:      row.Description,
		Status:           row.Status,
		ProductType:      row.ProductType,
		BasePrice:        numericToFloat(row.BasePrice),
		Currency:         row.Currency,
		TaxRate:          numericToFloat(row.TaxRate),
		StockQuantity:    int(row.StockQuantity),
		SKU:              row.Sku,
		AllowCustomPrint: row.AllowCustomPrint,
		Model3DFileURL:   row.Model3dFileUrl,
		Model3DUSDZURL:   row.Model3dUsdzUrl,
		Model3DViewbox:   viewbox,
		Images:           images,
		MetaTitle:        row.MetaTitle,
		MetaDescription:  row.MetaDescription,
		MetaKeywords:     row.MetaKeywords,
		CanonicalURL:     row.CanonicalUrl,
		OGImageURL:       row.OgImageUrl,
		DisplayOrder:     int(row.DisplayOrder),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

func toProductDetailFromProduct(row db.Product) *model.ProductDetail {
	var images []string
	if len(row.Images) > 0 {
		_ = json.Unmarshal(row.Images, &images)
	}
	var viewbox model.Model3DViewbox
	if len(row.Model3dViewbox) > 0 {
		_ = json.Unmarshal(row.Model3dViewbox, &viewbox)
	}

	var catID *int64
	if row.CategoryID.Valid {
		catID = &row.CategoryID.Int64
	}

	return &model.ProductDetail{
		ID:               row.ID,
		UUID:             row.Uuid.String(),
		CategoryID:       catID,
		Name:             row.Name,
		Slug:             row.Slug,
		ShortDescription: row.ShortDescription,
		Description:      row.Description,
		Status:           row.Status,
		ProductType:      row.ProductType,
		BasePrice:        numericToFloat(row.BasePrice),
		Currency:         row.Currency,
		TaxRate:          numericToFloat(row.TaxRate),
		StockQuantity:    int(row.StockQuantity),
		SKU:              row.Sku,
		AllowCustomPrint: row.AllowCustomPrint,
		Model3DFileURL:   row.Model3dFileUrl,
		Model3DUSDZURL:   row.Model3dUsdzUrl,
		Model3DViewbox:   viewbox,
		Images:           images,
		MetaTitle:        row.MetaTitle,
		MetaDescription:  row.MetaDescription,
		MetaKeywords:     row.MetaKeywords,
		CanonicalURL:     row.CanonicalUrl,
		OGImageURL:       row.OgImageUrl,
		DisplayOrder:     int(row.DisplayOrder),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

func textToPgtype(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func int64ToPgtype(i *int64) pgtype.Int8 {
	if i == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *i, Valid: true}
}

func pgtypeToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0.0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0.0
	}
	return f.Float64
}

func floatToNumeric(f float64) pgtype.Numeric {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return pgtype.Numeric{Valid: false}
	}
	var num pgtype.Numeric
	_ = num.Scan(strconv.FormatFloat(f, 'f', -1, 64))
	return num
}
