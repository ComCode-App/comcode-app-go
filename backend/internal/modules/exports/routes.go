package exports

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	exporthandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/exports/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/stepup"
)

// RegisterRoutes mounts export routes.
func RegisterRoutes(
	mux *http.ServeMux,
	h *exporthandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
	stepUp *stepup.Service,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}
	var requireStepUp func(http.Handler) http.Handler
	if stepUp != nil {
		requireStepUp = middleware.RequireStepUp(stepUp)
	} else {
		requireStepUp = func(next http.Handler) http.Handler { return next }
	}

	mux.Handle("GET /v1/platform/exports", middleware.Chain(
		http.HandlerFunc(h.List), authn, require(rbac.PermPlatformExportsRead),
	))
	mux.Handle("GET /v1/platform/exports/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Get), authn, require(rbac.PermPlatformExportsRead),
	))
	mux.Handle("GET /v1/platform/exports/{uuid}/download", middleware.Chain(
		http.HandlerFunc(h.Download), authn, require(rbac.PermPlatformExportsRead),
	))

	mux.Handle("POST /v1/platform/users/export", middleware.Chain(
		http.HandlerFunc(h.RequestUsersExport), authn, require(rbac.PermPlatformUsersExport), requireStepUp,
	))
	mux.Handle("POST /v1/platform/roles/export", middleware.Chain(
		http.HandlerFunc(h.RequestRolesExport), authn, require(rbac.PermPlatformRolesExport), requireStepUp,
	))
	mux.Handle("POST /v1/platform/notifications/export", middleware.Chain(
		http.HandlerFunc(h.RequestNotificationsExport), authn, require(rbac.PermPlatformNotificationsExport), requireStepUp,
	))
	mux.Handle("POST /v1/platform/activity/export", middleware.Chain(
		http.HandlerFunc(h.RequestActivityExport), authn, require(rbac.PermPlatformActivityRead), requireStepUp,
	))
	mux.Handle("POST /v1/platform/orders/{uuid}/manifest/export", middleware.Chain(
		http.HandlerFunc(h.RequestOrderManifestExport), authn, require(rbac.PermPlatformOrdersRead), requireStepUp,
	))

	// App (account) exports: the requester downloads their own app.* jobs (TEC-552).
	mux.Handle("POST /v1/leads/export", middleware.Chain(
		http.HandlerFunc(h.RequestLeadsExport), authn, require(rbac.PermAppLeadsRead), requireStepUp,
	))
	mux.Handle("GET /v1/exports/{uuid}", middleware.Chain(http.HandlerFunc(h.GetOwn), authn))
	mux.Handle("GET /v1/exports/{uuid}/download", middleware.Chain(http.HandlerFunc(h.DownloadOwn), authn))
}
