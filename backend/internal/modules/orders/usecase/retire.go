package usecase

import (
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
)

// SetRouteCache lets retirement drop the resolver cache entries of the retired cards (TEC-585).
func (s *Service) SetRouteCache(c *routecache.Cache) { s.routeCache = c }

// isClosedOrder reports whether the order was cancelled or its payment refunded. Its cards are
// retired and it can no longer be provisioned.
func isClosedOrder(ord *model.OrderDetail) bool {
	return ord.Status == "cancelled" || ord.Status == "refunded" || ord.PaymentStatus == "refunded"
}

// retireCardRoutes retires every card route of the order (architecture decision #6): the card
// shows "card deactivated" and its path is never handed out again. Retirement is one-way; moving
// the order back to another status does not revive the cards.
func (s *Service) retireCardRoutes(ctx context.Context, orderID int64) error {
	retired, err := s.repo.RetireCardRoutes(ctx, orderID)
	if err != nil {
		return err
	}
	for _, r := range retired {
		s.routeCache.InvalidatePaths(ctx, r.Path)
		s.routeCache.InvalidateCard(ctx, r.CardUUID)
	}
	return nil
}

// SetSearchIndexer keeps orders in the Cmd+K index (TEC-595).
func (s *Service) SetSearchIndexer(e searchengine.Enqueuer) { s.search = e }

// indexed passes a create/update result through and queues the order for search on success.
func (s *Service) indexed(ord *model.OrderDetail, err error) (*model.OrderDetail, error) {
	if err == nil && ord != nil && s.search != nil {
		s.search.EnqueueUpsert(context.Background(), "orders", ord.UUID)
	}
	return ord, err
}
