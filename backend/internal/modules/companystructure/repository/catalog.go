package repository

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/google/uuid"
)

type CatalogStore interface {
	CountSectors(ctx context.Context, query string) (int64, error)
	CreateSectorAdmin(ctx context.Context, arg db.CreateSectorAdminParams) (db.Sector, error)
	DeleteSectorAdmin(ctx context.Context, id uuid.UUID) (int64, error)
	GetSectorAdmin(ctx context.Context, id uuid.UUID) (db.Sector, error)
	ListSectors(ctx context.Context, arg db.ListSectorsParams) ([]db.Sector, error)
	UpdateSectorAdmin(ctx context.Context, arg db.UpdateSectorAdminParams) (db.Sector, error)
}

func (r *Repository) CountSectors(ctx context.Context, query string) (int64, error) {
	return r.q.CountSectors(ctx, query)
}
func (r *Repository) CreateSectorAdmin(ctx context.Context, arg db.CreateSectorAdminParams) (db.Sector, error) {
	return r.q.CreateSectorAdmin(ctx, arg)
}
func (r *Repository) DeleteSectorAdmin(ctx context.Context, id uuid.UUID) (int64, error) {
	return r.q.DeleteSectorAdmin(ctx, id)
}
func (r *Repository) GetSectorAdmin(ctx context.Context, id uuid.UUID) (db.Sector, error) {
	return r.q.GetSectorAdmin(ctx, id)
}
func (r *Repository) ListSectors(ctx context.Context, arg db.ListSectorsParams) ([]db.Sector, error) {
	return r.q.ListSectors(ctx, arg)
}
func (r *Repository) UpdateSectorAdmin(ctx context.Context, arg db.UpdateSectorAdminParams) (db.Sector, error) {
	return r.q.UpdateSectorAdmin(ctx, arg)
}
