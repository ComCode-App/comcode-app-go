// Package repository reads and writes cards and routes for platform support (TEC-950). Every
// method here is unscoped on purpose: these are platform routes, not account data.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/model"
)

// QuarantineWindow mirrors the 30-day hold in CheckPathExists (TEC-559/558).
const QuarantineWindow = 30 * 24 * time.Hour

type Repository interface {
	SearchCards(ctx context.Context, prefix string, limit int32) ([]model.Card, error)
	GetCard(ctx context.Context, id uuid.UUID) (*model.Card, error)
	// RetireCard retires the card's route and clears an unused activation code. It returns the
	// retired paths.
	RetireCard(ctx context.Context, cardID int64) ([]string, error)
	GetReassignTarget(ctx context.Context, profileUUID uuid.UUID) (*model.ReassignTarget, error)
	ReassignCard(ctx context.Context, cardID int64, target model.ReassignTarget) error
	SearchRoutes(ctx context.Context, pattern string, limit int32) ([]model.Route, error)
	GetRoute(ctx context.Context, id uuid.UUID) (*model.Route, error)
	// ReleasePath ends every quarantine hold on the path and returns the routes that held it.
	ReleasePath(ctx context.Context, path string) ([]uuid.UUID, error)
}

type Postgres struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewPostgres(pool *pgxpool.Pool, q *db.Queries) *Postgres {
	return &Postgres{pool: pool, q: q}
}

func (r *Postgres) withTx(ctx context.Context, fn func(q *db.Queries) error) error {
	if r.pool == nil {
		return fn(r.q)
	}
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

// escapeLike makes user input literal inside a LIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *Postgres) SearchCards(ctx context.Context, prefix string, limit int32) ([]model.Card, error) {
	rows, err := r.q.PlatformSearchCards(ctx, db.PlatformSearchCardsParams{Prefix: escapeLike(prefix), LimitCount: limit})
	if err != nil {
		return nil, err
	}
	out := make([]model.Card, 0, len(rows))
	for _, row := range rows {
		out = append(out, cardFromRow(db.PlatformGetCardRow(row)))
	}
	return out, nil
}

func (r *Postgres) GetCard(ctx context.Context, id uuid.UUID) (*model.Card, error) {
	row, err := r.q.PlatformGetCard(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c := cardFromRow(row)
	return &c, nil
}

func (r *Postgres) RetireCard(ctx context.Context, cardID int64) ([]string, error) {
	return r.q.PlatformRetireCard(ctx, pgtype.Int8{Int64: cardID, Valid: true})
}

func (r *Postgres) GetReassignTarget(ctx context.Context, profileUUID uuid.UUID) (*model.ReassignTarget, error) {
	row, err := r.q.PlatformGetProfileForCard(ctx, profileUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &model.ReassignTarget{
		ProfileID: row.ID, ProfileUUID: row.Uuid, AccountID: row.AccountID, OwnerUserID: row.OwnerUserID,
	}, nil
}

func (r *Postgres) ReassignCard(ctx context.Context, cardID int64, t model.ReassignTarget) error {
	_, err := r.q.PlatformReassignCard(ctx, db.PlatformReassignCardParams{
		ProfileID: pgtype.Int8{Int64: t.ProfileID, Valid: true},
		AccountID: t.AccountID,
		UserID:    t.OwnerUserID,
		CardID:    cardID,
	})
	return err
}

func (r *Postgres) SearchRoutes(ctx context.Context, pattern string, limit int32) ([]model.Route, error) {
	ids, err := r.q.PlatformSearchRoutes(ctx, db.PlatformSearchRoutesParams{Pattern: escapeLike(pattern), LimitCount: limit})
	if err != nil || len(ids) == 0 {
		return []model.Route{}, err
	}
	routes, err := r.loadRoutes(ctx, ids)
	if err != nil {
		return nil, err
	}
	// Keep the search order.
	byID := make(map[int64]model.Route, len(routes))
	for _, rt := range routes {
		byID[rt.ID] = rt
	}
	out := make([]model.Route, 0, len(ids))
	for _, id := range ids {
		if rt, ok := byID[id]; ok {
			out = append(out, rt)
		}
	}
	return out, nil
}

func (r *Postgres) GetRoute(ctx context.Context, id uuid.UUID) (*model.Route, error) {
	routeID, err := r.q.PlatformGetRouteIDByUUID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	routes, err := r.loadRoutes(ctx, []int64{routeID})
	if err != nil || len(routes) == 0 {
		return nil, err
	}
	return &routes[0], nil
}

func (r *Postgres) loadRoutes(ctx context.Context, ids []int64) ([]model.Route, error) {
	rows, err := r.q.PlatformGetRoutes(ctx, ids)
	if err != nil {
		return nil, err
	}
	hist, err := r.q.PlatformListRouteHistory(ctx, ids)
	if err != nil {
		return nil, err
	}
	byRoute := map[int64][]model.HistoryEntry{}
	for _, h := range hist {
		e := model.HistoryEntry{
			OldPath: h.OldPath, NewPath: h.NewPath, Reason: h.Reason, Quarantined: h.Quarantined,
			QuarantineReleasedAt: timePtr(h.QuarantineReleasedAt), CreatedAt: h.CreatedAt.Time,
		}
		if h.Quarantined {
			until := h.CreatedAt.Time.Add(QuarantineWindow)
			e.QuarantineUntil = &until
		}
		if h.ChangedByUuid.Valid {
			e.ChangedBy = &model.UserRef{UUID: h.ChangedByUuid.Bytes, Email: h.ChangedByEmail.String}
		}
		byRoute[h.RouteID] = append(byRoute[h.RouteID], e)
	}
	out := make([]model.Route, 0, len(rows))
	for _, row := range rows {
		rt := model.Route{
			ID: row.ID, UUID: row.Uuid, Path: row.Path, RouteType: row.RouteType, Status: row.Status,
			IsImmutable: row.IsImmutable, TargetType: row.TargetType, DirectURL: row.DirectUrl,
			DeletedAt: timePtr(row.DeletedAt), QuarantineReleasedAt: timePtr(row.QuarantineReleasedAt),
			History: byRoute[row.ID], CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		}
		if rt.History == nil {
			rt.History = []model.HistoryEntry{}
		}
		switch {
		case row.Quarantined:
			rt.Status = model.RouteStatusQuarantine
			until := row.DeletedAt.Time.Add(QuarantineWindow)
			rt.QuarantineUntil = &until
		case row.DeletedAt.Valid:
			rt.Status = "retired"
		}
		if row.AccountUuid.Valid {
			rt.Account = &model.AccountRef{UUID: row.AccountUuid.Bytes, Name: row.AccountName.String}
		}
		if row.OwnerUuid.Valid {
			rt.Owner = &model.UserRef{
				UUID: row.OwnerUuid.Bytes, Email: row.OwnerEmail.String,
				Name: row.OwnerName.String, Surname: row.OwnerSurname.String,
			}
		}
		if row.ProfileUuid.Valid {
			rt.Profile = &model.ProfileRef{UUID: row.ProfileUuid.Bytes, Username: row.ProfileUsername}
		}
		if row.CardUuid.Valid {
			rt.Card = &model.CardRef{UUID: row.CardUuid.Bytes, SmartCode: row.CardSmartCode.String}
		}
		out = append(out, rt)
	}
	return out, nil
}

func (r *Postgres) ReleasePath(ctx context.Context, path string) ([]uuid.UUID, error) {
	var out []uuid.UUID
	err := r.withTx(ctx, func(q *db.Queries) error {
		deleted, err := q.PlatformReleaseDeletedRoutePath(ctx, path)
		if err != nil {
			return err
		}
		renamed, err := q.PlatformReleaseRenamedPath(ctx, path)
		if err != nil {
			return err
		}
		seen := map[uuid.UUID]bool{}
		for _, id := range append(deleted, renamed...) {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
		return nil
	})
	return out, err
}

func cardFromRow(row db.PlatformGetCardRow) model.Card {
	c := model.Card{
		ID: row.ID, UUID: row.Uuid, Name: row.Name, SmartCode: row.SmartCode, CardUID: row.CardUid.String,
		LegacySerial: row.LegacySerial, TargetType: row.TargetType, DirectURL: row.DirectUrl,
		IsActive: row.IsActive, IsLocked: row.IsLocked,
		Account:   model.AccountRef{UUID: row.AccountUuid, Name: row.AccountName, Type: row.AccountType},
		Owner:     model.UserRef{UUID: row.OwnerUuid, Email: row.OwnerEmail, Name: row.OwnerName, Surname: row.OwnerSurname},
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
	if row.ProfileUuid.Valid {
		c.Profile = &model.ProfileRef{UUID: row.ProfileUuid.Bytes, Username: row.ProfileUsername}
	}
	if row.OrderUuid.Valid {
		c.Order = &model.OrderRef{UUID: row.OrderUuid.Bytes, Number: row.OrderNumber.String, Status: row.OrderStatus.String}
	}
	if row.RouteUuid.Valid {
		c.Route = &model.RouteRef{UUID: row.RouteUuid.Bytes, Path: row.RoutePath.String, Status: row.RouteStatus.String}
	}
	switch {
	case c.Route != nil && c.Route.Status == "retired":
		c.ActivationStatus = model.ActivationRetired
	case row.PendingActivation:
		c.ActivationStatus = model.ActivationPending
	default:
		c.ActivationStatus = model.ActivationActive
	}
	return c
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
