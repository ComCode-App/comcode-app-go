// Package usecase implements company employee asset reads and scoped card actions.
package usecase

import (
	"context"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository interface {
	InCompany(context.Context, authctx.Principal, func(*db.Queries, string) error) error
}

type Service struct {
	repo       Repository
	quotas     entitlement.Quotas
	cache      *routecache.Cache
	orders     OrderFactory
	afterOrder func(context.Context, authctx.Account, uuid.UUID)
}

func New(repo Repository, quotas entitlement.Quotas, cache *routecache.Cache) *Service {
	return &Service{repo: repo, quotas: quotas, cache: cache}
}

func admin(role string) bool {
	return role == authctx.AccountRoleOwner || role == authctx.AccountRoleAdmin
}

// CheckAccess also validates the live company membership for metadata reads.
func (s *Service) CheckAccess(ctx context.Context, p authctx.Principal, permission string) error {
	if !p.HasPermission(permission) {
		return model.ErrForbidden
	}
	return s.repo.InCompany(ctx, p, func(_ *db.Queries, _ string) error { return nil })
}

func validateFilter(f model.Filter, order bool) (model.Filter, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	if f.Sort == "" {
		f.Sort = "-created_at"
	}
	field := strings.TrimPrefix(f.Sort, "-")
	common := field == "created_at" || field == "status"
	named := (field == "name" && !order) || (field == "order_number" && order)
	if !common && !named {
		return f, &apiquery.ValidationError{Details: []apiquery.Detail{{
			Field: "sort", Message: "Expected one supported sort field", Code: "unknown",
		}}}
	}
	f.Q = strings.TrimSpace(f.Q)
	return f, nil
}

func (s *Service) Cards(ctx context.Context, p authctx.Principal, f model.Filter) ([]model.Card, int64, error) {
	out := []model.Card{}
	var total int64
	if !p.HasPermission(rbac.PermAppCardsRead) {
		return nil, 0, model.ErrForbidden
	}
	f, err := validateFilter(f, false)
	if err != nil {
		return nil, 0, err
	}
	err = s.repo.InCompany(ctx, p, func(q *db.Queries, role string) error {
		args := db.ListCompanyAssetCardsParams{
			AccountID: p.Account.ID, ActorID: p.UserInternal, ActorRole: role,
			Q: f.Q, Status: f.Status, Sort: f.Sort, OfficeUuid: nullableUUID(f.OfficeUUID), GroupUuid: nullableUUID(f.GroupUUID),
			IsActive: nullableBool(f.IsActive), LimitCount: f.Limit, OffsetCount: f.Offset,
		}
		rows, err := q.ListCompanyAssetCards(ctx, args)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, model.Card{
				UUID: r.Uuid, Name: r.Name, SmartCode: r.SmartCode, CardUID: r.CardUid, Status: r.Status,
				IsActive: r.IsActive, IsLocked: r.IsLocked, ProfileUUID: publicUUID(r.ProfileUuid),
				RouteUUID: publicUUID(r.RouteUuid), RoutePath: r.RoutePath.String, RouteStatus: r.RouteStatus.String,
				IsImmutable: r.IsImmutable.Bool, MemberUUID: publicUUID(r.MemberUuid), MemberName: r.MemberName,
				OfficeUUID: publicUUID(r.OfficeUuid), OfficeName: r.OfficeName.String,
				GroupUUID: publicUUID(r.GroupUuid), GroupName: r.GroupName.String, CreatedAt: r.CreatedAt.Time,
			})
		}
		total, err = q.CountCompanyAssetCards(ctx, db.CountCompanyAssetCardsParams{
			AccountID: args.AccountID, ActorID: args.ActorID, ActorRole: role, Q: f.Q, Status: f.Status,
			OfficeUuid: args.OfficeUuid, GroupUuid: args.GroupUuid, IsActive: args.IsActive,
		})
		return err
	})
	return out, total, err
}

func (s *Service) Orders(ctx context.Context, p authctx.Principal, f model.Filter) ([]model.Order, int64, error) {
	out := []model.Order{}
	var total int64
	if !p.HasPermission(rbac.PermAppOrdersRead) {
		return nil, 0, model.ErrForbidden
	}
	f, err := validateFilter(f, true)
	if err != nil {
		return nil, 0, err
	}
	err = s.repo.InCompany(ctx, p, func(q *db.Queries, role string) error {
		args := db.ListCompanyAssetOrdersParams{
			AccountID: p.Account.ID, ActorID: p.UserInternal, ActorRole: role,
			Q: f.Q, Status: f.Status, Sort: f.Sort, OfficeUuid: nullableUUID(f.OfficeUUID), GroupUuid: nullableUUID(f.GroupUUID),
			LimitCount: f.Limit, OffsetCount: f.Offset,
		}
		rows, err := q.ListCompanyAssetOrders(ctx, args)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, model.Order{
				UUID: r.Uuid, OrderNumber: r.OrderNumber, Status: r.Status, PaymentStatus: r.PaymentStatus,
				FulfillmentStatus: r.FulfillmentStatus, Currency: r.Currency, GrandTotal: r.GrandTotal,
				PlacedByUUID: r.PlacedByUuid, CreatedAt: r.CreatedAt.Time,
			})
		}
		total, err = q.CountCompanyAssetOrders(ctx, db.CountCompanyAssetOrdersParams{
			AccountID: args.AccountID, ActorID: args.ActorID, ActorRole: role, Q: f.Q, Status: f.Status,
			OfficeUuid: args.OfficeUuid, GroupUuid: args.GroupUuid,
		})
		return err
	})
	return out, total, err
}

func officeScope(ctx context.Context, q *db.Queries, p authctx.Principal, role string, office pgtype.UUID) error {
	if admin(role) {
		return nil
	}
	if role != authctx.AccountRoleOfficeManager || !office.Valid {
		return model.ErrNotFound
	}
	ok, err := q.StructureHasOfficeScope(ctx, db.StructureHasOfficeScopeParams{
		AccountID: p.Account.ID, UserID: p.UserInternal, Uuid: uuid.UUID(office.Bytes),
	})
	if err != nil {
		return err
	}
	if !ok {
		return model.ErrNotFound
	}
	return nil
}

// changeCard checks the source employee scope as well as lifecycle locks under row locks.
// Immutable route identity is never changed; is_active is reversible without retiring the route.
func (s *Service) changeCard(
	ctx context.Context,
	p authctx.Principal,
	id uuid.UUID,
	change func(*db.Queries, string, db.GetCompanyAssetCardForUpdateRow) error,
) error {
	if !p.HasPermission(rbac.PermAppCardsWrite) {
		return model.ErrForbidden
	}
	err := s.repo.InCompany(ctx, p, func(q *db.Queries, role string) error {
		if !admin(role) && role != authctx.AccountRoleOfficeManager {
			return model.ErrForbidden
		}
		c, err := q.GetCompanyAssetCardForUpdate(ctx, db.GetCompanyAssetCardForUpdateParams{
			AccountID: p.Account.ID, Uuid: id,
		})
		if err != nil {
			return err
		}
		if !admin(role) {
			if !c.ProfileID.Valid {
				return model.ErrNotFound
			}
			m, err := q.CompanyAssetProfileMember(ctx, db.CompanyAssetProfileMemberParams{
				AccountID: p.Account.ID, ProfileID: c.ProfileID,
			})
			if err != nil {
				return err
			}
			if err = officeScope(ctx, q, p, role, m.OfficeUuid); err != nil {
				return err
			}
		}
		if c.Status == "in_production" {
			return model.ErrConflict
		}
		if c.OrderID.Valid {
			o, err := q.LockCompanyAssetOrder(ctx, db.LockCompanyAssetOrderParams{AccountID: p.Account.ID, CardID: c.ID})
			if err != nil {
				return err
			}
			closed := o.Status == "cancelled" || o.Status == "refunded" || o.PaymentStatus == "refunded"
			if closed || o.Status == "in_production" || o.FulfillmentStatus == "in_production" {
				return model.ErrConflict
			}
		}
		r, err := q.LockCompanyAssetRoute(ctx, db.LockCompanyAssetRouteParams{
			AccountID: nullableInt(p.Account.ID), CardID: nullableInt(c.ID),
		})
		if err != nil {
			return err
		}
		if r.Status != "active" && r.Status != "locked" {
			return model.ErrConflict
		}
		return change(q, role, c)
	})
	if err == nil {
		s.cache.InvalidateCard(ctx, id)
	}
	return err
}

func (s *Service) AssignCard(ctx context.Context, p authctx.Principal, id, member uuid.UUID) error {
	if member == uuid.Nil {
		return model.ErrInvalid
	}
	return s.changeCard(ctx, p, id, func(q *db.Queries, role string, c db.GetCompanyAssetCardForUpdateRow) error {
		target, err := q.CompanyAssetProfileMember(ctx, db.CompanyAssetProfileMemberParams{
			AccountID: p.Account.ID, UserUuid: nullableUUID(&member),
		})
		if err != nil {
			return err
		}
		if err = officeScope(ctx, q, p, role, target.OfficeUuid); err != nil {
			return err
		}
		profileID := nullableInt(target.ProfileID)
		if err = q.AssignCompanyAssetCard(ctx, db.AssignCompanyAssetCardParams{
			AccountID: p.Account.ID, ID: c.ID, ProfileID: profileID,
		}); err != nil {
			return err
		}
		return q.AssignCompanyAssetRoute(ctx, db.AssignCompanyAssetRouteParams{
			AccountID: nullableInt(p.Account.ID), CardID: nullableInt(c.ID), ProfileID: profileID,
		})
	})
}

func (s *Service) SetCardActive(ctx context.Context, p authctx.Principal, id uuid.UUID, active bool) error {
	return s.changeCard(ctx, p, id, func(q *db.Queries, _ string, c db.GetCompanyAssetCardForUpdateRow) error {
		if active && !c.IsActive {
			if err := entitlement.Guard(ctx, s.quotas, p.Account.ID, entitlement.FeatureCardsMax); err != nil {
				return err
			}
		}
		return q.SetCompanyAssetCardActive(ctx, db.SetCompanyAssetCardActiveParams{
			AccountID: p.Account.ID, ID: c.ID, IsActive: active,
		})
	})
}

func nullableUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}
func nullableBool(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}
func nullableInt(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: true} }
func publicUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	v := uuid.UUID(id.Bytes)
	return &v
}
