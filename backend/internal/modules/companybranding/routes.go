// Package companybranding wires the company media library and company branding (TEC-946).
package companybranding

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(mux *http.ServeMux, h *handler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	wrap := func(fn http.HandlerFunc, permission string) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(permission))
	}
	mux.Handle("GET /v1/company/media", wrap(h.ListMedia, rbac.PermAppCompanyBrandingRead))
	mux.Handle("POST /v1/company/media", wrap(h.UploadMedia, rbac.PermAppCompanyBrandingWrite))
	mux.Handle("DELETE /v1/company/media/{uuid}", wrap(h.DeleteMedia, rbac.PermAppCompanyBrandingWrite))
	mux.Handle("GET /v1/company/branding", wrap(h.GetBranding, rbac.PermAppCompanyBrandingRead))
	mux.Handle("PUT /v1/company/branding", wrap(h.UpdateBranding, rbac.PermAppCompanyBrandingWrite))
}
