// Package businesspages wires ComCode Business pages (TEC-1327).
package businesspages

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(mux *http.ServeMux, h *handler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	wrap := func(fn http.HandlerFunc, permission string) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(permission))
	}
	mux.Handle("GET /v1/business-page", wrap(h.Get, rbac.PermAppBusinessPageRead))
	mux.Handle("PUT /v1/business-page", wrap(h.Save, rbac.PermAppBusinessPageWrite))
	mux.Handle("POST /v1/business-page/publish", wrap(h.Publish, rbac.PermAppBusinessPageWrite))
	mux.Handle("POST /v1/business-page/unpublish", wrap(h.Unpublish, rbac.PermAppBusinessPageWrite))
	// Visitor endpoint for comcode.../{username} business pages: no auth.
	mux.HandleFunc("GET /v1/public/business/{username}", h.Public)
}
