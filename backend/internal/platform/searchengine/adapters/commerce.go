package adapters

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Platform-wide commerce specs (TEC-595). Gated by platform.* permissions and linked to platform
// pages, so they never expose one account's data to another account's members.
const (
	SpecOrders    = "orders"
	SpecProducts  = "products"
	SpecCompanies = "companies"
)

func uuidArg(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("search: invalid uuid")
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

// single returns the only document of a one-uuid lookup.
func single(docs []searchengine.Document, spec string) (searchengine.Document, error) {
	if len(docs) == 0 {
		return searchengine.Document{}, fmt.Errorf("%s search: not found", spec)
	}
	return docs[0], nil
}

// OrdersAdapter indexes orders by number, customer and account.
type OrdersAdapter struct{ q *db.Queries }

func NewOrders(q *db.Queries) *OrdersAdapter { return &OrdersAdapter{q: q} }

func (a *OrdersAdapter) Spec() searchengine.Spec {
	return searchengine.Spec{
		ID: SpecOrders, LabelKey: "search.specs_orders", Permission: rbac.PermPlatformOrdersRead,
		Icon: "orders", Searchable: []string{"title", "subtitle", "keywords"},
	}
}

func (a *OrdersAdapter) ListAll(ctx context.Context) ([]searchengine.Document, error) {
	return a.list(ctx, pgtype.UUID{})
}

func (a *OrdersAdapter) Document(ctx context.Context, id string) (searchengine.Document, error) {
	arg, err := uuidArg(id)
	if err != nil {
		return searchengine.Document{}, err
	}
	docs, err := a.list(ctx, arg)
	if err != nil {
		return searchengine.Document{}, err
	}
	return single(docs, SpecOrders)
}

func (a *OrdersAdapter) list(ctx context.Context, id pgtype.UUID) ([]searchengine.Document, error) {
	rows, err := a.q.ListOrdersForSearch(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]searchengine.Document, 0, len(rows))
	for _, o := range rows {
		out = append(out, searchengine.Document{
			ID:       o.Uuid.String(),
			Spec:     SpecOrders,
			Title:    o.OrderNumber,
			Subtitle: strings.TrimSpace(o.AccountName + " · " + o.CustomerEmail),
			Keywords: []string{o.CustomerName, o.CustomerEmail, o.AccountName, o.Status},
			Href:     "/platform/orders?open=" + o.Uuid.String(),
			Icon:     "orders",
		})
	}
	return out, nil
}

// ProductsAdapter indexes catalog products by name, slug and SKU.
type ProductsAdapter struct{ q *db.Queries }

func NewProducts(q *db.Queries) *ProductsAdapter { return &ProductsAdapter{q: q} }

func (a *ProductsAdapter) Spec() searchengine.Spec {
	return searchengine.Spec{
		ID: SpecProducts, LabelKey: "search.specs_products", Permission: rbac.PermPlatformProductsRead,
		Icon: "products", Searchable: []string{"title", "subtitle", "keywords"},
	}
}

func (a *ProductsAdapter) ListAll(ctx context.Context) ([]searchengine.Document, error) {
	return a.list(ctx, pgtype.UUID{})
}

func (a *ProductsAdapter) Document(ctx context.Context, id string) (searchengine.Document, error) {
	arg, err := uuidArg(id)
	if err != nil {
		return searchengine.Document{}, err
	}
	docs, err := a.list(ctx, arg)
	if err != nil {
		return searchengine.Document{}, err
	}
	return single(docs, SpecProducts)
}

func (a *ProductsAdapter) list(ctx context.Context, id pgtype.UUID) ([]searchengine.Document, error) {
	rows, err := a.q.ListProductsForSearch(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]searchengine.Document, 0, len(rows))
	for _, p := range rows {
		out = append(out, searchengine.Document{
			ID:       p.Uuid.String(),
			Spec:     SpecProducts,
			Title:    p.Name,
			Subtitle: strings.TrimSpace(strings.Join([]string{p.Sku, p.Status}, " · ")),
			Keywords: []string{p.Slug, p.Sku},
			Href:     "/platform/products?open=" + p.Uuid.String(),
			Icon:     "products",
		})
	}
	return out, nil
}

// CompaniesAdapter indexes company accounts.
type CompaniesAdapter struct{ q *db.Queries }

func NewCompanies(q *db.Queries) *CompaniesAdapter { return &CompaniesAdapter{q: q} }

func (a *CompaniesAdapter) Spec() searchengine.Spec {
	return searchengine.Spec{
		ID: SpecCompanies, LabelKey: "search.specs_companies", Permission: rbac.PermPlatformAccountsRead,
		Icon: "companies", Searchable: []string{"title", "subtitle", "keywords"},
	}
}

func (a *CompaniesAdapter) ListAll(ctx context.Context) ([]searchengine.Document, error) {
	return a.list(ctx, pgtype.UUID{})
}

func (a *CompaniesAdapter) Document(ctx context.Context, id string) (searchengine.Document, error) {
	arg, err := uuidArg(id)
	if err != nil {
		return searchengine.Document{}, err
	}
	docs, err := a.list(ctx, arg)
	if err != nil {
		return searchengine.Document{}, err
	}
	return single(docs, SpecCompanies)
}

func (a *CompaniesAdapter) list(ctx context.Context, id pgtype.UUID) ([]searchengine.Document, error) {
	rows, err := a.q.ListCompaniesForSearch(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]searchengine.Document, 0, len(rows))
	for _, c := range rows {
		out = append(out, searchengine.Document{
			ID:       c.Uuid.String(),
			Spec:     SpecCompanies,
			Title:    c.Name,
			Subtitle: c.Slug,
			Keywords: []string{c.Slug, string(c.Status)},
			Href:     "/platform/companies/" + c.Uuid.String(),
			Icon:     "companies",
		})
	}
	return out, nil
}
