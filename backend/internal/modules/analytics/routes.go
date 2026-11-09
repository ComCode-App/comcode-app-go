package analytics

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	analyticshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/analytics/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

// RegisterRoutes registers analytics querying and admin configuration endpoints.
func RegisterRoutes(
	mux *http.ServeMux,
	h *analyticshandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	readAnalytics := middleware.RequirePermission(rbac.PermPlatformAnalyticsRead)
	manageAnalytics := middleware.RequirePermission(rbac.PermPlatformAnalyticsManage)

	// Analytics queries (authenticated)
	mux.Handle("GET /v1/analytics/summary", middleware.Chain(
		http.HandlerFunc(h.Summary), authn,
	))
	mux.Handle("GET /v1/analytics/timeseries", middleware.Chain(
		http.HandlerFunc(h.TimeSeries), authn,
	))
	mux.Handle("GET /v1/analytics/breakdown", middleware.Chain(
		http.HandlerFunc(h.Breakdown), authn,
	))
	mux.Handle("GET /v1/analytics/sources", middleware.Chain(
		http.HandlerFunc(h.Sources), authn,
	))
	mux.Handle("GET /v1/analytics/events", middleware.Chain(
		http.HandlerFunc(h.Events), authn, readAnalytics,
	))

	// Platform-wide KPIs from the aggregate tables (TEC-859)
	mux.Handle("GET /v1/platform/analytics/overview", middleware.Chain(
		http.HandlerFunc(h.PlatformOverview), authn, readAnalytics,
	))

	// Admin analytics configuration
	mux.Handle("GET /v1/platform/analytics/config", middleware.Chain(
		http.HandlerFunc(h.GetConfig), authn, readAnalytics,
	))
	mux.Handle("PATCH /v1/platform/analytics/config", middleware.Chain(
		http.HandlerFunc(h.PatchConfig), authn, manageAnalytics,
	))
}
