package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
)

// statusRepo is a concurrency-safe fake: "paid" provisions in a background goroutine.
type statusRepo struct {
	repository.Repository
	mu        sync.Mutex
	order     model.OrderDetail
	retires   int
	provision int // GetByUUID calls, made only by the provisioning goroutine here
}

func (r *statusRepo) snapshot() *model.OrderDetail {
	o := r.order
	return &o
}

func (r *statusRepo) UpdateStatus(_ context.Context, _ uuid.UUID, status string, _ *int64) (*model.OrderDetail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order.Status = status
	return r.snapshot(), nil
}

func (r *statusRepo) UpdatePaymentStatus(_ context.Context, _ uuid.UUID, status string, _, _ *string, _ *int64) (*model.OrderDetail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order.PaymentStatus = status
	return r.snapshot(), nil
}

func (r *statusRepo) GetByUUID(context.Context, uuid.UUID) (*model.OrderDetail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.provision++
	return r.snapshot(), nil
}

func (r *statusRepo) RetireCardRoutes(context.Context, int64) ([]model.RetiredCardRoute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retires++
	return nil, nil
}

func (r *statusRepo) counts() (retires, provision int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.retires, r.provision
}

type recordingIndexer struct {
	mu      sync.Mutex
	upserts []string
}

func (i *recordingIndexer) EnqueueUpsert(_ context.Context, spec, id string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.upserts = append(i.upserts, spec+":"+id)
}

func (i *recordingIndexer) EnqueueDelete(context.Context, string, string) {}

func waitFor(t *testing.T, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

// TestOrderStatusSideEffects pins what each order/payment status change triggers: card
// retirement for closed orders, background provisioning on payment, and search re-indexing.
func TestOrderStatusSideEffects(t *testing.T) {
	cases := []struct {
		name          string
		payment       bool // UpdatePaymentStatus instead of UpdateStatus
		value         string
		start         model.OrderDetail
		wantRetire    bool
		wantProvision bool
	}{
		{name: "pending", value: "pending", start: model.OrderDetail{Status: "paid", PaymentStatus: "paid"}},
		{name: "paid provisions", value: "paid", start: model.OrderDetail{Status: "pending", PaymentStatus: "paid"}, wantProvision: true},
		{name: "in_production", value: "in_production", start: model.OrderDetail{Status: "paid", PaymentStatus: "paid"}},
		{name: "ready_to_ship", value: "ready_to_ship", start: model.OrderDetail{Status: "in_production", PaymentStatus: "paid"}},
		{name: "delivered", value: "delivered", start: model.OrderDetail{Status: "ready_to_ship", PaymentStatus: "paid"}},
		{name: "cancelled retires", value: "cancelled", start: model.OrderDetail{Status: "paid", PaymentStatus: "paid"}, wantRetire: true},
		{name: "refunded status retires", value: "refunded", start: model.OrderDetail{Status: "delivered", PaymentStatus: "paid"}, wantRetire: true},
		{name: "status change on refunded payment retires", value: "in_production", start: model.OrderDetail{Status: "paid", PaymentStatus: "refunded"}, wantRetire: true},
		{name: "payment paid provisions", payment: true, value: "paid", start: model.OrderDetail{Status: "pending", PaymentStatus: "unpaid"}, wantProvision: true},
		{name: "payment failed", payment: true, value: "failed", start: model.OrderDetail{Status: "pending", PaymentStatus: "unpaid"}},
		{name: "payment refunded retires", payment: true, value: "refunded", start: model.OrderDetail{Status: "delivered", PaymentStatus: "paid"}, wantRetire: true},
		{name: "payment on cancelled order retires", payment: true, value: "pending", start: model.OrderDetail{Status: "cancelled", PaymentStatus: "unpaid"}, wantRetire: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := tc.start
			start.ID, start.UUID = 7, uuid.NewString()
			repo := &statusRepo{order: start}
			idx := &recordingIndexer{}
			svc := NewService(repo, nil, nil, nil, "https://comcode.test")
			svc.SetSearchIndexer(idx)

			var err error
			if tc.payment {
				_, err = svc.UpdatePaymentStatus(context.Background(), uuid.New(), model.UpdateOrderPaymentStatusInput{PaymentStatus: tc.value})
			} else {
				_, err = svc.UpdateStatus(context.Background(), uuid.New(), model.UpdateOrderStatusInput{Status: tc.value})
			}
			if err != nil {
				t.Fatal(err)
			}

			if retires, _ := repo.counts(); (retires == 1) != tc.wantRetire || retires > 1 {
				t.Fatalf("retire calls = %d, want retire=%v", retires, tc.wantRetire)
			}
			if tc.wantProvision {
				if !waitFor(t, func() bool { _, p := repo.counts(); return p > 0 }) {
					t.Fatal("payment must start provisioning in the background")
				}
			} else {
				time.Sleep(20 * time.Millisecond)
				if _, p := repo.counts(); p != 0 {
					t.Fatalf("unexpected provisioning (%d lookups)", p)
				}
			}
			idx.mu.Lock()
			defer idx.mu.Unlock()
			if len(idx.upserts) != 1 || idx.upserts[0] != "orders:"+start.UUID {
				t.Fatalf("search upserts = %v, want the order re-indexed once", idx.upserts)
			}
		})
	}
}

func TestIsClosedOrder(t *testing.T) {
	for _, tc := range []struct {
		status, payment string
		want            bool
	}{
		{"pending", "unpaid", false},
		{"paid", "paid", false},
		{"delivered", "partially_refunded", false},
		{"cancelled", "unpaid", true},
		{"refunded", "paid", true},
		{"delivered", "refunded", true},
	} {
		if got := isClosedOrder(&model.OrderDetail{Status: tc.status, PaymentStatus: tc.payment}); got != tc.want {
			t.Errorf("isClosedOrder(%s, %s) = %v, want %v", tc.status, tc.payment, got, tc.want)
		}
	}
}
