// Package repository runs company analytics reads in one consistent, read-only snapshot.
package repository

import (
	"context"
	"errors"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Read reloads the caller's role in the company account and runs fn with it inside a read-only
// repeatable-read transaction, so totals, series and the comparison table see the same data.
// A caller that is not a member of a company account gets ErrForbidden.
func (r *Repository) Read(ctx context.Context, accountID, userID int64, fn func(*db.Queries, string) error) error {
	if accountID <= 0 || userID <= 0 {
		return model.ErrForbidden
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	role, err := q.CompanyAnalyticsActorRole(ctx, db.CompanyAnalyticsActorRoleParams{AccountID: accountID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrForbidden
	}
	if err != nil {
		return err
	}
	if err = fn(q, string(role)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
