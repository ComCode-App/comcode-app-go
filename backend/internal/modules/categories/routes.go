package categories

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *handler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}

	// Platform Category Routes
	mux.Handle("GET /v1/platform/categories/meta", authn(require(rbac.PermPlatformCategoriesRead)(http.HandlerFunc(h.Meta))))
	mux.Handle("GET /v1/platform/categories/active", authn(require(rbac.PermPlatformCategoriesRead)(http.HandlerFunc(h.ListActive))))
	mux.Handle("GET /v1/platform/categories", authn(require(rbac.PermPlatformCategoriesRead)(http.HandlerFunc(h.List))))
	mux.Handle("POST /v1/platform/categories", authn(require(rbac.PermPlatformCategoriesWrite)(http.HandlerFunc(h.Create))))
	mux.Handle("GET /v1/platform/categories/{uuid}", authn(require(rbac.PermPlatformCategoriesRead)(http.HandlerFunc(h.Get))))
	mux.Handle("PUT /v1/platform/categories/{uuid}", authn(require(rbac.PermPlatformCategoriesWrite)(http.HandlerFunc(h.Update))))
	mux.Handle("DELETE /v1/platform/categories/{uuid}", authn(require(rbac.PermPlatformCategoriesWrite)(http.HandlerFunc(h.Delete))))

	// Public / App Active Categories
	mux.HandleFunc("GET /v1/categories/active", h.ListActive)
}
