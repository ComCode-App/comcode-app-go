package usecase

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	notifModel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
	"github.com/google/uuid"
)

func TestOrderNotificationInput(t *testing.T) {
	cases := []struct{ name, status, tr, en string }{
		{"received", "pending", "Sipariş alındı", "Order received"},
		{"payment", "paid", "Ödeme onaylandı", "Payment confirmed"},
		{"design requested", "design_pending", "Tasarım bekleniyor", "Awaiting design"},
		{"design approved", "design_approved", "Tasarım onaylandı", "Design approved"},
		{"production", "in_production", "Üretimde", "In production"},
		{"ready", "ready_to_ship", "Kargoya hazır", "Ready to ship"},
		{"shipping", "shipped", "Kargoya verildi", "Shipped"},
		{"delivery", "delivered", "Teslim edildi", "Delivered"},
		{"cancellation", "cancelled", "İptal edildi", "Cancelled"},
		{"refund", "refunded", "İade edildi", "Refunded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, lang := range []string{"tr", "en"} {
				t.Run(lang, func(t *testing.T) {
					event := model.NotificationEvent{
						Status: tc.status, CustomerID: 42, CustomerName: "Ada Example", CustomerLocale: lang,
						Snapshot: map[string]string{"order_uuid": "order-uuid", "order_number": "ORD-123"},
					}
					got := orderNotificationInput(event, "https://app.test/")
					label := tc.tr
					if lang == "en" {
						label = tc.en
					}
					if got.TemplateCode != "orders.status."+tc.status || got.TemplateVars["status"] != label {
						t.Fatalf("template=%s status=%s", got.TemplateCode, got.TemplateVars["status"])
					}
					if got.UserID == nil || *got.UserID != 42 || got.Language != lang {
						t.Fatalf("recipient or language: %+v", got)
					}
					if !reflect.DeepEqual(got.Channels, []string{"email", "inapp"}) {
						t.Fatalf("channels=%v", got.Channels)
					}
					if got.SourceEvent != "orders.status:order-uuid:"+tc.status {
						t.Fatalf("dedupe key=%s", got.SourceEvent)
					}
					if got.ActionURL == nil || *got.ActionURL != "https://app.test/orders" ||
						got.TemplateVars["order_number"] != "ORD-123" || got.TemplateVars["customer_name"] != "Ada Example" ||
						got.TemplateVars["tracking_info"] != "" {
						t.Fatalf("variables/action=%+v", got)
					}
				})
			}
		})
	}
}

func TestOrderNotificationLanguage(t *testing.T) {
	for _, tc := range []struct{ name, locale, want string }{
		{"Turkish", "tr", "tr"}, {"English", "en", "en"},
		{"normalizes", " EN ", "en"}, {"missing", "", "tr"}, {"unsupported", "de", "tr"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := orderNotificationLanguage(tc.locale); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestOrderNotificationTracking(t *testing.T) {
	event := model.NotificationEvent{
		Status: "shipped", CustomerLocale: "en", Snapshot: map[string]string{
			"shipping_carrier": "Carrier", "tracking_number": "TRACK-1", "tracking_url": "https://carrier.test/1",
		},
	}
	got := orderNotificationInput(event, "https://app.test")
	if got.TemplateVars["tracking_info"] != "Tracking: Carrier TRACK-1 https://carrier.test/1" {
		t.Fatalf("tracking=%q", got.TemplateVars["tracking_info"])
	}
	event.CustomerLocale = "tr"
	if got := orderNotificationInput(event, ""); !strings.HasPrefix(got.TemplateVars["tracking_info"], "Kargo takibi:") {
		t.Fatalf("tracking=%q", got.TemplateVars["tracking_info"])
	}
}

type notificationFailureRepo struct {
	*statusRepo
	marked bool
}

func (r *notificationFailureRepo) ListPendingNotifications(context.Context, int64, int64) ([]model.NotificationEvent, error) {
	return []model.NotificationEvent{{ID: 1, Status: "design_pending", Snapshot: map[string]string{}}}, nil
}
func (r *notificationFailureRepo) MarkNotificationDispatched(context.Context, int64, int64, int64) error {
	r.marked = true
	return nil
}

type failingOrderNotifier struct{}

func (failingOrderNotifier) Enqueue(context.Context, notifModel.EnqueueInput) ([]notifModel.Notification, error) {
	return nil, errors.New("queue unavailable")
}

func TestOrderNotificationFailureDoesNotFailTransition(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	repo := &notificationFailureRepo{statusRepo: &statusRepo{order: model.OrderDetail{
		ID: 1, AccountID: 2, UUID: uuid.NewString(), Status: "pending",
	}}}
	svc := NewService(repo, nil, failingOrderNotifier{}, nil, "https://app.test")
	ctx := reqid.With(context.Background(), "tec584-failure")
	ord, err := svc.UpdateStatus(ctx, uuid.New(), model.UpdateOrderStatusInput{Status: "design_pending"})
	if err != nil || ord.Status != "design_pending" || repo.marked {
		t.Fatalf("transition err=%v order=%+v marked=%v", err, ord, repo.marked)
	}
	if !strings.Contains(logs.String(), `"request_id":"tec584-failure"`) ||
		!strings.Contains(logs.String(), "order_notification_enqueue_failed") {
		t.Fatalf("missing correlated failure log: %s", logs.String())
	}
}
