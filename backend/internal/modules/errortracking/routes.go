package errortracking

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	errortrackinghandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/errortracking/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

// RegisterRoutes mounts the "Hata izleme" setting (super admin) and the public SDK config.
func RegisterRoutes(
	mux *http.ServeMux,
	h *errortrackinghandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}
	superAdmin := middleware.RequireSuperAdmin

	mux.Handle("GET /v1/platform/error-tracking", middleware.Chain(
		http.HandlerFunc(h.Get), authn, require(rbac.PermPlatformErrorTrackingRead), superAdmin,
	))
	mux.Handle("PATCH /v1/platform/error-tracking", middleware.Chain(
		http.HandlerFunc(h.Patch), authn, require(rbac.PermPlatformErrorTrackingWrite), superAdmin,
	))
	mux.Handle("POST /v1/platform/error-tracking/test", middleware.Chain(
		http.HandlerFunc(h.SendTest), authn, require(rbac.PermPlatformErrorTrackingWrite), superAdmin,
	))
	mux.Handle("GET /v1/public/error-tracking", http.HandlerFunc(h.PublicConfig))
}
