package adapters_test

// Product catalog export and import (TEC-596) against Postgres: export filters, the dry-run
// preview with the product form's rules, apply (create and update by slug) and rollback.

import (
	"context"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

type productRow struct {
	Name, Status, SKU, Currency, ShortDescription string
	BasePrice, TaxRate                            string
	Stock, DisplayOrder                           int
	CategoryID                                    *int64
	Images                                        string
	Deleted                                       bool
}

func loadProduct(t *testing.T, d *pgtest.DB, slug string) (productRow, uuid.UUID, bool) {
	t.Helper()
	var p productRow
	var id uuid.UUID
	err := d.Pool.QueryRow(context.Background(), `
		SELECT uuid, name, status, sku, currency, short_description, base_price::text, tax_rate::text,
		       stock_quantity, display_order, category_id, images::text, deleted_at IS NOT NULL
		FROM products WHERE slug = $1 ORDER BY created_at DESC LIMIT 1`, slug,
	).Scan(&id, &p.Name, &p.Status, &p.SKU, &p.Currency, &p.ShortDescription, &p.BasePrice, &p.TaxRate,
		&p.Stock, &p.DisplayOrder, &p.CategoryID, &p.Images, &p.Deleted)
	if err != nil {
		return p, id, false
	}
	return p, id, true
}

func TestIntegrationProductsImportExport(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	a := adapters.NewProducts(d.Queries)
	tag := pgtest.Unique()

	var catID int64
	var catUUID uuid.UUID
	catSlug := "kat-" + tag
	if err := d.Pool.QueryRow(ctx, `INSERT INTO product_categories (name, slug) VALUES ($1, $2) RETURNING id, uuid`,
		"Kategori "+tag, catSlug).Scan(&catID, &catUUID); err != nil {
		t.Fatal(err)
	}
	// An existing product with media the import must leave alone.
	existingSlug := "mevcut-" + tag
	if _, err := d.Pool.Exec(ctx, `
		INSERT INTO products (name, slug, status, base_price, tax_rate, stock_quantity, sku, short_description, images)
		VALUES ($1, $2, 'active', 100, 20, 5, $3, 'eski', '["https://cdn.example/a.png"]')`,
		"Mevcut "+tag, existingSlug, "SKU-"+tag); err != nil {
		t.Fatal(err)
	}

	// --- Dry run: ops and every rule of the product form.
	ok := func(row map[string]any, wantOp string) {
		t.Helper()
		op, errs := a.PreviewRow(ctx, row, nil)
		if len(errs) > 0 || op != wantOp {
			t.Fatalf("preview %v: op=%q errs=%v, want %s", row, op, errs, wantOp)
		}
	}
	fails := func(row map[string]any, field, code string) {
		t.Helper()
		_, errs := a.PreviewRow(ctx, row, nil)
		for _, e := range errs {
			if e.Field == field && e.Error == code {
				return
			}
		}
		t.Fatalf("preview %v: want %s %s, got %v", row, field, code, errs)
	}
	newSlug := "yeni-" + tag
	valid := func(extra map[string]any) map[string]any {
		row := map[string]any{"slug": newSlug, "name": "Yeni Ürün", "base_price": "49.90"}
		for k, v := range extra {
			row[k] = v
		}
		return row
	}
	ok(valid(nil), "create")
	ok(map[string]any{"slug": strings.ToUpper(existingSlug), "name": ""}, "update") // slug is normalized; empty name keeps the current one
	fails(map[string]any{"name": "X", "base_price": "1"}, "slug", "required")
	fails(map[string]any{"slug": newSlug, "name": "Fiyatsız"}, "base_price", "required")
	fails(valid(map[string]any{"name": ""}), "name", "required")
	fails(valid(map[string]any{"base_price": "-1"}), "base_price", "negative")
	fails(valid(map[string]any{"base_price": "1.005"}), "base_price", "too_many_decimals")
	fails(valid(map[string]any{"base_price": "abc"}), "base_price", "invalid_number")
	fails(valid(map[string]any{"tax_rate": "100.01"}), "tax_rate", "out_of_range")
	fails(valid(map[string]any{"tax_rate": "-1"}), "tax_rate", "out_of_range")
	fails(valid(map[string]any{"tax_rate": "18.123"}), "tax_rate", "too_many_decimals")
	fails(valid(map[string]any{"stock_quantity": "-3"}), "stock_quantity", "negative")
	fails(valid(map[string]any{"stock_quantity": "2.5"}), "stock_quantity", "invalid_integer")
	fails(valid(map[string]any{"status": "published"}), "status", "invalid_status")
	fails(valid(map[string]any{"category": "yok-" + tag}), "category", "category_not_found")
	fails(valid(map[string]any{"category": uuid.NewString()}), "category", "category_not_found")
	fails(valid(map[string]any{"allow_custom_print": "belki"}), "allow_custom_print", "invalid_boolean")
	fails(valid(map[string]any{"sku": strings.Repeat("x", 65)}), "sku", "too_long")
	ok(valid(map[string]any{"base_price": "0", "tax_rate": "100", "stock_quantity": "0", "category": catSlug}), "create")
	ok(valid(map[string]any{"category": catUUID.String(), "base_price": "12,50"}), "create")
	// The dry run writes nothing.
	if _, _, found := loadProduct(t, d, newSlug); found {
		t.Fatal("preview created a product")
	}

	// --- Apply: a create (category by slug) and an update (empty cells keep values).
	created, err := a.ApplyRow(ctx, valid(map[string]any{
		"status": "active", "tax_rate": "10", "stock_quantity": "7", "category": catSlug, "allow_custom_print": "evet",
	}), nil)
	if err != nil || !created.OK || created.Op != "create" || created.EntityType != "product" || created.Previous != nil {
		t.Fatalf("create: %+v %v", created, err)
	}
	np, nid, found := loadProduct(t, d, newSlug)
	if !found || nid.String() != created.EntityUUID || np.BasePrice != "49.90" || np.TaxRate != "10.00" ||
		np.Stock != 7 || np.Status != "active" || np.CategoryID == nil || *np.CategoryID != catID {
		t.Fatalf("created product: %+v", np)
	}

	updated, err := a.ApplyRow(ctx, map[string]any{
		"slug": existingSlug, "name": "", "base_price": "120.5", "category": catSlug, "short_description": "",
	}, nil)
	if err != nil || !updated.OK || updated.Op != "update" || updated.Previous == nil {
		t.Fatalf("update: %+v %v", updated, err)
	}
	up, uid, _ := loadProduct(t, d, existingSlug)
	if up.Name != "Mevcut "+tag || up.BasePrice != "120.50" || up.ShortDescription != "eski" || up.Stock != 5 ||
		up.SKU != "SKU-"+tag || up.CategoryID == nil || *up.CategoryID != catID || !strings.Contains(up.Images, "a.png") {
		t.Fatalf("updated product: %+v", up)
	}

	// An invalid row is refused at apply time too, with the field in the message.
	bad, err := a.ApplyRow(ctx, valid(map[string]any{"slug": "kotu-" + tag, "tax_rate": "150"}), nil)
	if err != nil || bad.OK || !strings.Contains(bad.Error, "tax_rate") {
		t.Fatalf("invalid row applied: %+v %v", bad, err)
	}

	// --- Rollback: the update comes back exactly (no category again), the create is deleted.
	if err := a.RevertRow(ctx, updated.EntityType, updated.EntityUUID, updated.Previous); err != nil {
		t.Fatal(err)
	}
	rp, rid, _ := loadProduct(t, d, existingSlug)
	if rid != uid || rp.BasePrice != "100.00" || rp.CategoryID != nil || rp.ShortDescription != "eski" ||
		!strings.Contains(rp.Images, "a.png") || rp.Deleted {
		t.Fatalf("reverted update: %+v", rp)
	}
	if err := a.RevertRow(ctx, created.EntityType, created.EntityUUID, created.Previous); err != nil {
		t.Fatal(err)
	}
	if dp, _, _ := loadProduct(t, d, newSlug); !dp.Deleted {
		t.Fatal("reverted create is not deleted")
	}

	// --- Export: the list filters and the importable columns (round trip).
	export := func(q ioengine.ExportQuery) map[string]map[string]any {
		t.Helper()
		ds, err := a.Export(ctx, q, i18n.LocaleEN)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]map[string]any{}
		for _, r := range ds.Rows {
			out[r["slug"].(string)] = r
		}
		return out
	}
	if _, err := a.ApplyRow(ctx, map[string]any{"slug": "taslak-" + tag, "name": "Taslak " + tag, "base_price": "5", "category": catSlug}, nil); err != nil {
		t.Fatal(err)
	}
	all := export(ioengine.ExportQuery{"q": tag})
	if len(all) != 2 || all[existingSlug] == nil || all["taslak-"+tag] == nil {
		t.Fatalf("q filter (deleted excluded): %v", all)
	}
	if got := export(ioengine.ExportQuery{"q": tag, "status": "draft"}); len(got) != 1 || got["taslak-"+tag] == nil {
		t.Fatalf("status filter: %v", got)
	}
	if got := export(ioengine.ExportQuery{"q": tag, "product_type": "digital"}); len(got) != 0 {
		t.Fatalf("product_type filter: %v", got)
	}
	dr := all["taslak-"+tag]
	if dr["category"] != catSlug || dr["base_price"] != "5.00" || dr["tax_rate"] != "20.00" || dr["currency"] != "TRY" {
		t.Fatalf("export row: %+v", dr)
	}
	schema := map[string]bool{}
	for _, f := range a.ImportSchema() {
		schema[f.Key] = true
	}
	for _, c := range a.ExportColumns() {
		if _, has := dr[c.Key]; !has {
			t.Fatalf("row misses column %s", c.Key)
		}
		for _, loc := range []i18n.Locale{i18n.LocaleTR, i18n.LocaleEN} {
			if i18n.Translate(loc, c.LabelKey) == c.LabelKey {
				t.Fatalf("%s: no %s label", c.Key, loc)
			}
		}
		if c.Key != "uuid" && c.Key != "created_at" && c.Key != "updated_at" && !schema[c.Key] {
			t.Fatalf("exported column %s cannot be imported back", c.Key)
		}
	}
	if len(a.SampleRows()) == 0 {
		t.Fatal("no sample rows")
	}
	for _, s := range a.SampleRows() {
		if _, errs := a.PreviewRow(ctx, s, nil); len(errs) > 0 {
			t.Fatalf("sample row invalid: %v", errs)
		}
	}
}
