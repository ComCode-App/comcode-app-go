package banks

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	bankshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/banks/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *bankshandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}

	// Active list for subscribers/all authenticated users (and public lookup)
	mux.Handle("GET /v1/banks", middleware.Chain(
		http.HandlerFunc(h.ListActive), authn,
	))

	// Platform Admin CRUD
	mux.Handle("GET /v1/platform/banks/meta", middleware.Chain(
		http.HandlerFunc(h.Meta), authn, require(rbac.PermPlatformBanksRead),
	))
	mux.Handle("GET /v1/platform/banks", middleware.Chain(
		http.HandlerFunc(h.List), authn, require(rbac.PermPlatformBanksRead),
	))
	mux.Handle("POST /v1/platform/banks", middleware.Chain(
		http.HandlerFunc(h.Create), authn, require(rbac.PermPlatformBanksWrite),
	))
	mux.Handle("GET /v1/platform/banks/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Get), authn, require(rbac.PermPlatformBanksRead),
	))
	mux.Handle("PUT /v1/platform/banks/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Update), authn, require(rbac.PermPlatformBanksWrite),
	))
	mux.Handle("POST /v1/platform/banks/{uuid}/icon", middleware.Chain(
		http.HandlerFunc(h.UploadIcon), authn, require(rbac.PermPlatformBanksWrite),
	))
	mux.Handle("DELETE /v1/platform/banks/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Delete), authn, require(rbac.PermPlatformBanksWrite),
	))
}
