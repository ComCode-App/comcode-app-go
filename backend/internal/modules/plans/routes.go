package plans

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	planshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/plans/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *planshandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}

	mux.Handle("GET /v1/platform/plans/meta", middleware.Chain(
		http.HandlerFunc(h.Meta), authn, require(rbac.PermPlatformPlansRead),
	))
	mux.Handle("GET /v1/platform/feature-definitions", middleware.Chain(
		http.HandlerFunc(h.ListFeatures), authn, require(rbac.PermPlatformPlansRead),
	))
	mux.Handle("GET /v1/platform/plans", middleware.Chain(
		http.HandlerFunc(h.List), authn, require(rbac.PermPlatformPlansRead),
	))
	mux.Handle("POST /v1/platform/plans", middleware.Chain(
		http.HandlerFunc(h.Create), authn, require(rbac.PermPlatformPlansWrite),
	))
	mux.Handle("GET /v1/platform/plans/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Get), authn, require(rbac.PermPlatformPlansRead),
	))
	mux.Handle("PATCH /v1/platform/plans/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Patch), authn, require(rbac.PermPlatformPlansWrite),
	))
	mux.Handle("DELETE /v1/platform/plans/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Delete), authn, require(rbac.PermPlatformPlansWrite),
	))
}
