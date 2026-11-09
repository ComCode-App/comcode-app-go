package bankaccounts

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	bankaccountshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *bankaccountshandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}

	mux.Handle("GET /v1/bank-accounts/meta", middleware.Chain(
		http.HandlerFunc(h.Meta), authn, require(rbac.PermAppBankAccountsRead),
	))
	mux.Handle("GET /v1/bank-accounts", middleware.Chain(
		http.HandlerFunc(h.List), authn, require(rbac.PermAppBankAccountsRead),
	))
	mux.Handle("POST /v1/bank-accounts", middleware.Chain(
		http.HandlerFunc(h.Create), authn, require(rbac.PermAppBankAccountsWrite),
	))
	mux.Handle("POST /v1/bank-accounts/reorder", middleware.Chain(
		http.HandlerFunc(h.Reorder), authn, require(rbac.PermAppBankAccountsWrite),
	))
	mux.Handle("GET /v1/bank-accounts/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Get), authn, require(rbac.PermAppBankAccountsRead),
	))
	mux.Handle("PUT /v1/bank-accounts/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Update), authn, require(rbac.PermAppBankAccountsWrite),
	))
	mux.Handle("PATCH /v1/bank-accounts/{uuid}/toggle-active", middleware.Chain(
		http.HandlerFunc(h.ToggleActive), authn, require(rbac.PermAppBankAccountsWrite),
	))
	mux.Handle("DELETE /v1/bank-accounts/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Delete), authn, require(rbac.PermAppBankAccountsWrite),
	))
}
