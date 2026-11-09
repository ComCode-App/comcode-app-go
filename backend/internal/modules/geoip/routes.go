package geoip

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	geoiphandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/geoip/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

// RegisterRoutes registers endpoints for GeoIP status, update, cache clearing, and configuration.
func RegisterRoutes(
	mux *http.ServeMux,
	h *geoiphandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	readGeoIP := middleware.RequirePermission(rbac.PermPlatformGeoIPRead)
	manageGeoIP := middleware.RequirePermission(rbac.PermPlatformGeoIPManage)

	mux.Handle("GET /v1/platform/geoip/status", middleware.Chain(
		http.HandlerFunc(h.Status), authn, readGeoIP,
	))
	mux.Handle("GET /v1/platform/geoip/lookup", middleware.Chain(
		http.HandlerFunc(h.Lookup), authn, readGeoIP,
	))
	mux.Handle("POST /v1/platform/geoip/update", middleware.Chain(
		http.HandlerFunc(h.Update), authn, manageGeoIP,
	))
	mux.Handle("POST /v1/platform/geoip/cache/clear", middleware.Chain(
		http.HandlerFunc(h.ClearCache), authn, manageGeoIP,
	))
	mux.Handle("GET /v1/platform/geoip/config", middleware.Chain(
		http.HandlerFunc(h.GetConfig), authn, readGeoIP,
	))
	mux.Handle("PATCH /v1/platform/geoip/config", middleware.Chain(
		http.HandlerFunc(h.PatchConfig), authn, manageGeoIP,
	))
}
