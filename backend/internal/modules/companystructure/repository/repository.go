// Package repository executes company-scoped structure transactions.
package repository

import (
	"context"
	"errors"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	AddStructureScope(ctx context.Context, arg db.AddStructureScopeParams) (int64, error)
	ClearStructureScopes(ctx context.Context, arg db.ClearStructureScopesParams) error
	CountStructureAssignments(ctx context.Context, arg db.CountStructureAssignmentsParams) (int64, error)
	CountStructureGroups(ctx context.Context, arg db.CountStructureGroupsParams) (int64, error)
	CountStructureOffices(ctx context.Context, arg db.CountStructureOfficesParams) (int64, error)
	CreateStructureGroup(ctx context.Context, arg db.CreateStructureGroupParams) (db.CompanyGroup, error)
	CreateStructureOffice(ctx context.Context, arg db.CreateStructureOfficeParams) (db.CompanyOffice, error)
	DeleteStructureGroup(ctx context.Context, arg db.DeleteStructureGroupParams) (int64, error)
	DeleteStructureOffice(ctx context.Context, arg db.DeleteStructureOfficeParams) (int64, error)
	GetCompanySector(ctx context.Context, id int64) (pgtype.UUID, error)
	GetStructureAssignment(ctx context.Context, arg db.GetStructureAssignmentParams) (db.GetStructureAssignmentRow, error)
	GetStructureGroup(ctx context.Context, arg db.GetStructureGroupParams) (db.GetStructureGroupRow, error)
	GetStructureOffice(ctx context.Context, arg db.GetStructureOfficeParams) (db.CompanyOffice, error)
	ListStructureAssignments(ctx context.Context, arg db.ListStructureAssignmentsParams) ([]db.ListStructureAssignmentsRow, error)
	ListStructureGroups(ctx context.Context, arg db.ListStructureGroupsParams) ([]db.ListStructureGroupsRow, error)
	ListStructureOffices(ctx context.Context, arg db.ListStructureOfficesParams) ([]db.CompanyOffice, error)
	ListStructureScopes(ctx context.Context, arg db.ListStructureScopesParams) ([]uuid.UUID, error)
	SetCompanySector(ctx context.Context, arg db.SetCompanySectorParams) (int64, error)
	SetStructureAssignment(ctx context.Context, arg db.SetStructureAssignmentParams) (int64, error)
	StructureHasOfficeScope(ctx context.Context, arg db.StructureHasOfficeScopeParams) (bool, error)
	UpdateStructureGroup(ctx context.Context, arg db.UpdateStructureGroupParams) (db.CompanyGroup, error)
	UpdateStructureOffice(ctx context.Context, arg db.UpdateStructureOfficeParams) (db.CompanyOffice, error)
	GetAccountMemberByUserUUID(context.Context, db.GetAccountMemberByUserUUIDParams) (db.GetAccountMemberByUserUUIDRow, error)
	InsertActivityEvent(ctx context.Context, arg db.InsertActivityEventParams) (db.ActivityEvent, error)
}
type Repository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func New(pool *pgxpool.Pool, q *db.Queries) *Repository { return &Repository{pool: pool, q: q} }

// InCompany serializes structure writes and scope changes. Reloading membership under the
// account lock prevents a stale principal from keeping revoked company privileges.
func (r *Repository) InCompany(ctx context.Context, p authctx.Principal, fn func(Store, string) error) error {
	if p.Account.ID <= 0 || p.Account.Type != authctx.AccountTypeCompany {
		return model.ErrForbidden
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.q.WithTx(tx)
	if _, err = q.LockStructureAccount(ctx, p.Account.ID); err != nil {
		return MapError(err)
	}
	role, err := q.StructureActorRole(ctx, db.StructureActorRoleParams{AccountID: p.Account.ID, UserID: p.UserInternal})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ErrForbidden
		}
		return err
	}
	if err = fn(q, string(role)); err != nil {
		return MapError(err)
	}
	return MapError(tx.Commit(ctx))
}

// ReadAdmin is deliberately separate from active-account operations.
func (r *Repository) ReadAdmin(ctx context.Context, account uuid.UUID, fn func(Store, int64) error) error {
	id, err := r.q.GetStructureAccountAdmin(ctx, account)
	if err != nil {
		return MapError(err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.q.WithTx(tx)
	if _, err = q.LockStructureAccount(ctx, id); err != nil {
		return MapError(err)
	}
	if err = fn(q, id); err != nil {
		return MapError(err)
	}
	return MapError(tx.Commit(ctx))
}
func MapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrNotFound
	}
	var e *pgconn.PgError
	if errors.As(err, &e) {
		switch e.Code {
		case "23503", "23505":
			return model.ErrConflict
		case "23514", "22001":
			return model.ErrInvalid
		}
	}
	return err
}
