// Package companyanalytics wires the company analytics API (TEC-574).
package companyanalytics

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(mux *http.ServeMux, h *handler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	wrap := func(fn http.HandlerFunc) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(rbac.PermAppCompanyAnalyticsRead))
	}
	mux.Handle("GET /v1/company/analytics/summary", wrap(h.Summary))
	mux.Handle("GET /v1/company/analytics/timeseries", wrap(h.TimeSeries))
	mux.Handle("GET /v1/company/analytics/breakdown", wrap(h.Breakdown))
	mux.Handle("GET /v1/company/analytics/employees", wrap(h.Employees))
}
