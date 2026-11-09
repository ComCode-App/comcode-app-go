package orders

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/handler"
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

	// Platform Order Management Routes
	mux.Handle("GET /v1/platform/orders/meta", authn(require(rbac.PermPlatformOrdersRead)(http.HandlerFunc(h.Meta))))
	mux.Handle("GET /v1/platform/orders/kpis", authn(require(rbac.PermPlatformOrdersRead)(http.HandlerFunc(h.KPIs))))
	mux.Handle("GET /v1/platform/orders", authn(require(rbac.PermPlatformOrdersRead)(http.HandlerFunc(h.List))))
	mux.Handle("POST /v1/platform/orders", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.Create))))
	mux.Handle("POST /v1/platform/orders/bulk", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.Create))))
	mux.Handle("GET /v1/platform/orders/{uuid}", authn(require(rbac.PermPlatformOrdersRead)(http.HandlerFunc(h.Get))))
	mux.Handle("GET /v1/platform/orders/{uuid}/manifest/qr-codes", authn(require(rbac.PermPlatformOrdersRead)(http.HandlerFunc(h.ManifestQRCodes))))
	mux.Handle("POST /v1/platform/orders/{uuid}/provision", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.ProvisionOrder))))
	mux.Handle("POST /v1/platform/orders/{uuid}/items/{itemUuid}/provision", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.ProvisionItem))))
	mux.Handle("PUT /v1/platform/orders/{uuid}/items/bulk", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.BulkUpdateItems))))
	mux.Handle("PUT /v1/platform/orders/{uuid}/status", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.UpdateStatus))))
	mux.Handle("PUT /v1/platform/orders/{uuid}/payment", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.UpdatePaymentStatus))))
	mux.Handle("PUT /v1/platform/orders/{uuid}/fulfillment", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.UpdateFulfillmentStatus))))
	mux.Handle("PUT /v1/platform/orders/{uuid}/shipping", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.UpdateShipping))))
	mux.Handle("PUT /v1/platform/orders/{uuid}/notes", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.UpdateNotes))))
	mux.Handle("DELETE /v1/platform/orders/{uuid}", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.Delete))))

	// Platform Asset Requests
	mux.Handle("POST /v1/platform/orders/{uuid}/assets", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.CreateAssetRequest))))
	mux.Handle("PUT /v1/platform/orders/{uuid}/assets/{assetUuid}/review", authn(require(rbac.PermPlatformOrdersWrite)(http.HandlerFunc(h.ReviewAsset))))

	// Public / Client Token Asset Upload Routes
	mux.HandleFunc("GET /v1/orders/assets/{token}", h.GetAssetByToken)
	mux.HandleFunc("POST /v1/orders/assets/upload", h.UploadAsset)

	// App / Customer Order Routes
	mux.Handle("GET /v1/orders", authn(require(rbac.PermAppOrdersRead)(http.HandlerFunc(h.ListMyOrders))))
	mux.Handle("GET /v1/orders/{uuid}", authn(require(rbac.PermAppOrdersRead)(http.HandlerFunc(h.GetMyOrder))))
	mux.Handle("POST /v1/orders", authn(require(rbac.PermAppOrdersWrite)(http.HandlerFunc(h.CreateMine))))
	mux.Handle("POST /v1/orders/bulk", authn(require(rbac.PermAppOrdersWrite)(http.HandlerFunc(h.CreateMine))))
}
