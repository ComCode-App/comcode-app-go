package handler_test

// Platform product create/update (TEC-1299) against Postgres: the product form's rules return
// 400 VALIDATION_ERROR with the field, valid writes succeed, and detail, list and write
// responses carry the category's uuid.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	categoriesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type errEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Details []struct {
			Field string `json:"field"`
			Code  string `json:"code"`
		} `json:"details"`
	} `json:"error"`
}

type product struct {
	UUID         string  `json:"uuid"`
	CategoryUUID string  `json:"category_uuid"`
	CategorySlug string  `json:"category_slug"`
	BasePrice    float64 `json:"base_price"`
	TaxRate      float64 `json:"tax_rate"`
	Status       string  `json:"status"`
}

func do(t *testing.T, h http.HandlerFunc, method, path, id string, body any) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	r := httptest.NewRequest(method, path, &buf)
	if id != "" {
		r.SetPathValue("uuid", id)
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w.Code, w.Body.Bytes()
}

func data[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var env struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &env), string(raw))
	return env.Data
}

func TestIntegrationProductValidationAndCategory(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	h := handler.New(usecase.NewService(repository.NewPostgres(d.Queries), categoriesrepo.NewPostgres(d.Queries), nil))

	tag := pgtest.Unique()
	var catUUID uuid.UUID
	require.NoError(t, d.Pool.QueryRow(ctx, `INSERT INTO product_categories (name, slug) VALUES ($1, $2) RETURNING uuid`,
		"Kategori "+tag, "kat-"+tag).Scan(&catUUID))
	cat := catUUID.String()

	valid := func(extra map[string]any) map[string]any {
		body := map[string]any{"name": "Kart " + tag, "base_price": 49.9, "stock_quantity": 3, "category_uuid": cat}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}
	expectField := func(code int, raw []byte, field, rule string) {
		t.Helper()
		require.Equal(t, http.StatusBadRequest, code, string(raw))
		var env errEnvelope
		require.NoError(t, json.Unmarshal(raw, &env))
		require.Equal(t, "VALIDATION_ERROR", env.Error.Code)
		require.Len(t, env.Error.Details, 1, string(raw))
		require.Equal(t, field, env.Error.Details[0].Field)
		require.Equal(t, rule, env.Error.Details[0].Code)
	}
	bad := []struct {
		name, field, rule string
		body              map[string]any
	}{
		{"negative price", "base_price", "negative", valid(map[string]any{"base_price": -1})},
		{"three decimals", "base_price", "too_many_decimals", valid(map[string]any{"base_price": 10.555})},
		{"tax 150", "tax_rate", "out_of_range", valid(map[string]any{"tax_rate": 150})},
		{"tax three decimals", "tax_rate", "too_many_decimals", valid(map[string]any{"tax_rate": 18.005})},
		{"negative stock", "stock_quantity", "negative", valid(map[string]any{"stock_quantity": -2})},
		{"unknown status", "status", "invalid_status", valid(map[string]any{"status": "published"})},
	}

	// --- Create: each rule is a 400 with its field; nothing is stored.
	for _, c := range bad {
		code, raw := do(t, h.Create, "POST", "/v1/platform/products", "", c.body)
		expectField(code, raw, c.field, c.rule)
	}
	var n int
	require.NoError(t, d.Pool.QueryRow(ctx, `SELECT count(*) FROM products WHERE name = $1`, "Kart "+tag).Scan(&n))
	require.Zero(t, n, "an invalid product was stored")
	// A stock that is not a whole number cannot even be decoded.
	code, _ := do(t, h.Create, "POST", "/v1/platform/products", "", valid(map[string]any{"stock_quantity": 1.5}))
	require.Equal(t, http.StatusBadRequest, code)

	// --- Valid create: 201, defaults for omitted status, the category's uuid in the response.
	code, raw := do(t, h.Create, "POST", "/v1/platform/products", "", valid(nil))
	require.Equal(t, http.StatusCreated, code, string(raw))
	created := data[product](t, raw)
	require.Equal(t, "draft", created.Status)
	require.Equal(t, cat, created.CategoryUUID)
	require.Equal(t, "kat-"+tag, created.CategorySlug)

	// --- Update: the same rules, then a valid update that keeps the status it does not send.
	for _, c := range bad {
		code, raw := do(t, h.Update, "PUT", "/v1/platform/products/"+created.UUID, created.UUID, c.body)
		expectField(code, raw, c.field, c.rule)
	}
	code, raw = do(t, h.Update, "PUT", "/v1/platform/products/"+created.UUID, created.UUID,
		valid(map[string]any{"base_price": 0, "tax_rate": 100, "status": "active"}))
	require.Equal(t, http.StatusOK, code, string(raw))
	updated := data[product](t, raw)
	require.Equal(t, "active", updated.Status)
	require.Equal(t, 100.0, updated.TaxRate)
	require.Equal(t, cat, updated.CategoryUUID)
	code, raw = do(t, h.Update, "PUT", "/v1/platform/products/"+created.UUID, created.UUID, valid(map[string]any{"base_price": 12.5}))
	require.Equal(t, http.StatusOK, code, string(raw))
	require.Equal(t, "active", data[product](t, raw).Status, "an omitted status is kept")

	// --- Detail (by uuid and by slug) and list carry category_uuid.
	code, raw = do(t, h.Get, "GET", "/v1/platform/products/"+created.UUID, created.UUID, nil)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, cat, data[product](t, raw).CategoryUUID)

	var slug string
	require.NoError(t, d.Pool.QueryRow(ctx, `SELECT slug FROM products WHERE uuid = $1`, created.UUID).Scan(&slug))
	r := httptest.NewRequest("GET", "/v1/products/by-slug/"+slug, nil)
	r.SetPathValue("slug", slug)
	w := httptest.NewRecorder()
	h.GetBySlug(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, cat, data[product](t, w.Body.Bytes()).CategoryUUID)

	code, raw = do(t, h.List, "GET", "/v1/platform/products?q="+tag, "", nil)
	require.Equal(t, http.StatusOK, code)
	page := data[struct {
		Items []product `json:"items"`
	}](t, raw)
	require.Len(t, page.Items, 1)
	require.Equal(t, cat, page.Items[0].CategoryUUID)
}
