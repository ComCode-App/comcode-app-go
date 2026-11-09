package usecase

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/model"
	orders "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
)

// OrderCreator is the existing order creation flow, bound to the company transaction.
type OrderCreator interface {
	CreateForAccount(context.Context, authctx.Account, int64, orders.CreateOrderInput) (*orders.OrderDetail, error)
}
type OrderFactory func(*db.Queries) OrderCreator

// SetOrders supplies a transaction-bound order flow and publishes notifications/search only
// after commit. The bound service must have no external notification or search side effects.
func (s *Service) SetOrders(
	factory OrderFactory,
	afterCommit func(context.Context, authctx.Account, uuid.UUID),
) {
	s.orders = factory
	s.afterOrder = afterCommit
}

func (s *Service) Reorder(ctx context.Context, p authctx.Principal, in model.ReorderInput) (*orders.OrderDetail, error) {
	if !p.HasPermission(rbac.PermAppOrdersWrite) {
		return nil, model.ErrForbidden
	}
	if len(in.MemberUUIDs) == 0 || len(in.MemberUUIDs) > 100 || in.ProductUUID == uuid.Nil {
		return nil, model.ErrInvalid
	}
	var out *orders.OrderDetail
	err := s.repo.InCompany(ctx, p, func(q *db.Queries, role string) error {
		// Placing an order spends company funds; office scope grants no billing/order-write role.
		if !admin(role) {
			return model.ErrForbidden
		}
		if s.orders == nil {
			return model.ErrForbidden
		}
		product, err := q.GetProductByUUID(ctx, in.ProductUUID)
		if err != nil {
			return err
		}
		if product.Status != "active" {
			return model.ErrInvalid
		}
		var variant *string
		if in.VariantUUID != nil {
			v, err := q.GetProductVariantByUUID(ctx, *in.VariantUUID)
			if err != nil {
				return err
			}
			if v.ProductID != product.ID {
				return model.ErrInvalid
			}
			id := in.VariantUUID.String()
			variant = &id
		}
		items := make([]orders.CreateOrderItemInput, 0, len(in.MemberUUIDs))
		seen := make(map[uuid.UUID]bool, len(in.MemberUUIDs))
		for _, id := range in.MemberUUIDs {
			if id == uuid.Nil || seen[id] {
				return model.ErrInvalid
			}
			seen[id] = true
			m, err := q.CompanyAssetProfileMember(ctx, db.CompanyAssetProfileMemberParams{
				AccountID: p.Account.ID, UserUuid: nullableUUID(&id),
			})
			if err != nil {
				return err
			}
			profile := m.ProfileUuid.String()
			items = append(items, orders.CreateOrderItemInput{
				ProductUUID: in.ProductUUID.String(), VariantUUID: variant, Quantity: 1, ProfileUUID: &profile,
				RecipientName: m.Name + " " + m.Surname, RecipientEmail: m.Email, RecipientDepartment: m.GroupName,
			})
		}
		account := p.Account
		account.Role = role
		out, err = s.orders(q).CreateForAccount(ctx, account, p.UserInternal, orders.CreateOrderInput{
			Items: items, ShippingAddress: in.ShippingAddress, BillingAddress: in.BillingAddress, CustomerNote: in.CustomerNote,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	if s.afterOrder != nil {
		s.afterOrder(context.WithoutCancel(ctx), p.Account, uuid.MustParse(out.UUID))
	}
	return out, nil
}
