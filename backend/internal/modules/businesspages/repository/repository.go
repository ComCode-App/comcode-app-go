// Package repository persists ComCode Business pages (TEC-1327). Every app method takes the active
// account; the visitor lookups are separate and read only published, live data.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound: no such row in the account (or no live public route).
var ErrNotFound = errors.New("not found")

// Route is a business page username row of the routes registry.
type Route struct {
	ID     int64
	Path   string
	Status string // active | locked | retired
}

// PathOwner is whatever holds a path in the routes registry.
type PathOwner struct {
	RouteID   int64
	RouteType string
	Status    string
	AccountID int64 // 0 = none
}

// PublicRoute is a business page route resolved by path, with its company account.
type PublicRoute struct {
	Route
	AccountID   int64
	AccountUUID uuid.UUID
	AccountName string
}

// NewBlock is one block row to write.
type NewBlock struct {
	UUID         uuid.UUID
	Type         string
	Enabled      bool
	Config       []byte
	Translations []byte
}

// Save is one transactional save of a page: settings, the full block list and the username.
type Save struct {
	AccountID     int64
	UserID        int64
	Theme         string
	PrimaryLocale string
	Translations  []byte
	Blocks        []NewBlock
	// Current is the live route (nil: none yet). NewPath set = assign or rename.
	Current  *Route
	NewPath  string
	ReviveID int64  // a retired route of this account that holds NewPath
	Status   string // route status for a new or revived route: active | locked
}

// Postgres is the sqlc-backed store.
type Postgres struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewPostgres(pool *pgxpool.Pool, q *db.Queries) *Postgres { return &Postgres{pool: pool, q: q} }

func (r *Postgres) withTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(r.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Page returns the account's page row.
func (r *Postgres) Page(ctx context.Context, accountID int64) (*db.GetBusinessPageRow, error) {
	row, err := r.q.GetBusinessPage(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("business page: %w", err)
	}
	return &row, nil
}

// Blocks lists the page's blocks in order.
func (r *Postgres) Blocks(ctx context.Context, accountID, pageID int64) ([]db.ListBusinessPageBlocksRow, error) {
	rows, err := r.q.ListBusinessPageBlocks(ctx, db.ListBusinessPageBlocksParams{AccountID: accountID, PageID: pageID})
	if err != nil {
		return nil, fmt.Errorf("business page blocks: %w", err)
	}
	return rows, nil
}

// LiveRoute is the account's active or locked business page route; nil when none.
func (r *Postgres) LiveRoute(ctx context.Context, accountID int64) (*Route, error) {
	row, err := r.q.GetBusinessPageRoute(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("business page route: %w", err)
	}
	return &Route{ID: row.ID, Path: row.Path, Status: row.Status}, nil
}

// PathOwner returns what holds path (nil: free). A legacy collision alias keeps its printed
// root identity reserved, as for profile usernames.
func (r *Postgres) PathOwner(ctx context.Context, path string) (*PathOwner, error) {
	if !strings.Contains(path, "/") {
		legacy, err := r.q.GetBusinessPathOwner(ctx, "r/legacy/"+strings.ToLower(strings.TrimSpace(path)))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("legacy path owner: %w", err)
		}
		if err == nil && legacy.RouteType == "legacy_route" {
			return &PathOwner{RouteID: legacy.ID, RouteType: legacy.RouteType, Status: legacy.Status}, nil
		}
	}
	row, err := r.q.GetBusinessPathOwner(ctx, path)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("path owner: %w", err)
	}
	return &PathOwner{RouteID: row.ID, RouteType: row.RouteType, Status: row.Status, AccountID: row.AccountID.Int64}, nil
}

// Save writes the page, replaces its blocks and assigns or renames its username in one
// transaction.
func (r *Postgres) Save(ctx context.Context, in Save) error {
	return r.withTx(ctx, func(q *db.Queries) error {
		pageID, err := q.UpsertBusinessPage(ctx, db.UpsertBusinessPageParams{
			AccountID: in.AccountID, Theme: in.Theme, PrimaryLocale: in.PrimaryLocale, Translations: in.Translations,
			UpdatedByUserID: pgtype.Int8{Int64: in.UserID, Valid: in.UserID > 0},
		})
		if err != nil {
			return fmt.Errorf("save business page: %w", err)
		}
		if err := q.DeleteBusinessPageBlocks(ctx, db.DeleteBusinessPageBlocksParams{AccountID: in.AccountID, PageID: pageID}); err != nil {
			return fmt.Errorf("clear blocks: %w", err)
		}
		for i, b := range in.Blocks {
			if err := q.InsertBusinessPageBlock(ctx, db.InsertBusinessPageBlockParams{
				Uuid: b.UUID, AccountID: in.AccountID, PageID: pageID, Type: b.Type, Position: int32(i),
				Enabled: b.Enabled, Config: b.Config, Translations: b.Translations,
			}); err != nil {
				return fmt.Errorf("add block: %w", err)
			}
		}
		if in.NewPath == "" {
			return nil
		}
		if in.Current != nil {
			if err := q.RetireBusinessPageRoute(ctx, db.RetireBusinessPageRouteParams{ID: in.Current.ID, AccountID: in.AccountID}); err != nil {
				return fmt.Errorf("retire route: %w", err)
			}
			// The old username keeps redirecting to the new one.
			if err := q.InsertRouteHistory(ctx, db.InsertRouteHistoryParams{
				RouteID: in.Current.ID, RouteType: "business_page",
				AccountID: pgtype.Int8{Int64: in.AccountID, Valid: true},
				OldPath:   in.Current.Path, NewPath: in.NewPath,
				ChangedByUserID: pgtype.Int8{Int64: in.UserID, Valid: in.UserID > 0},
				Reason:          "username_change",
			}); err != nil {
				return fmt.Errorf("route history: %w", err)
			}
		}
		if in.ReviveID != 0 {
			n, err := q.ReviveBusinessPageRoute(ctx, db.ReviveBusinessPageRouteParams{Status: in.Status, ID: in.ReviveID, AccountID: in.AccountID})
			if err != nil {
				return fmt.Errorf("revive route: %w", err)
			}
			if n == 0 {
				return ErrNotFound
			}
			return nil
		}
		if _, err := q.CreateBusinessPageRoute(ctx, db.CreateBusinessPageRouteParams{
			Path: in.NewPath, Status: in.Status, AccountID: in.AccountID,
			OwnerUserID: pgtype.Int8{Int64: in.UserID, Valid: in.UserID > 0},
		}); err != nil {
			return fmt.Errorf("create route: %w", err)
		}
		return nil
	})
}

// SetStatus publishes (route active) or unpublishes (route locked) the page.
func (r *Postgres) SetStatus(ctx context.Context, accountID, userID int64, status string, route *Route) error {
	routeStatus := "locked"
	if status == "published" {
		routeStatus = "active"
	}
	return r.withTx(ctx, func(q *db.Queries) error {
		n, err := q.SetBusinessPageStatus(ctx, db.SetBusinessPageStatusParams{
			Status: status, UpdatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0}, AccountID: accountID,
		})
		if err != nil {
			return fmt.Errorf("business page status: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		if route == nil {
			return nil
		}
		if err := q.SetBusinessPageRouteStatus(ctx, db.SetBusinessPageRouteStatusParams{Status: routeStatus, ID: route.ID, AccountID: accountID}); err != nil {
			return fmt.Errorf("business page route status: %w", err)
		}
		return nil
	})
}

// PublicRoute resolves a business page route by path (any status) whose account is a live company.
func (r *Postgres) PublicRoute(ctx context.Context, path string) (*PublicRoute, error) {
	row, err := r.q.GetPublicBusinessRoute(ctx, path)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("public business route: %w", err)
	}
	return &PublicRoute{
		Route:     Route{ID: row.ID, Path: row.Path, Status: row.Status},
		AccountID: row.AccountID, AccountUUID: row.AccountUuid, AccountName: row.AccountName,
	}, nil
}

// Team lists the company's published, open employee profiles.
func (r *Postgres) Team(ctx context.Context, accountID int64, locale string) ([]db.ListBusinessTeamProfilesRow, error) {
	return r.q.ListBusinessTeamProfiles(ctx, db.ListBusinessTeamProfilesParams{AccountID: accountID, Locale: locale})
}

// Offices lists the company's offices.
func (r *Postgres) Offices(ctx context.Context, accountID int64) ([]db.ListBusinessOfficesRow, error) {
	return r.q.ListBusinessOffices(ctx, accountID)
}

// BankAccounts lists the active company bank accounts among ids.
func (r *Postgres) BankAccounts(ctx context.Context, accountID int64, ids []uuid.UUID) ([]db.ListBusinessBankAccountsRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return r.q.ListBusinessBankAccounts(ctx, db.ListBusinessBankAccountsParams{AccountID: accountID, Uuids: ids})
}

// MediaKeys maps company library image uuids to their object keys.
func (r *Postgres) MediaKeys(ctx context.Context, accountID int64, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.q.ListBusinessMedia(ctx, db.ListBusinessMediaParams{AccountID: accountID, Uuids: ids})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.Uuid] = row.ObjectKey
	}
	return out, nil
}

// SocialIcons maps social platform slugs to icon URLs.
func (r *Postgres) SocialIcons(ctx context.Context, slugs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(slugs) == 0 {
		return out, nil
	}
	rows, err := r.q.ListSocialPlatformIconsBySlug(ctx, slugs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.Slug] = row.IconUrl
	}
	return out, nil
}
