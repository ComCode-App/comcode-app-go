package usecase

import (
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/model"
)

func TestValidateProductInputMatchesTheForm(t *testing.T) {
	base := func() model.UpsertProductInput {
		return model.UpsertProductInput{
			Name: "Kart", Status: "draft", ProductType: "physical_card", Currency: "TRY",
			BasePrice: 0, TaxRate: tax(0), StockQuantity: 0,
		}
	}
	if errs := ValidateProductInput(base()); len(errs) != 0 {
		t.Fatalf("valid input rejected: %v", errs)
	}
	edge := base()
	edge.BasePrice, edge.TaxRate, edge.StockQuantity = 499.99, tax(100), 3
	if errs := ValidateProductInput(edge); len(errs) != 0 {
		t.Fatalf("edge values rejected: %v", errs)
	}
	cases := []struct {
		field, code string
		change      func(*model.UpsertProductInput)
	}{
		{"name", "required", func(in *model.UpsertProductInput) { in.Name = "  " }},
		{"name", "too_long", func(in *model.UpsertProductInput) { in.Name = strings.Repeat("ş", 201) }},
		{"slug", "too_long", func(in *model.UpsertProductInput) { in.Slug = strings.Repeat("a", 151) }},
		{"short_description", "too_long", func(in *model.UpsertProductInput) { in.ShortDescription = strings.Repeat("a", 301) }},
		{"status", "invalid_status", func(in *model.UpsertProductInput) { in.Status = "" }},
		{"status", "invalid_status", func(in *model.UpsertProductInput) { in.Status = "published" }},
		{"product_type", "required", func(in *model.UpsertProductInput) { in.ProductType = "" }},
		{"base_price", "negative", func(in *model.UpsertProductInput) { in.BasePrice = -0.01 }},
		{"base_price", "too_many_decimals", func(in *model.UpsertProductInput) { in.BasePrice = 10.001 }},
		{"tax_rate", "out_of_range", func(in *model.UpsertProductInput) { in.TaxRate = tax(100.5) }},
		{"tax_rate", "out_of_range", func(in *model.UpsertProductInput) { in.TaxRate = tax(-1) }},
		{"tax_rate", "too_many_decimals", func(in *model.UpsertProductInput) { in.TaxRate = tax(18.005) }},
		{"stock_quantity", "negative", func(in *model.UpsertProductInput) { in.StockQuantity = -1 }},
		{"currency", "required", func(in *model.UpsertProductInput) { in.Currency = "" }},
		{"sku", "too_long", func(in *model.UpsertProductInput) { in.SKU = strings.Repeat("a", 65) }},
		{"meta_title", "too_long", func(in *model.UpsertProductInput) { in.MetaTitle = strings.Repeat("a", 256) }},
		{"meta_keywords", "too_long", func(in *model.UpsertProductInput) { in.MetaKeywords = strings.Repeat("a", 256) }},
	}
	for _, c := range cases {
		in := base()
		c.change(&in)
		errs := ValidateProductInput(in)
		if len(errs) != 1 || errs[0].Field != c.field || errs[0].Code != c.code {
			t.Errorf("%s/%s: got %v", c.field, c.code, errs)
		}
	}
	// An omitted VAT is not checked: the default applies on create, the stored value on update.
	omitted := base()
	omitted.TaxRate = nil
	if errs := ValidateProductInput(omitted); len(errs) != 0 {
		t.Fatalf("omitted tax_rate rejected: %v", errs)
	}
	if NormalizeSlug("ComCode Kart Ş") != "comcode-kart-s" {
		t.Fatal(NormalizeSlug("ComCode Kart Ş"))
	}
}

func tax(v float64) *float64 { return &v }
