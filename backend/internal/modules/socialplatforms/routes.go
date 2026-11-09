package socialplatforms

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	socialplatformshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *socialplatformshandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}

	// Active list for subscribers/all authenticated users
	mux.Handle("GET /v1/social-platforms", middleware.Chain(
		http.HandlerFunc(h.ListActive), authn,
	))

	// Platform Admin CRUD
	mux.Handle("GET /v1/platform/social-platforms/meta", middleware.Chain(
		http.HandlerFunc(h.Meta), authn, require(rbac.PermPlatformSocialPlatformsRead),
	))
	mux.Handle("GET /v1/platform/social-platforms", middleware.Chain(
		http.HandlerFunc(h.List), authn, require(rbac.PermPlatformSocialPlatformsRead),
	))
	mux.Handle("POST /v1/platform/social-platforms", middleware.Chain(
		http.HandlerFunc(h.Create), authn, require(rbac.PermPlatformSocialPlatformsWrite),
	))
	mux.Handle("POST /v1/platform/social-platforms/bulk-fetch-icons", middleware.Chain(
		http.HandlerFunc(h.BulkFetchIcons), authn, require(rbac.PermPlatformSocialPlatformsWrite),
	))
	mux.Handle("GET /v1/platform/social-platforms/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Get), authn, require(rbac.PermPlatformSocialPlatformsRead),
	))
	mux.Handle("PUT /v1/platform/social-platforms/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Update), authn, require(rbac.PermPlatformSocialPlatformsWrite),
	))
	mux.Handle("POST /v1/platform/social-platforms/{uuid}/icon", middleware.Chain(
		http.HandlerFunc(h.UploadIcon), authn, require(rbac.PermPlatformSocialPlatformsWrite),
	))
	mux.Handle("POST /v1/platform/social-platforms/{uuid}/fetch-icon", middleware.Chain(
		http.HandlerFunc(h.FetchIcon), authn, require(rbac.PermPlatformSocialPlatformsWrite),
	))
	mux.Handle("DELETE /v1/platform/social-platforms/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Delete), authn, require(rbac.PermPlatformSocialPlatformsWrite),
	))
}
