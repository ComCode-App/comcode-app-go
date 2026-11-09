package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
)

type retireRepo struct {
	repository.Repository
	order   *model.OrderDetail
	retired []model.RetiredCardRoute
	calls   int
}

func (r *retireRepo) GetByUUID(context.Context, uuid.UUID) (*model.OrderDetail, error) {
	return r.order, nil
}

func (r *retireRepo) UpdateStatus(_ context.Context, _ uuid.UUID, status string, _ *int64) (*model.OrderDetail, error) {
	r.order.Status = status
	return r.order, nil
}

func (r *retireRepo) UpdatePaymentStatus(_ context.Context, _ uuid.UUID, status string, _, _ *string, _ *int64) (*model.OrderDetail, error) {
	r.order.PaymentStatus = status
	return r.order, nil
}

func (r *retireRepo) RetireCardRoutes(context.Context, int64) ([]model.RetiredCardRoute, error) {
	r.calls++
	return r.retired, nil
}

func TestCancelRetiresCardRoutesAndDropsCache(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := routecache.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test", 0)
	ctx := context.Background()
	card := uuid.New()
	rc.Set(ctx, "c/abc", []byte(`{}`), &card, nil)

	repo := &retireRepo{
		order:   &model.OrderDetail{ID: 9, Status: "paid", PaymentStatus: "paid"},
		retired: []model.RetiredCardRoute{{Path: "c/abc", CardUUID: card}},
	}
	svc := NewService(repo, nil, nil, nil, "https://comcode.test")
	svc.SetRouteCache(rc)

	if _, err := svc.UpdateStatus(ctx, uuid.New(), model.UpdateOrderStatusInput{Status: "in_production"}); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 0 {
		t.Fatal("open orders must not retire cards")
	}
	if _, err := svc.UpdateStatus(ctx, uuid.New(), model.UpdateOrderStatusInput{Status: "cancelled"}); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 {
		t.Fatalf("retire calls = %d", repo.calls)
	}
	if _, ok := rc.Get(ctx, "c/abc"); ok {
		t.Fatal("retired card must be dropped from the resolver cache")
	}
	if _, err := svc.ProvisionOrder(ctx, uuid.New()); !errors.Is(err, ErrOrderClosed) {
		t.Fatalf("provisioning a cancelled order: %v", err)
	}
}

func TestRefundRetiresCardRoutes(t *testing.T) {
	repo := &retireRepo{order: &model.OrderDetail{ID: 3, Status: "delivered", PaymentStatus: "paid"}}
	svc := NewService(repo, nil, nil, nil, "https://comcode.test")
	if _, err := svc.UpdatePaymentStatus(context.Background(), uuid.New(), model.UpdateOrderPaymentStatusInput{PaymentStatus: "refunded"}); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 {
		t.Fatalf("refund must retire cards, calls = %d", repo.calls)
	}
	if _, err := svc.ProvisionSingleItem(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrOrderClosed) {
		t.Fatalf("provisioning a refunded order: %v", err)
	}
}
