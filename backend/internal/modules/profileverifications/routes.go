// Package profileverifications is the verified profile badge (TEC-549): owners request
// verification of a profile, platform admins review it, and the visitor API shows the badge.
package profileverifications

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	pvhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

// RegisterRoutes mounts the owner and platform verification routes.
func RegisterRoutes(
	mux *http.ServeMux,
	h *pvhandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}

	// Owner side: scoped to the active account; writes also need an owner/admin account role.
	mux.Handle("GET /v1/profiles/{uuid}/verification", middleware.Chain(
		http.HandlerFunc(h.OwnerGet), authn, require(rbac.PermAppProfilesRead),
	))
	mux.Handle("POST /v1/profiles/{uuid}/verification", middleware.Chain(
		http.HandlerFunc(h.Request), authn, require(rbac.PermAppProfilesWrite),
	))
	mux.Handle("DELETE /v1/profiles/{uuid}/verification", middleware.Chain(
		http.HandlerFunc(h.Cancel), authn, require(rbac.PermAppProfilesWrite),
	))

	// Platform review queue (all accounts).
	manage := require(rbac.PermPlatformProfileVerificationsManage)
	mux.Handle("GET /v1/platform/profile-verifications", middleware.Chain(
		http.HandlerFunc(h.List), authn, manage,
	))
	mux.Handle("GET /v1/platform/profile-verifications/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Get), authn, manage,
	))
	mux.Handle("POST /v1/platform/profile-verifications/{uuid}/approve", middleware.Chain(
		http.HandlerFunc(h.Approve), authn, manage,
	))
	mux.Handle("POST /v1/platform/profile-verifications/{uuid}/reject", middleware.Chain(
		http.HandlerFunc(h.Reject), authn, manage,
	))
	mux.Handle("POST /v1/platform/profile-verifications/{uuid}/revoke", middleware.Chain(
		http.HandlerFunc(h.Revoke), authn, manage,
	))
}
