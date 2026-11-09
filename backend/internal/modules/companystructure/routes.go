package companystructure

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(mux *http.ServeMux, h *handler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	wrap := func(fn http.HandlerFunc, perm string) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(perm))
	}
	mux.Handle("GET /v1/company/offices", wrap(h.Offices, rbac.PermAppCompanyStructureRead))
	mux.Handle("POST /v1/company/offices", wrap(h.CreateOffice, rbac.PermAppCompanyStructureWrite))
	mux.Handle("GET /v1/company/offices/{uuid}", wrap(h.GetOffice, rbac.PermAppCompanyStructureRead))
	mux.Handle("PUT /v1/company/offices/{uuid}", wrap(h.UpdateOffice, rbac.PermAppCompanyStructureWrite))
	mux.Handle("DELETE /v1/company/offices/{uuid}", wrap(h.DeleteOffice, rbac.PermAppCompanyStructureWrite))
	mux.Handle("GET /v1/company/groups", wrap(h.Groups, rbac.PermAppCompanyStructureRead))
	mux.Handle("POST /v1/company/groups", wrap(h.CreateGroup, rbac.PermAppCompanyStructureWrite))
	mux.Handle("GET /v1/company/groups/{uuid}", wrap(h.GetGroup, rbac.PermAppCompanyStructureRead))
	mux.Handle("PUT /v1/company/groups/{uuid}", wrap(h.UpdateGroup, rbac.PermAppCompanyStructureWrite))
	mux.Handle("DELETE /v1/company/groups/{uuid}", wrap(h.DeleteGroup, rbac.PermAppCompanyStructureWrite))
	mux.Handle("GET /v1/company/assignments", wrap(h.Assignments, rbac.PermAppCompanyStructureRead))
	mux.Handle("GET /v1/company/members/{id}/assignment", wrap(h.Assignment, rbac.PermAppCompanyStructureRead))
	mux.Handle("PUT /v1/company/members/{id}/assignment", wrap(h.SetAssignment, rbac.PermAppCompanyStructureWrite))
	mux.Handle("GET /v1/company/members/{id}/scopes", wrap(h.Scopes, rbac.PermAppCompanyStructureRead))
	mux.Handle("PUT /v1/company/members/{id}/scopes", wrap(h.SetScopes, rbac.PermAppCompanyStructureWrite))
	mux.Handle("GET /v1/company/sector", wrap(h.Sector, rbac.PermAppCompanyStructureRead))
	mux.Handle("PUT /v1/company/sector", wrap(h.SetSector, rbac.PermAppCompanyStructureWrite))
	mux.Handle("GET /v1/sectors", wrap(h.Sectors, rbac.PermAppCompanyStructureRead))
	mux.Handle("GET /v1/platform/sectors/meta", wrap(h.SectorsMeta, rbac.PermPlatformSectorsRead))
	mux.Handle("GET /v1/platform/sectors", wrap(h.Sectors, rbac.PermPlatformSectorsRead))
	mux.Handle("POST /v1/platform/sectors", wrap(h.CreateSector, rbac.PermPlatformSectorsWrite))
	mux.Handle("GET /v1/platform/sectors/{uuid}", wrap(h.GetSector, rbac.PermPlatformSectorsRead))
	mux.Handle("PUT /v1/platform/sectors/{uuid}", wrap(h.UpdateSector, rbac.PermPlatformSectorsWrite))
	mux.Handle("DELETE /v1/platform/sectors/{uuid}", wrap(h.DeleteSector, rbac.PermPlatformSectorsWrite))
	mux.Handle("GET /v1/platform/accounts/{uuid}/offices", wrap(h.AdminOffices, rbac.PermPlatformAccountsRead))
	mux.Handle("GET /v1/platform/accounts/{uuid}/groups", wrap(h.AdminGroups, rbac.PermPlatformAccountsRead))
}
