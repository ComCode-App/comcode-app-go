// Package overview is the platform home's business KPIs (TEC-594): users, profiles, cards,
// orders, subscriptions/MRR, most viewed profiles and event volume, plus the order conversion
// funnel (TEC-944). Platform-wide, unscoped.
package overview

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	overviewhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

// RegisterRoutes mounts GET /v1/platform/overview and GET /v1/platform/overview/funnel.
func RegisterRoutes(
	mux *http.ServeMux,
	h *overviewhandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	mux.Handle("GET /v1/platform/overview", middleware.Chain(
		http.HandlerFunc(h.Get), middleware.Authenticate(tokens, loader), middleware.RequirePermission(rbac.PermPlatformOverviewRead),
	))
	mux.Handle("GET /v1/platform/overview/funnel", middleware.Chain(
		http.HandlerFunc(h.Funnel), middleware.Authenticate(tokens, loader), middleware.RequirePermission(rbac.PermPlatformOverviewRead),
	))
}
