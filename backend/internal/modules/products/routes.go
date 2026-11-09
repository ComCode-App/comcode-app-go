package products

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/handler"
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

	// Platform Product Routes
	mux.Handle("GET /v1/platform/products/meta", authn(require(rbac.PermPlatformProductsRead)(http.HandlerFunc(h.Meta))))
	mux.Handle("GET /v1/platform/products", authn(require(rbac.PermPlatformProductsRead)(http.HandlerFunc(h.List))))
	mux.Handle("POST /v1/platform/products", authn(require(rbac.PermPlatformProductsWrite)(http.HandlerFunc(h.Create))))
	mux.Handle("GET /v1/platform/products/{uuid}", authn(require(rbac.PermPlatformProductsRead)(http.HandlerFunc(h.Get))))
	mux.Handle("PUT /v1/platform/products/{uuid}", authn(require(rbac.PermPlatformProductsWrite)(http.HandlerFunc(h.Update))))
	mux.Handle("PUT /v1/platform/products/{uuid}/viewbox", authn(require(rbac.PermPlatformProductsWrite)(http.HandlerFunc(h.Update3DViewbox))))
	mux.Handle("POST /v1/platform/products/{uuid}/3d-model", authn(require(rbac.PermPlatformProductsWrite)(http.HandlerFunc(h.Upload3DModel))))
	mux.Handle("POST /v1/platform/products/{uuid}/images", authn(require(rbac.PermPlatformProductsWrite)(http.HandlerFunc(h.UploadImage))))
	mux.Handle("DELETE /v1/platform/products/{uuid}", authn(require(rbac.PermPlatformProductsWrite)(http.HandlerFunc(h.Delete))))

	// Variant Routes
	mux.Handle("GET /v1/platform/products/{uuid}/variants", authn(require(rbac.PermPlatformProductsRead)(http.HandlerFunc(h.ListVariants))))
	mux.Handle("POST /v1/platform/products/{uuid}/variants", authn(require(rbac.PermPlatformProductsWrite)(http.HandlerFunc(h.CreateVariant))))
	mux.Handle("PUT /v1/platform/products/{uuid}/variants/{variantUuid}", authn(require(rbac.PermPlatformProductsWrite)(http.HandlerFunc(h.UpdateVariant))))
	mux.Handle("DELETE /v1/platform/products/{uuid}/variants/{variantUuid}", authn(require(rbac.PermPlatformProductsWrite)(http.HandlerFunc(h.DeleteVariant))))

	// Public / Storefront Routes
	mux.HandleFunc("GET /v1/products", h.List)
	mux.HandleFunc("GET /v1/products/by-slug/{slug}", h.GetBySlug)
	mux.HandleFunc("GET /v1/products/{uuid}", h.Get)
}
