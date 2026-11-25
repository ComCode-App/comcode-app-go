package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	categoriesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/model"
	productsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	productsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const ResourceProducts = "platform.products"

// productsExportCap bounds one export; the platform list is paginated, the file is not.
const productsExportCap = 10000

const entityProduct = "product"

// ProductsAdapter exports and imports platform catalog products (TEC-596).
//
// Export reads the list query of GET /v1/platform/products with its filters (q over name, slug
// and SKU; status; product_type), in list order (display order, then newest).
//
// Import matches rows by slug, the product's natural key: active slugs are unique (partial
// unique index uq_products_slug_active), SKU is optional and not unique. A row whose slug, as
// the product form normalizes it, belongs to a live product updates it; otherwise it creates
// one. On update, empty cells keep the current value. Every row is checked with
// productsusecase.ValidateProductInput (the product form's rules) and written through the
// products use case (Create/Update), so slugs, categories and the search index behave as in the
// editor. Categories are matched by slug or UUID among live platform categories. Variants,
// images, 3D models and the SEO image/canonical URL are not imported: an update leaves them
// as they are. Rollback deletes created products and restores updated ones field by field,
// including their category.
type ProductsAdapter struct {
	q    *db.Queries
	svc  *productsusecase.Service
	cats categoriesrepo.Repository
}

// NewProducts builds the adapter over its own products use case (no object storage: imports
// never upload media).
func NewProducts(q *db.Queries) *ProductsAdapter {
	cats := categoriesrepo.NewPostgres(q)
	return &ProductsAdapter{
		q:    q,
		svc:  productsusecase.NewService(productsrepo.NewPostgres(q), cats, nil),
		cats: cats,
	}
}

// SetSearchIndexer keeps imported products in the Cmd+K index, as the editor does.
func (a *ProductsAdapter) SetSearchIndexer(e searchengine.Enqueuer) { a.svc.SetSearchIndexer(e) }

var (
	_ ioengine.RowPreviewer   = (*ProductsAdapter)(nil)
	_ ioengine.SampleProvider = (*ProductsAdapter)(nil)
)

func (a *ProductsAdapter) Resource() string { return ResourceProducts }

// productFields are the exported and importable columns, in file order.
var productFields = []struct {
	key   string
	typ   ioengine.ColumnType
	label string
}{
	{"slug", ioengine.ColumnTypeString, "products.fields.slug"},
	{"name", ioengine.ColumnTypeString, "products.fields.name"},
	{"sku", ioengine.ColumnTypeString, "products.fields.sku"},
	{"status", ioengine.ColumnTypeEnum, "products.fields.status"},
	{"product_type", ioengine.ColumnTypeEnum, "products.fields.type"},
	{"category", ioengine.ColumnTypeString, "products.fields.category"},
	{"base_price", ioengine.ColumnTypeString, "products.fields.base_price"},
	{"currency", ioengine.ColumnTypeString, "products.fields.currency"},
	{"tax_rate", ioengine.ColumnTypeString, "products.fields.tax_rate"},
	{"stock_quantity", ioengine.ColumnTypeString, "products.fields.stock"},
	{"allow_custom_print", ioengine.ColumnTypeBoolean, "products.fields.allow_custom_print"},
	{"display_order", ioengine.ColumnTypeString, "products.fields.display_order"},
	{"short_description", ioengine.ColumnTypeString, "products.fields.short_description"},
	{"description", ioengine.ColumnTypeString, "products.fields.description"},
	{"meta_title", ioengine.ColumnTypeString, "products.fields.meta_title"},
	{"meta_description", ioengine.ColumnTypeString, "products.fields.meta_description"},
	{"meta_keywords", ioengine.ColumnTypeString, "products.fields.meta_keywords"},
}

func (a *ProductsAdapter) ExportColumns() []ioengine.Column {
	cols := make([]ioengine.Column, 0, len(productFields)+3)
	for _, f := range productFields {
		cols = append(cols, ioengine.Column{Key: f.key, LabelKey: f.label, Type: f.typ})
	}
	return append(cols,
		ioengine.Column{Key: "created_at", LabelKey: "products.fields.created_at", Type: ioengine.ColumnTypeDatetime},
		ioengine.Column{Key: "updated_at", LabelKey: "products.fields.updated_at", Type: ioengine.ColumnTypeDatetime},
		ioengine.Column{Key: "uuid", LabelKey: "products.fields.uuid", Type: ioengine.ColumnTypeUUID},
	)
}

func (a *ProductsAdapter) Export(ctx context.Context, query ioengine.ExportQuery, _ i18n.Locale) (ioengine.Dataset, error) {
	rows, err := a.q.ListProducts(ctx, db.ListProductsParams{
		Q:           textArg(query["q"]),
		Status:      textArg(query["status"]),
		ProductType: textArg(query["product_type"]),
		LimitCount:  productsExportCap,
	})
	if err != nil {
		return ioengine.Dataset{}, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"slug": r.Slug, "name": r.Name, "sku": r.Sku, "status": r.Status,
			"product_type": r.ProductType, "category": r.CategorySlug.String,
			"base_price": moneyString(r.BasePrice), "currency": r.Currency,
			"tax_rate": moneyString(r.TaxRate), "stock_quantity": strconv.Itoa(int(r.StockQuantity)),
			"allow_custom_print": r.AllowCustomPrint, "display_order": strconv.Itoa(int(r.DisplayOrder)),
			"short_description": r.ShortDescription, "description": r.Description,
			"meta_title": r.MetaTitle, "meta_description": r.MetaDescription, "meta_keywords": r.MetaKeywords,
			"created_at": r.CreatedAt.Time, "updated_at": r.UpdatedAt.Time, "uuid": r.Uuid.String(),
		})
	}
	return ioengine.Dataset{Resource: ResourceProducts, Columns: a.ExportColumns(), Rows: out}, nil
}

func (a *ProductsAdapter) ImportSchema() []ioengine.ImportField {
	required := map[string]bool{"slug": true, "name": true}
	hints := map[string]string{
		"status": "draft", "product_type": "physical_card", "currency": "TRY", "tax_rate": "20",
		"stock_quantity": "0", "allow_custom_print": "false", "display_order": "0",
	}
	out := make([]ioengine.ImportField, 0, len(productFields))
	for _, f := range productFields {
		out = append(out, ioengine.ImportField{
			Key: f.key, LabelKey: f.label, Type: f.typ, Required: required[f.key], DefaultHint: hints[f.key],
		})
	}
	return out
}

func (a *ProductsAdapter) SampleRows() []map[string]any {
	return []map[string]any{
		{
			"slug": "comcode-kart-pro", "name": "ComCode Kart Pro", "sku": "CC-PRO-01", "status": "draft",
			"product_type": "physical_card", "category": "", "base_price": "499.90", "currency": "TRY",
			"tax_rate": "20", "stock_quantity": "100", "allow_custom_print": "true", "display_order": "10",
			"short_description": "NFC ve QR destekli dijital kartvizit", "description": "",
			"meta_title": "", "meta_description": "", "meta_keywords": "",
		},
	}
}

// productPlan is one validated row: the input Create/Update receives and, for an update, the
// product as it was.
type productPlan struct {
	input    model.UpsertProductInput
	existing *model.ProductDetail
}

func (a *ProductsAdapter) plan(ctx context.Context, row map[string]any) (productPlan, []ioengine.RowError) {
	var errs []ioengine.RowError
	bad := func(field, msg string) { errs = append(errs, ioengine.RowError{Field: field, Error: msg}) }

	rawSlug := strVal(row, "slug")
	if rawSlug == "" {
		bad("slug", "required")
		return productPlan{}, errs
	}
	slug := productsusecase.NormalizeSlug(rawSlug)
	p := productPlan{}
	existing, err := a.svc.GetBySlug(ctx, slug)
	switch {
	case err == nil:
		p.existing = existing
		p.input = inputFromDetail(existing)
	case errors.Is(err, productsusecase.ErrNotFound):
		p.input = model.UpsertProductInput{
			Status: "draft", ProductType: "physical_card", Currency: "TRY", TaxRate: ptrFloat(model.DefaultTaxRate),
		}
		if strVal(row, "base_price") == "" {
			bad("base_price", "required")
		}
	default:
		bad("slug", err.Error())
		return p, errs
	}
	in := &p.input
	in.Slug = slug

	text := func(key string, dst *string) {
		if v := strVal(row, key); v != "" {
			*dst = v
		}
	}
	text("name", &in.Name)
	text("sku", &in.SKU)
	text("product_type", &in.ProductType)
	text("short_description", &in.ShortDescription)
	text("description", &in.Description)
	text("meta_title", &in.MetaTitle)
	text("meta_description", &in.MetaDescription)
	text("meta_keywords", &in.MetaKeywords)
	if v := strVal(row, "status"); v != "" {
		in.Status = strings.ToLower(v)
	}
	if v := strVal(row, "currency"); v != "" {
		in.Currency = strings.ToUpper(v)
	}
	number := func(key string, dst *float64) {
		v := strVal(row, key)
		if v == "" {
			return
		}
		f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
		if err != nil {
			bad(key, "invalid_number")
			return
		}
		*dst = f
	}
	number("base_price", &in.BasePrice)
	// An explicit 0 % VAT is kept (TEC-1301); an empty cell keeps the default or current VAT.
	if in.TaxRate != nil {
		tax := *in.TaxRate
		number("tax_rate", &tax)
		in.TaxRate = &tax
	}
	integer := func(key string, dst *int) {
		v := strVal(row, key)
		if v == "" {
			return
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			bad(key, "invalid_integer")
			return
		}
		*dst = n
	}
	integer("stock_quantity", &in.StockQuantity)
	integer("display_order", &in.DisplayOrder)
	if v := strVal(row, "allow_custom_print"); v != "" {
		b, ok := parseBool(v)
		if !ok {
			bad("allow_custom_print", "invalid_boolean")
		}
		in.AllowCustomPrint = b
	}
	if v := strVal(row, "category"); v != "" {
		catUUID, err := a.resolveCategory(ctx, v)
		if err != nil {
			bad("category", err.Error())
		} else {
			in.CategoryUUID = &catUUID
		}
	}
	for _, fe := range productsusecase.ValidateProductInput(*in) {
		bad(fe.Field, fe.Code)
	}
	return p, errs
}

// resolveCategory finds a live platform category by UUID or slug.
func (a *ProductsAdapter) resolveCategory(ctx context.Context, v string) (string, error) {
	if id, err := uuid.Parse(v); err == nil {
		if c, err := a.cats.GetByUUID(ctx, id); err == nil && c != nil {
			return c.UUID, nil
		}
		return "", errors.New("category_not_found")
	}
	if c, err := a.cats.GetBySlug(ctx, strings.ToLower(v)); err == nil && c != nil {
		return c.UUID, nil
	}
	return "", errors.New("category_not_found")
}

func parseBool(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "evet", "y", "e":
		return true, true
	case "false", "0", "no", "hayır", "hayir", "n", "h":
		return false, true
	}
	return false, false
}

// inputFromDetail is the full input that rewrites a product as it is, media included.
func inputFromDetail(d *model.ProductDetail) model.UpsertProductInput {
	in := model.UpsertProductInput{
		Name: d.Name, Slug: d.Slug, ShortDescription: d.ShortDescription, Description: d.Description,
		Status: d.Status, ProductType: d.ProductType, BasePrice: d.BasePrice, Currency: d.Currency,
		TaxRate: ptrFloat(d.TaxRate), StockQuantity: d.StockQuantity, SKU: d.SKU, AllowCustomPrint: d.AllowCustomPrint,
		Model3DFileURL: d.Model3DFileURL, Model3DUSDZURL: d.Model3DUSDZURL, Images: d.Images,
		MetaTitle: d.MetaTitle, MetaDescription: d.MetaDescription, MetaKeywords: d.MetaKeywords,
		CanonicalURL: d.CanonicalURL, OGImageURL: d.OGImageURL, DisplayOrder: d.DisplayOrder,
	}
	vb := d.Model3DViewbox
	in.Model3DViewbox = &vb
	if in.Images == nil {
		in.Images = []string{}
	}
	// No category: Update keeps the current one (the row's category cell, when set, replaces it).
	return in
}

func (a *ProductsAdapter) PreviewRow(ctx context.Context, row map[string]any, _ map[string]any) (string, []ioengine.RowError) {
	p, errs := a.plan(ctx, row)
	if len(errs) > 0 {
		return "", errs
	}
	if p.existing != nil {
		return "update", nil
	}
	return "create", nil
}

// productSnapshot is the before-state of an updated product, enough to rewrite it exactly.
// CategoryID is the stored category (nil: none); Update alone cannot clear a category.
type productSnapshot struct {
	Input      model.UpsertProductInput `json:"input"`
	CategoryID *int64                   `json:"category_id"`
}

func (a *ProductsAdapter) ApplyRow(ctx context.Context, row map[string]any, _ map[string]any) (ioengine.RowResult, error) {
	p, errs := a.plan(ctx, row)
	if len(errs) > 0 {
		return ioengine.RowResult{OK: false, Error: joinRowErrors(errs)}, nil
	}
	if p.existing == nil {
		created, err := a.svc.Create(ctx, p.input)
		if err != nil {
			return ioengine.RowResult{OK: false, Error: err.Error()}, nil
		}
		return ioengine.RowResult{OK: true, Op: "create", EntityType: entityProduct, EntityUUID: created.UUID}, nil
	}
	snap := productSnapshot{Input: inputFromDetail(p.existing), CategoryID: p.existing.CategoryID}
	prev, err := toMap(snap)
	if err != nil {
		return ioengine.RowResult{}, err
	}
	id, err := uuid.Parse(p.existing.UUID)
	if err != nil {
		return ioengine.RowResult{}, err
	}
	if _, err := a.svc.Update(ctx, id, p.input); err != nil {
		return ioengine.RowResult{OK: false, Error: err.Error()}, nil
	}
	return ioengine.RowResult{OK: true, Op: "update", EntityType: entityProduct, EntityUUID: p.existing.UUID, Previous: prev}, nil
}

// RevertRow deletes a product the import created (no previous state) and restores one it
// updated.
func (a *ProductsAdapter) RevertRow(ctx context.Context, entityType, entityUUID string, previous map[string]any) error {
	if entityType != entityProduct {
		return fmt.Errorf("unexpected entity %q", entityType)
	}
	id, err := uuid.Parse(entityUUID)
	if err != nil {
		return err
	}
	if len(previous) == 0 {
		return a.svc.Delete(ctx, id)
	}
	b, err := json.Marshal(previous)
	if err != nil {
		return err
	}
	var snap productSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return err
	}
	if _, err := a.svc.Update(ctx, id, snap.Input); err != nil {
		return err
	}
	cat := pgtype.Int8{}
	if snap.CategoryID != nil {
		cat = pgtype.Int8{Int64: *snap.CategoryID, Valid: true}
	}
	return a.q.SetProductCategory(ctx, db.SetProductCategoryParams{Uuid: id, CategoryID: cat})
}

func toMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(b, &m)
}

func joinRowErrors(errs []ioengine.RowError) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		if e.Field != "" {
			parts = append(parts, e.Field+": "+e.Error)
		} else {
			parts = append(parts, e.Error)
		}
	}
	return strings.Join(parts, "; ")
}

func ptrFloat(f float64) *float64 { return &f }
