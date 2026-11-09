package searchengine_test

import (
	"context"
	"slices"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine/searchtest"
)

var (
	platformSpec = searchengine.Spec{ID: "products"}
	scopedSpec   = searchengine.Spec{ID: "profiles", AccountScoped: true}
)

func seed(t *testing.T, meili *searchtest.Meili, c *searchengine.Client) {
	t.Helper()
	ctx := context.Background()
	for _, spec := range []searchengine.Spec{platformSpec, scopedSpec} {
		if err := c.EnsureIndex(ctx, spec); err != nil {
			t.Fatalf("ensure %s: %v", spec.ID, err)
		}
	}
	if err := c.UpsertDocuments(ctx, "products", []searchengine.Document{
		{ID: "p1", Spec: "products", Title: "Ada card stock"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.UpsertDocuments(ctx, "profiles", []searchengine.Document{
		{ID: "a", Spec: "profiles", Title: "Ada of account 1", AccountID: 1},
		{ID: "b", Spec: "profiles", Title: "Ada of account 2", AccountID: 2},
	}); err != nil {
		t.Fatal(err)
	}
}

func hitIDs(hits []searchengine.Hit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Spec+":"+h.ID)
	}
	slices.Sort(out)
	return out
}

func TestEnsureIndexMakesAccountIDFilterable(t *testing.T) {
	meili := searchtest.New(t)
	c := searchengine.NewClient(meili.Config(), nil)
	seed(t, meili, c)

	filterable := func(index string) []any {
		v, _ := meili.Settings(index)["filterableAttributes"].([]any)
		return v
	}
	if !slices.Contains(filterable("test_profiles"), any("account_id")) {
		t.Fatalf("profiles filterable = %v, want account_id", filterable("test_profiles"))
	}
	if slices.Contains(filterable("test_products"), any("account_id")) {
		t.Fatalf("products filterable = %v, platform specs are not account scoped", filterable("test_products"))
	}
}

func TestSearchFiltersAccountScopedSpecs(t *testing.T) {
	meili := searchtest.New(t)
	c := searchengine.NewClient(meili.Config(), nil)
	seed(t, meili, c)

	hits, err := c.Search(context.Background(), []searchengine.Spec{platformSpec, scopedSpec}, "ada", 20, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := hitIDs(hits), []string{"products:p1", "profiles:a"}; !slices.Equal(got, want) {
		t.Fatalf("hits = %v, want %v", got, want)
	}
	for _, q := range meili.Queries() {
		if q.IndexUID == "test_profiles" && q.Filter != "account_id = 1" {
			t.Fatalf("profiles filter = %v", q.Filter)
		}
		if q.IndexUID == "test_products" && q.Filter != nil {
			t.Fatalf("products filter = %v, platform search stays unscoped", q.Filter)
		}
	}
}

func TestSearchWithoutAccountFailsClosed(t *testing.T) {
	meili := searchtest.New(t)
	c := searchengine.NewClient(meili.Config(), nil)
	seed(t, meili, c)

	hits, err := c.Search(context.Background(), []searchengine.Spec{scopedSpec}, "ada", 20, 0)
	if err != nil || len(hits) != 0 {
		t.Fatalf("hits = %v, err = %v; want none", hits, err)
	}
	if n := len(meili.Queries()); n != 0 {
		t.Fatalf("sent %d queries, want none", n)
	}

	// Platform specs in the same request still answer.
	hits, err = c.Search(context.Background(), []searchengine.Spec{platformSpec, scopedSpec}, "ada", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := hitIDs(hits), []string{"products:p1"}; !slices.Equal(got, want) {
		t.Fatalf("hits = %v, want %v", got, want)
	}
}

func TestSearchDropsForeignHitsWhenIndexIgnoresFilter(t *testing.T) {
	meili := searchtest.New(t)
	c := searchengine.NewClient(meili.Config(), nil)
	seed(t, meili, c)
	meili.IgnoreFilter = true

	hits, err := c.Search(context.Background(), []searchengine.Spec{scopedSpec}, "ada", 20, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := hitIDs(hits), []string{"profiles:b"}; !slices.Equal(got, want) {
		t.Fatalf("hits = %v, want %v", got, want)
	}
}
