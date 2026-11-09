// Package repository serializes employee asset operations with company structure changes.
package repository

import (
	"context"
	"errors"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// InCompany shares TEC-571's company/member locks and scope queries. All domain writes in fn
// use this transaction, so scope revocation, assignment changes and partial orders cannot race it.
func (r *Repository) InCompany(ctx context.Context, p authctx.Principal, fn func(*db.Queries, string) error) error {
	if p.Account.ID <= 0 || p.Account.Type != authctx.AccountTypeCompany || p.UserInternal <= 0 {
		return model.ErrForbidden
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	if _, err = q.LockStructureAccount(ctx, p.Account.ID); err != nil {
		return mapError(err)
	}
	role, err := q.StructureActorRole(ctx, db.StructureActorRoleParams{AccountID: p.Account.ID, UserID: p.UserInternal})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrForbidden
	}
	if err != nil {
		return err
	}
	if err = fn(q, string(role)); err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrNotFound
	}
	return err
}
