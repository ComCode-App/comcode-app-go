package subscriptions

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	subscriptionshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/subscriptions/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *subscriptionshandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}

	mux.Handle("GET /v1/platform/subscriptions/meta", middleware.Chain(
		http.HandlerFunc(h.Meta), authn, require(rbac.PermPlatformSubscriptionsRead),
	))
	mux.Handle("GET /v1/platform/subscriptions", middleware.Chain(
		http.HandlerFunc(h.List), authn, require(rbac.PermPlatformSubscriptionsRead),
	))
	mux.Handle("POST /v1/platform/subscriptions", middleware.Chain(
		http.HandlerFunc(h.Create), authn, require(rbac.PermPlatformSubscriptionsWrite),
	))
	mux.Handle("GET /v1/platform/subscriptions/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Get), authn, require(rbac.PermPlatformSubscriptionsRead),
	))
	mux.Handle("PATCH /v1/platform/subscriptions/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Patch), authn, require(rbac.PermPlatformSubscriptionsWrite),
	))
}
