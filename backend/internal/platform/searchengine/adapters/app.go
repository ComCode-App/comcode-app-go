package adapters

import (
	"context"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	"github.com/jackc/pgx/v5/pgtype"
)

// Account-scoped app specs (TEC-951). Documents carry their account_id and the search
// service filters every query on the caller's active account, so members of one account
// never see another account's profiles or cards.
const (
	SpecProfiles = "profiles"
	SpecCards    = "cards"
)

// All returns every search adapter. The API, the worker and search-reindex register the same
// list so a full reindex covers every spec.
func All(q *db.Queries) []searchengine.Adapter {
	return []searchengine.Adapter{
		NewUsers(q),
		NewRoles(q),
		NewOrders(q),
		NewProducts(q),
		NewCompanies(q),
		NewProfiles(q),
		NewCards(q),
	}
}

func compact(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ProfilesAdapter indexes an account's profiles by name, title, company and username,
// across all of their translations (tr and en).
type ProfilesAdapter struct{ q *db.Queries }

func NewProfiles(q *db.Queries) *ProfilesAdapter { return &ProfilesAdapter{q: q} }

func (a *ProfilesAdapter) Spec() searchengine.Spec {
	return searchengine.Spec{
		ID: SpecProfiles, LabelKey: "search.specs_profiles", Permission: rbac.PermAppProfilesRead,
		Icon: "profiles", Searchable: []string{"title", "subtitle", "keywords"}, AccountScoped: true,
	}
}

func (a *ProfilesAdapter) ListAll(ctx context.Context) ([]searchengine.Document, error) {
	return a.list(ctx, pgtype.UUID{})
}

func (a *ProfilesAdapter) Document(ctx context.Context, id string) (searchengine.Document, error) {
	arg, err := uuidArg(id)
	if err != nil {
		return searchengine.Document{}, err
	}
	docs, err := a.list(ctx, arg)
	if err != nil {
		return searchengine.Document{}, err
	}
	return single(docs, SpecProfiles)
}

func (a *ProfilesAdapter) list(ctx context.Context, id pgtype.UUID) ([]searchengine.Document, error) {
	rows, err := a.q.ListProfilesForSearch(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]searchengine.Document, 0, len(rows))
	for _, p := range rows {
		title := p.DisplayName
		if title == "" {
			title = p.Username
		}
		out = append(out, searchengine.Document{
			ID:        p.Uuid.String(),
			Spec:      SpecProfiles,
			Title:     title,
			Subtitle:  strings.Join(compact(p.Title, p.CompanyName, p.Username), " · "),
			Keywords:  compact(p.Username, p.Translations),
			Href:      "/profiles/" + p.Uuid.String(),
			Icon:      "profiles",
			AccountID: p.AccountID,
		})
	}
	return out, nil
}

// CardsAdapter indexes an account's NFC cards by name, smart code and linked profile.
// Activation codes and passcodes are never indexed.
type CardsAdapter struct{ q *db.Queries }

func NewCards(q *db.Queries) *CardsAdapter { return &CardsAdapter{q: q} }

func (a *CardsAdapter) Spec() searchengine.Spec {
	return searchengine.Spec{
		ID: SpecCards, LabelKey: "search.specs_cards", Permission: rbac.PermAppCardsRead,
		Icon: "cards", Searchable: []string{"title", "subtitle", "keywords"}, AccountScoped: true,
	}
}

func (a *CardsAdapter) ListAll(ctx context.Context) ([]searchengine.Document, error) {
	return a.list(ctx, pgtype.UUID{})
}

func (a *CardsAdapter) Document(ctx context.Context, id string) (searchengine.Document, error) {
	arg, err := uuidArg(id)
	if err != nil {
		return searchengine.Document{}, err
	}
	docs, err := a.list(ctx, arg)
	if err != nil {
		return searchengine.Document{}, err
	}
	return single(docs, SpecCards)
}

func (a *CardsAdapter) list(ctx context.Context, id pgtype.UUID) ([]searchengine.Document, error) {
	rows, err := a.q.ListCardsForSearch(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]searchengine.Document, 0, len(rows))
	for _, c := range rows {
		out = append(out, searchengine.Document{
			ID:        c.Uuid.String(),
			Spec:      SpecCards,
			Title:     c.Name,
			Subtitle:  strings.Join(compact(c.SmartCode, c.ProfileName), " · "),
			Keywords:  compact(c.SmartCode, c.ProfileName, c.Status),
			Href:      "/cards?open=" + c.Uuid.String(),
			Icon:      "cards",
			AccountID: c.AccountID,
		})
	}
	return out, nil
}

var (
	_ searchengine.Adapter = (*ProfilesAdapter)(nil)
	_ searchengine.Adapter = (*CardsAdapter)(nil)
)
