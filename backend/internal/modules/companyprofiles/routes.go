// Package companyprofiles wires the employee profile routes of company accounts (TEC-573).
// Bulk job routes live in modules/bulk, import routes in modules/imports.
package companyprofiles

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(mux *http.ServeMux, h *handler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	wrap := func(fn http.HandlerFunc, perm string) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(perm))
	}
	mux.Handle("GET /v1/company/member-profiles", wrap(h.MemberProfiles, rbac.PermAppProfilesRead))
	mux.Handle("GET /v1/company/member-profiles/meta", wrap(h.Meta, rbac.PermAppProfilesRead))
	mux.Handle("PUT /v1/company/members/{id}/profile", wrap(h.LinkProfile, rbac.PermAppProfilesWrite))
}
