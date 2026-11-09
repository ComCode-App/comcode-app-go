package payment

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	paymenthandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/payment/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *paymenthandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}
	superAdmin := middleware.RequireSuperAdmin

	mux.Handle("GET /v1/platform/payment/settings", middleware.Chain(
		http.HandlerFunc(h.GetSettings), authn, require(rbac.PermPlatformPaymentRead), superAdmin,
	))
	mux.Handle("PATCH /v1/platform/payment/settings", middleware.Chain(
		http.HandlerFunc(h.PatchSettings), authn, require(rbac.PermPlatformPaymentWrite), superAdmin,
	))
	mux.Handle("GET /v1/platform/payment/providers/paytr", middleware.Chain(
		http.HandlerFunc(h.GetPayTR), authn, require(rbac.PermPlatformPaymentRead), superAdmin,
	))
	mux.Handle("PATCH /v1/platform/payment/providers/paytr", middleware.Chain(
		http.HandlerFunc(h.PatchPayTR), authn, require(rbac.PermPlatformPaymentWrite), superAdmin,
	))
	mux.Handle("GET /v1/platform/payment/providers/iyzico", middleware.Chain(
		http.HandlerFunc(h.GetIyzico), authn, require(rbac.PermPlatformPaymentRead), superAdmin,
	))
	mux.Handle("PATCH /v1/platform/payment/providers/iyzico", middleware.Chain(
		http.HandlerFunc(h.PatchIyzico), authn, require(rbac.PermPlatformPaymentWrite), superAdmin,
	))
}
