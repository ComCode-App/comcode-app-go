package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	notifModel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
)

type orderNotifier interface {
	Enqueue(context.Context, notifModel.EnqueueInput) ([]notifModel.Notification, error)
}

// Webhooks fans a domain event out to the account's webhook endpoints (TEC-603).
type Webhooks interface {
	Dispatch(ctx context.Context, accountID int64, eventName string, data map[string]any)
}

// SetWebhooks enables order.created / order.status_changed webhooks.
func (s *Service) SetWebhooks(w Webhooks) { s.webhooks = w }

// emitOrderWebhook sends order.created for the first milestone and order.status_changed for the
// rest; both carry the same order snapshot.
func (s *Service) emitOrderWebhook(ctx context.Context, ord *model.OrderDetail, milestone string) {
	if s.webhooks == nil {
		return
	}
	name := "order.status_changed"
	if milestone == "pending" {
		name = "order.created"
	}
	items := make([]map[string]any, 0, len(ord.Items))
	for _, it := range ord.Items {
		items = append(items, map[string]any{"uuid": it.UUID, "product_name": it.ProductName, "quantity": it.Quantity, "recipient_name": it.RecipientName})
	}
	s.webhooks.Dispatch(ctx, ord.AccountID, name, map[string]any{
		"order_uuid": ord.UUID, "order_number": ord.OrderNumber, "order_type": ord.OrderType,
		"milestone": milestone, "status": ord.Status, "payment_status": ord.PaymentStatus,
		"grand_total": ord.GrandTotal, "currency": ord.Currency,
		"shipping_carrier": ord.ShippingCarrier, "tracking_number": ord.TrackingNumber, "tracking_url": ord.TrackingURL,
		"items": items, "created_at": ord.CreatedAt.UTC().Format(time.RFC3339),
	})
}

// Milestones shared by status and payment changes use the same template and dedupe key.
var orderStatusLabels = map[string][2]string{
	"pending":         {"Sipariş alındı", "Order received"},
	"paid":            {"Ödeme onaylandı", "Payment confirmed"},
	"design_pending":  {"Tasarım bekleniyor", "Awaiting design"},
	"design_approved": {"Tasarım onaylandı", "Design approved"},
	"in_production":   {"Üretimde", "In production"},
	"ready_to_ship":   {"Kargoya hazır", "Ready to ship"},
	"shipped":         {"Kargoya verildi", "Shipped"},
	"delivered":       {"Teslim edildi", "Delivered"},
	"cancelled":       {"İptal edildi", "Cancelled"},
	"refunded":        {"İade edildi", "Refunded"},
}

func orderNotificationLanguage(locale string) string {
	if strings.EqualFold(strings.TrimSpace(locale), "en") {
		return "en"
	}
	return "tr"
}

func orderNotificationInput(event model.NotificationEvent, frontendURL string) notifModel.EnqueueInput {
	lang := orderNotificationLanguage(event.CustomerLocale)
	label := event.Status
	if labels, ok := orderStatusLabels[event.Status]; ok {
		label = labels[0]
		if lang == "en" {
			label = labels[1]
		}
	}
	vars := make(map[string]string, len(event.Snapshot)+4)
	for k, v := range event.Snapshot {
		vars[k] = v
	}
	vars["status"] = label
	vars["customer_name"] = event.CustomerName
	// The existing customer page has no detail route or query-param deep link.
	vars["order_url"] = strings.TrimRight(frontendURL, "/") + "/orders"
	tracking := strings.TrimSpace(strings.Join([]string{
		vars["shipping_carrier"], vars["tracking_number"], vars["tracking_url"],
	}, " "))
	vars["tracking_info"] = ""
	if tracking != "" {
		prefix := "Kargo takibi: "
		if lang == "en" {
			prefix = "Tracking: "
		}
		vars["tracking_info"] = prefix + tracking
	}
	actionURL := vars["order_url"]
	return notifModel.EnqueueInput{
		UserID:   &event.CustomerID,
		Channels: []string{notifModel.ChannelEmail, notifModel.ChannelInapp},
		Language: lang, TemplateCode: "orders.status." + event.Status, TemplateVars: vars,
		// Fallback content also supports a future status before its templates are seeded.
		Title:       vars["order_number"] + ": " + label,
		Body:        event.CustomerName + ", " + label + ". " + vars["tracking_info"] + " " + actionURL,
		ActionURL:   &actionURL,
		SourceEvent: "orders.status:" + vars["order_uuid"] + ":" + event.Status,
		Payload: map[string]any{
			"order_uuid": vars["order_uuid"], "order_number": vars["order_number"],
			"status": event.Status, "shipping_carrier": vars["shipping_carrier"],
			"tracking_number": vars["tracking_number"], "tracking_url": vars["tracking_url"],
		},
	}
}

// notifyOrderTransitions reads only committed events after a repository mutation.
// Failed/partial enqueues remain pending and are retried on the next mutation of this order.
func (s *Service) notifyOrderTransitions(ctx context.Context, ord *model.OrderDetail) {
	if s.notifSvc == nil || ord == nil {
		return
	}
	// Keep request correlation, but finish the committed side effect if the client disconnects.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.dispatchOrderNotifications(ctx, ord); err != nil {
		slog.ErrorContext(ctx, "order_notification_enqueue_failed", "request_id", reqid.From(ctx),
			"account_id", ord.AccountID, "order_uuid", ord.UUID, "error", err)
	}
}

func (s *Service) dispatchOrderNotifications(ctx context.Context, ord *model.OrderDetail) error {
	events, err := s.repo.ListPendingNotifications(ctx, ord.AccountID, ord.ID)
	if err != nil {
		return fmt.Errorf("list order notification events: %w", err)
	}
	for _, event := range events {
		if _, err := s.notifSvc.Enqueue(ctx, orderNotificationInput(event, s.frontendURL)); err != nil {
			return fmt.Errorf("enqueue order milestone %s: %w", event.Status, err)
		}
		// After the notification is queued: a failed enqueue re-runs this milestone later, and the
		// webhook must not go out twice.
		s.emitOrderWebhook(ctx, ord, event.Status)
		if err := s.repo.MarkNotificationDispatched(ctx, ord.AccountID, ord.ID, event.ID); err != nil {
			return fmt.Errorf("mark order notification dispatched: %w", err)
		}
	}
	return nil
}
