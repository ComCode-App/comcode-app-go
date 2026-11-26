// Package profileshares wires profile sharing with companies (TEC-1315).
package profileshares

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileshares/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(mux *http.ServeMux, h *handler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	wrap := func(fn http.HandlerFunc, perm string) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(perm))
	}
	mux.Handle("GET /v1/profiles/{uuid}/company-shares", wrap(h.Targets, rbac.PermAppProfilesRead))
	mux.Handle("PUT /v1/profiles/{uuid}/company-shares/{company_uuid}", wrap(h.Share, rbac.PermAppProfilesWrite))
	mux.Handle("DELETE /v1/profiles/{uuid}/company-shares/{company_uuid}", wrap(h.Unshare, rbac.PermAppProfilesWrite))
	mux.Handle("GET /v1/company/shared-profiles", wrap(h.CompanyProfiles, rbac.PermAppProfilesRead))
}
