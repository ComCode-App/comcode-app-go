package backgrounds

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	backgroundshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

// RegisterRoutes mounts Background Studio persistence. App designs are part of a profile's
// presentation, so they reuse the profiles permissions; presets are platform settings.
func RegisterRoutes(mux *http.ServeMux, h *backgroundshandler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	with := func(fn http.HandlerFunc, slug string) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(slug))
	}

	mux.Handle("GET /v1/backgrounds", with(h.List, rbac.PermAppProfilesRead))
	mux.Handle("POST /v1/backgrounds", with(h.Create, rbac.PermAppProfilesWrite))
	mux.Handle("PUT /v1/backgrounds/{uuid}", with(h.Update, rbac.PermAppProfilesWrite))
	mux.Handle("DELETE /v1/backgrounds/{uuid}", with(h.Delete, rbac.PermAppProfilesWrite))

	mux.Handle("GET /v1/platform/backgrounds", with(h.ListPresets, rbac.PermPlatformSettingsRead))
	mux.Handle("POST /v1/platform/backgrounds", with(h.CreatePreset, rbac.PermPlatformSettingsWrite))
	mux.Handle("PUT /v1/platform/backgrounds/{uuid}", with(h.UpdatePreset, rbac.PermPlatformSettingsWrite))
	mux.Handle("DELETE /v1/platform/backgrounds/{uuid}", with(h.DeletePreset, rbac.PermPlatformSettingsWrite))
}
