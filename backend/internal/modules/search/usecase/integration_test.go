package usecase_test

// Account isolation of Cmd+K profile and card search (TEC-951). Seeds Postgres (skipped unless
// TEST_DATABASE_URL is set, see internal/testutil/pgtest) and indexes through the real adapters
// into an in-memory Meilisearch.

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	searchusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/search/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine/searchtest"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

// seedOwner creates a user whose personal account has an "Ada <surname>" profile and a card
// linked to it, and returns the account id with the profile and card uuids.
func seedOwner(t *testing.T, d *pgtest.DB, surname string) (int64, string, string) {
	t.Helper()
	ctx := context.Background()
	u := d.CreateUser(t)
	var profileID int64
	var profileUUID uuid.UUID
	if err := d.Pool.QueryRow(ctx, `INSERT INTO profiles (user_id, account_id, is_primary) VALUES ($1, $2, true)
		RETURNING id, uuid`, u.ID, u.AccountID).Scan(&profileID, &profileUUID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `INSERT INTO profile_translations (profile_id, locale, first_name, last_name, title, is_default)
		VALUES ($1, 'tr', 'Ada', $2, 'Mühendis', true), ($1, 'en', 'Ada', $2, 'Engineer', false)`, profileID, surname); err != nil {
		t.Fatal(err)
	}
	card := d.CreateCard(t, u)
	if _, err := d.Pool.Exec(ctx, `UPDATE nfc_cards SET name = 'Ada kartı', profile_id = $2 WHERE id = $1`, card.ID, profileID); err != nil {
		t.Fatal(err)
	}
	return u.AccountID, profileUUID.String(), card.UUID.String()
}

func TestIntegrationSearchIsolatesAccounts(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	accountA, profileA, cardA := seedOwner(t, d, "Lovelace")
	accountB, profileB, cardB := seedOwner(t, d, "Byron")

	meili := searchtest.New(t)
	client := searchengine.NewClient(meili.Config(), nil)
	reg := searchengine.NewRegistry(adapters.All(d.Queries)...)
	// A full reindex, as search-reindex runs it.
	if err := searchengine.NewIndexer(client, reg, nil, nil).ProcessReindex(ctx, ""); err != nil {
		t.Fatal(err)
	}
	svc := searchusecase.New(client, reg, nil)

	search := func(account int64, spec, q string, perms ...string) []string {
		t.Helper()
		pctx := authctx.WithPrincipal(ctx, authctx.Principal{
			Permissions: perms,
			Account:     authctx.Account{ID: account, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner},
		})
		var out []string
		for _, h := range svc.Search(pctx, q, spec, 50) {
			out = append(out, h.Spec+":"+h.ID)
		}
		slices.Sort(out)
		return out
	}
	appPerms := []string{rbac.PermAppProfilesRead, rbac.PermAppCardsRead}
	sorted := func(v ...string) []string { slices.Sort(v); return v }

	// Both accounts have an "Ada"; each only finds its own profile and card.
	if got, want := search(accountA, "", "ada", appPerms...), sorted("profiles:"+profileA, "cards:"+cardA); !slices.Equal(got, want) {
		t.Fatalf("account A hits = %v, want %v", got, want)
	}
	if got, want := search(accountB, "", "ada", appPerms...), sorted("profiles:"+profileB, "cards:"+cardB); !slices.Equal(got, want) {
		t.Fatalf("account B hits = %v, want %v", got, want)
	}
	// Searching for the other account's distinct surname finds nothing.
	if got := search(accountA, "profiles", "byron", appPerms...); len(got) != 0 {
		t.Fatalf("account A found account B's profile: %v", got)
	}
	if got := search(accountB, "cards", "lovelace", appPerms...); len(got) != 0 {
		t.Fatalf("account B found account A's card: %v", got)
	}
	// English translation and username are searchable too.
	if got := search(accountA, "profiles", "engineer", appPerms...); !slices.Equal(got, []string{"profiles:" + profileA}) {
		t.Fatalf("en translation hits = %v", got)
	}
	// No active account: fail closed.
	if got := search(0, "", "ada", appPerms...); len(got) != 0 {
		t.Fatalf("no-account hits = %v, want none", got)
	}
	// Without the app read permissions the specs are not searched at all.
	if got := search(accountA, "", "ada"); len(got) != 0 {
		t.Fatalf("unpermitted hits = %v, want none", got)
	}
}
