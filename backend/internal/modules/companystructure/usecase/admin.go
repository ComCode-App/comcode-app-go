package usecase

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
)

// AdminService reads arbitrary companies through separate repository entry points.
type AdminService struct{ repo *repository.Repository }

func NewAdmin(r *repository.Repository) *AdminService { return &AdminService{repo: r} }
func (s *AdminService) Offices(ctx context.Context, p authctx.Principal, account uuid.UUID, limit, offset int32) ([]model.Office, int64, error) {
	out := []model.Office{}
	var total int64
	if !p.HasPermission(rbac.PermPlatformAccountsRead) {
		return out, 0, model.ErrForbidden
	}
	err := s.repo.ReadAdmin(ctx, account, func(q repository.Store, id int64) error {
		rows, err := q.ListStructureOffices(ctx, db.ListStructureOfficesParams{AccountID: id, LimitCount: limit, OffsetCount: offset})
		if err != nil {
			return err
		}
		for _, v := range rows {
			out = append(out, officeModel(v))
		}
		total, err = q.CountStructureOffices(ctx, db.CountStructureOfficesParams{AccountID: id})
		return err
	})
	return out, total, err
}
func (s *AdminService) Groups(ctx context.Context, p authctx.Principal, account uuid.UUID, limit, offset int32) ([]model.Group, int64, error) {
	out := []model.Group{}
	var total int64
	if !p.HasPermission(rbac.PermPlatformAccountsRead) {
		return out, 0, model.ErrForbidden
	}
	err := s.repo.ReadAdmin(ctx, account, func(q repository.Store, id int64) error {
		rows, err := q.ListStructureGroups(ctx, db.ListStructureGroupsParams{AccountID: id, LimitCount: limit, OffsetCount: offset})
		if err != nil {
			return err
		}
		for _, v := range rows {
			out = append(out, model.Group{UUID: v.Uuid, GroupInput: model.GroupInput{Name: v.Name, Kind: v.Kind, OfficeUUID: publicID(v.OfficeUuid)}, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time})
		}
		total, err = q.CountStructureGroups(ctx, db.CountStructureGroupsParams{AccountID: id})
		return err
	})
	return out, total, err
}
