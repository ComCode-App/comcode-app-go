package billing

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	billinghandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/stepup"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *billinghandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	requireSession := middleware.RequirePermission(rbac.PermAuthSession)

	mux.Handle("GET /v1/billing/subscription", middleware.Chain(
		http.HandlerFunc(h.GetSubscription), authn, requireSession,
	))
	mux.Handle("DELETE /v1/billing/subscription", middleware.Chain(
		http.HandlerFunc(h.CancelSubscription), authn, requireSession, middleware.RequirePermission(rbac.PermAppBillingWrite),
	))
	mux.Handle("POST /v1/billing/subscription/resume", middleware.Chain(
		http.HandlerFunc(h.ResumeSubscription), authn, requireSession, middleware.RequirePermission(rbac.PermAppBillingWrite),
	))
	mux.Handle("PATCH /v1/billing/subscription/plan", middleware.Chain(
		http.HandlerFunc(h.ChangePlan), authn, requireSession, middleware.RequirePermission(rbac.PermAppBillingWrite),
	))

	mux.Handle("POST /v1/billing/subscribe", middleware.Chain(
		http.HandlerFunc(h.Subscribe), authn, requireSession,
	))
	mux.Handle("GET /v1/billing/cards", middleware.Chain(
		http.HandlerFunc(h.ListCards), authn, requireSession,
	))
	mux.Handle("POST /v1/billing/cards", middleware.Chain(
		http.HandlerFunc(h.StartAddCard), authn, requireSession,
	))
	mux.Handle("GET /v1/billing/cards/add/{uuid}", middleware.Chain(
		http.HandlerFunc(h.GetAddCardSession), authn, requireSession,
	))
	mux.Handle("DELETE /v1/billing/cards/{uuid}", middleware.Chain(
		http.HandlerFunc(h.DeleteCard), authn, requireSession,
	))
	mux.Handle("PATCH /v1/billing/cards/{uuid}/default", middleware.Chain(
		http.HandlerFunc(h.SetDefaultCard), authn, requireSession,
	))
	mux.Handle("GET /v1/billing/plans", middleware.Chain(
		http.HandlerFunc(h.ListPlans), authn, requireSession,
	))
	mux.Handle("POST /v1/billing/checkout", middleware.Chain(
		http.HandlerFunc(h.StartCheckout), authn, requireSession,
	))
	mux.Handle("GET /v1/billing/checkout/{uuid}", middleware.Chain(
		http.HandlerFunc(h.GetCheckout), authn, requireSession,
	))
	// Payment history and PDF receipts of the active account (TEC-590): any member.
	mux.Handle("GET /v1/billing/payments", middleware.Chain(
		http.HandlerFunc(h.ListPayments), authn, requireSession,
	))
	mux.Handle("GET /v1/billing/payments/{uuid}/receipt", middleware.Chain(
		http.HandlerFunc(h.DownloadReceipt), authn, requireSession,
	))
	// Card order payments (TEC-943): owner/admin of the active account, checked in the usecase.
	mux.Handle("POST /v1/orders/{uuid}/checkout", middleware.Chain(
		http.HandlerFunc(h.StartOrderCheckout), authn, middleware.RequirePermission(rbac.PermAppOrdersWrite),
	))
	mux.Handle("GET /v1/orders/checkout/{uuid}", middleware.Chain(
		http.HandlerFunc(h.GetOrderCheckout), authn, middleware.RequirePermission(rbac.PermAppOrdersRead),
	))
	mux.Handle("POST /v1/public/payment/paytr/callback", http.HandlerFunc(h.PayTRCallback))
	mux.Handle("GET /v1/public/payment/iyzico/callback", http.HandlerFunc(h.IyzicoCallback))
	mux.Handle("POST /v1/public/payment/iyzico/callback", http.HandlerFunc(h.IyzicoCallback))
}

// RegisterRefundRoutes mounts the platform order refund routes (TEC-935): reads need
// platform.orders.read; refunding needs platform.orders.write and a step-up grant.
func RegisterRefundRoutes(mux *http.ServeMux, h *billinghandler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader, stepUp *stepup.Service) {
	requireStepUp := func(next http.Handler) http.Handler { return next }
	if stepUp != nil {
		requireStepUp = middleware.RequireStepUp(stepUp)
	}
	RegisterRefunds(mux, h, middleware.Authenticate(tokens, loader), requireStepUp)
}

// RegisterRefunds mounts the refund routes behind the given authentication and step-up middleware.
func RegisterRefunds(mux *http.ServeMux, h *billinghandler.Handler, authn, requireStepUp func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/platform/orders/{uuid}/refunds", middleware.Chain(
		http.HandlerFunc(h.ListOrderRefunds), authn, middleware.RequirePermission(rbac.PermPlatformOrdersRead),
	))
	mux.Handle("POST /v1/platform/orders/{uuid}/refunds", middleware.Chain(
		http.HandlerFunc(h.RefundOrder), authn, middleware.RequirePermission(rbac.PermPlatformOrdersWrite), requireStepUp,
	))
	mux.Handle("POST /v1/platform/orders/{uuid}/refunds/{refundUuid}/retry", middleware.Chain(
		http.HandlerFunc(h.RetryOrderRefund), authn, middleware.RequirePermission(rbac.PermPlatformOrdersWrite), requireStepUp,
	))
}
