package imports

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	importhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *importhandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}

	mux.Handle("GET /v1/platform/imports", middleware.Chain(
		http.HandlerFunc(h.List), authn, require(rbac.PermPlatformImportsRead),
	))
	mux.Handle("GET /v1/platform/imports/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Get), authn, require(rbac.PermPlatformImportsRead),
	))
	mux.Handle("PATCH /v1/platform/imports/{uuid}/mapping", middleware.Chain(
		http.HandlerFunc(h.UpdateMapping), authn, require(rbac.PermPlatformImportsRead),
	))
	mux.Handle("POST /v1/platform/imports/{uuid}/preview", middleware.Chain(
		http.HandlerFunc(h.Preview), authn, require(rbac.PermPlatformImportsRead),
	))
	mux.Handle("POST /v1/platform/imports/{uuid}/confirm", middleware.Chain(
		http.HandlerFunc(h.Confirm), authn, require(rbac.PermPlatformImportsRead),
	))
	mux.Handle("POST /v1/platform/imports/{uuid}/rollback", middleware.Chain(
		http.HandlerFunc(h.Rollback), authn, require(rbac.PermPlatformImportsRead),
	))

	mux.Handle("POST /v1/platform/users/import", middleware.Chain(
		http.HandlerFunc(h.UploadUsers), authn, require(rbac.PermPlatformUsersImport),
	))
	mux.Handle("GET /v1/platform/users/import/sample", middleware.Chain(
		http.HandlerFunc(h.SampleUsers), authn, require(rbac.PermPlatformUsersImport),
	))
	mux.Handle("POST /v1/platform/roles/import", middleware.Chain(
		http.HandlerFunc(h.UploadRoles), authn, require(rbac.PermPlatformRolesImport),
	))
	mux.Handle("GET /v1/platform/roles/import/sample", middleware.Chain(
		http.HandlerFunc(h.SampleRoles), authn, require(rbac.PermPlatformRolesImport),
	))

	// Employee import of the active company account (TEC-573): owners/admins only, and the job
	// routes reach only the caller's jobs of that account.
	company := func(fn http.HandlerFunc) http.Handler {
		return middleware.Chain(fn, authn, require(rbac.PermAppCompanyProfilesImport), importhandler.RequireCompanyImporter)
	}
	mux.Handle("POST /v1/company/employees/import", company(h.UploadCompanyEmployees))
	mux.Handle("GET /v1/company/employees/import/sample", company(h.SampleCompanyEmployees))
	mux.Handle("GET /v1/company/imports", company(h.CompanyList))
	mux.Handle("GET /v1/company/imports/{uuid}", company(h.CompanyGet))
	mux.Handle("PATCH /v1/company/imports/{uuid}/mapping", company(h.UpdateMapping))
	mux.Handle("POST /v1/company/imports/{uuid}/preview", company(h.Preview))
	mux.Handle("POST /v1/company/imports/{uuid}/confirm", company(h.Confirm))
	mux.Handle("POST /v1/company/imports/{uuid}/rollback", company(h.Rollback))
}
