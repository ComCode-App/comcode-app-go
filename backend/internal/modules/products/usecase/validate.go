package usecase

import (
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/model"
)

// ProductStatuses are the statuses the platform product form offers.
var ProductStatuses = []string{"active", "draft", "archived"}

// FieldError is one invalid field of a product input: Field is the input key, Code a stable
// machine-readable reason (required, too_long, invalid_status, negative, too_many_decimals,
// out_of_range).
type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// ValidateProductInput checks a product against the rules of the platform product form
// (frontend/src/features/products/schemas/product-form.ts) and the column sizes: name 1-200
// characters, slug at most 150, short description at most 300, status active|draft|archived,
// base price >= 0 with at most two decimals, tax rate 0-100 with at most two decimals, stock a
// whole number >= 0, SKU at most 64, currency 1-8, product type 1-32, meta title and keywords
// at most 255. The product import (TEC-596) runs every row through it before Create/Update.
//
// Create and Update of the REST API run it too (TEC-1299, see validateUpsert), so the form,
// the import and the API share one set of rules.
func ValidateProductInput(in model.UpsertProductInput) []FieldError {
	var errs []FieldError
	bad := func(field, code string) { errs = append(errs, FieldError{Field: field, Code: code}) }
	longer := func(s string, n int) bool { return utf8.RuneCountInString(s) > n }

	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		bad("name", "required")
	case longer(name, 200):
		bad("name", "too_long")
	}
	if longer(in.Slug, 150) {
		bad("slug", "too_long")
	}
	if longer(in.ShortDescription, 300) {
		bad("short_description", "too_long")
	}
	if !slices.Contains(ProductStatuses, in.Status) {
		bad("status", "invalid_status")
	}
	if pt := strings.TrimSpace(in.ProductType); pt == "" {
		bad("product_type", "required")
	} else if longer(pt, 32) {
		bad("product_type", "too_long")
	}
	switch {
	case math.IsNaN(in.BasePrice) || math.IsInf(in.BasePrice, 0):
		bad("base_price", "invalid_number")
	case in.BasePrice < 0:
		bad("base_price", "negative")
	case !twoDecimals(in.BasePrice):
		bad("base_price", "too_many_decimals")
	}
	switch {
	case math.IsNaN(in.TaxRate) || math.IsInf(in.TaxRate, 0):
		bad("tax_rate", "invalid_number")
	case in.TaxRate < 0 || in.TaxRate > 100:
		bad("tax_rate", "out_of_range")
	case !twoDecimals(in.TaxRate):
		bad("tax_rate", "too_many_decimals")
	}
	if in.StockQuantity < 0 {
		bad("stock_quantity", "negative")
	}
	if cur := strings.TrimSpace(in.Currency); cur == "" {
		bad("currency", "required")
	} else if longer(cur, 8) {
		bad("currency", "too_long")
	}
	if longer(in.SKU, 64) {
		bad("sku", "too_long")
	}
	if longer(in.MetaTitle, 255) {
		bad("meta_title", "too_long")
	}
	if longer(in.MetaKeywords, 255) {
		bad("meta_keywords", "too_long")
	}
	return errs
}

// ValidationError is a product input that breaks the rules of ValidateProductInput. The handler
// writes it as 400 VALIDATION_ERROR with one detail per field.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Code
	}
	return "invalid product: " + strings.Join(parts, ", ")
}

// validateUpsert checks what a Create or Update request sends. Status, product type and
// currency are optional in the API (an empty value means the default on create and "keep" on
// update), so an empty one is not checked; every other field is written as sent and checked
// as is.
func validateUpsert(in model.UpsertProductInput) error {
	if in.Status == "" {
		in.Status = ProductStatuses[0]
	}
	if in.ProductType == "" {
		in.ProductType = "physical_card"
	}
	if in.Currency == "" {
		in.Currency = "TRY"
	}
	if errs := ValidateProductInput(in); len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}
	return nil
}

// twoDecimals is the form's check: at most two decimal places, as NUMERIC(…, 2) stores.
func twoDecimals(v float64) bool {
	return math.Abs(math.Round(v*100)-v*100) < 1e-6
}

// NormalizeSlug is the slug Create and Update store for a given slug: lower case, Turkish
// letters folded, every other run of characters a single hyphen.
func NormalizeSlug(slug string) string { return sanitizeSlug(slug) }
