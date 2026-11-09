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
)

// CanonicalRoute is a profile's username row in the routes registry.
type CanonicalRoute struct {
	ID                 int64
	Path               string
	Status             string // active | locked | retired
	PresentationConfig []byte
}

// PublicRoute is a canonical route resolved by path, with its profile.
type PublicRoute struct {
	CanonicalRoute
	ProfileID   int64
	ProfileUUID uuid.UUID
	AccountID   int64
}

// PathOwner describes whatever currently holds a path.
type PathOwner struct {
	RouteType string
	Status    string
	ProfileID int64 // 0 when the route has no profile
	RouteID   int64
}

// ApplyPublicPage is one transactional change to a profile's public page.
type ApplyPublicPage struct {
	ProfileID   int64
	AccountID   int64
	OwnerUserID int64
	Current     *CanonicalRoute // live route, nil if the profile has none
	NewPath     string          // set to rename (or assign the first username)
	ReviveID    int64           // a retired route of this profile that holds NewPath
	Status      string          // active | locked
	Config      []byte
	BankIDs     *[]int64 // nil leaves the selection unchanged
	ChangedBy   int64    // actor recorded in route_history on a rename
}

// PublicPageStore persists usernames, presentation config and bank account picks.
type PublicPageStore interface {
	ProfileRef(ctx context.Context, accountID int64, profileUUID uuid.UUID) (profileID, ownerUserID int64, err error)
	LiveRoute(ctx context.Context, profileID int64) (*CanonicalRoute, error)
	PathOwner(ctx context.Context, path string) (*PathOwner, error)
	RouteByPath(ctx context.Context, path string) (*PublicRoute, error)
	BankAccountIDs(ctx context.Context, accountID int64, uuids []uuid.UUID) (map[uuid.UUID]int64, error)
	PublicBankAccounts(ctx context.Context, profileID int64) ([]db.ListProfileBankAccountsPublicRow, error)
	Apply(ctx context.Context, in ApplyPublicPage) error
}

var _ PublicPageStore = (*Postgres)(nil)

func (r *Postgres) ProfileRef(ctx context.Context, accountID int64, profileUUID uuid.UUID) (int64, int64, error) {
	row, err := r.q.GetProfileIDForAccount(ctx, db.GetProfileIDForAccountParams{Uuid: profileUUID, AccountID: accountID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, ErrNotFound
	}
	if err != nil {
		return 0, 0, fmt.Errorf("profile ref: %w", err)
	}
	return row.ID, row.UserID.Int64, nil
}

func (r *Postgres) LiveRoute(ctx context.Context, profileID int64) (*CanonicalRoute, error) {
	row, err := r.q.GetLiveCanonicalRouteForProfile(ctx, pgtype.Int8{Int64: profileID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("live canonical route: %w", err)
	}
	return &CanonicalRoute{ID: row.ID, Path: row.Path, Status: row.Status, PresentationConfig: row.PresentationConfig}, nil
}

func (r *Postgres) PathOwner(ctx context.Context, path string) (*PathOwner, error) {
	// A collision alias permanently reserves its original printed root identity.
	if !strings.Contains(path, "/") {
		legacy, err := r.q.GetRouteByPathAnyType(ctx, "r/legacy/"+strings.ToLower(strings.TrimSpace(path)))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("legacy path owner: %w", err)
		}
		if err == nil && legacy.RouteType == "legacy_route" {
			return &PathOwner{RouteType: legacy.RouteType, Status: legacy.Status, RouteID: legacy.ID}, nil
		}
	}
	row, err := r.q.GetRouteByPathAnyType(ctx, path)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("path owner: %w", err)
	}
	return &PathOwner{RouteType: row.RouteType, Status: row.Status, ProfileID: row.ProfileID.Int64, RouteID: row.ID}, nil
}

func (r *Postgres) RouteByPath(ctx context.Context, path string) (*PublicRoute, error) {
	row, err := r.q.GetCanonicalRouteByPath(ctx, path)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("route by path: %w", err)
	}
	return &PublicRoute{
		CanonicalRoute: CanonicalRoute{ID: row.ID, Path: row.Path, Status: row.Status, PresentationConfig: row.PresentationConfig},
		ProfileID:      row.ProfileID,
		ProfileUUID:    row.ProfileUuid,
		AccountID:      row.AccountID,
	}, nil
}

func (r *Postgres) BankAccountIDs(ctx context.Context, accountID int64, uuids []uuid.UUID) (map[uuid.UUID]int64, error) {
	out := make(map[uuid.UUID]int64, len(uuids))
	if len(uuids) == 0 {
		return out, nil
	}
	rows, err := r.q.ListAccountBankAccountIDsByUUID(ctx, db.ListAccountBankAccountIDsByUUIDParams{AccountID: accountID, Uuids: uuids})
	if err != nil {
		return nil, fmt.Errorf("bank account ids: %w", err)
	}
	for _, row := range rows {
		out[row.Uuid] = row.ID
	}
	return out, nil
}

func (r *Postgres) PublicBankAccounts(ctx context.Context, profileID int64) ([]db.ListProfileBankAccountsPublicRow, error) {
	rows, err := r.q.ListProfileBankAccountsPublic(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("profile bank accounts: %w", err)
	}
	return rows, nil
}

func (r *Postgres) Apply(ctx context.Context, in ApplyPublicPage) error {
	return r.withTx(ctx, func(q *db.Queries) error {
		switch {
		case in.NewPath != "":
			if in.Current != nil {
				if err := q.RetireCanonicalRoute(ctx, in.Current.ID); err != nil {
					return fmt.Errorf("retire route: %w", err)
				}
				// TEC-558: the old username keeps redirecting (301) to the new one.
				if err := q.InsertRouteHistory(ctx, db.InsertRouteHistoryParams{
					RouteID:         in.Current.ID,
					RouteType:       "canonical_profile",
					AccountID:       pgtype.Int8{Int64: in.AccountID, Valid: true},
					OldPath:         in.Current.Path,
					NewPath:         in.NewPath,
					ChangedByUserID: pgtype.Int8{Int64: in.ChangedBy, Valid: in.ChangedBy != 0},
					Reason:          "username_change",
				}); err != nil {
					return fmt.Errorf("route history: %w", err)
				}
			}
			if in.ReviveID != 0 {
				if err := q.ReviveCanonicalRoute(ctx, db.ReviveCanonicalRouteParams{ID: in.ReviveID, Status: in.Status, PresentationConfig: in.Config}); err != nil {
					return fmt.Errorf("revive route: %w", err)
				}
				break
			}
			if _, err := q.CreateCanonicalRoute(ctx, db.CreateCanonicalRouteParams{
				Path:               in.NewPath,
				ProfileID:          pgtype.Int8{Int64: in.ProfileID, Valid: true},
				AccountID:          in.AccountID,
				OwnerUserID:        pgtype.Int8{Int64: in.OwnerUserID, Valid: in.OwnerUserID != 0},
				PresentationConfig: in.Config,
			}); err != nil {
				return fmt.Errorf("create route: %w", err)
			}
		case in.Current != nil:
			if err := q.SetCanonicalRouteState(ctx, db.SetCanonicalRouteStateParams{ID: in.Current.ID, Status: in.Status, PresentationConfig: in.Config}); err != nil {
				return fmt.Errorf("update route: %w", err)
			}
		}
		if in.BankIDs != nil {
			if err := q.DeleteProfileBankAccounts(ctx, in.ProfileID); err != nil {
				return fmt.Errorf("clear bank accounts: %w", err)
			}
			for i, id := range *in.BankIDs {
				if err := q.InsertProfileBankAccount(ctx, db.InsertProfileBankAccountParams{ProfileID: in.ProfileID, BankAccountID: id, DisplayOrder: int32(i)}); err != nil {
					return fmt.Errorf("add bank account: %w", err)
				}
			}
		}
		return nil
	})
}
