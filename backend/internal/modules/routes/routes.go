package routes

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/handler"
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

	// Routes Resource Meta (before /{uuid})
	mux.Handle("GET /v1/routes/meta", authn(require(rbac.PermAppRoutesRead)(http.HandlerFunc(h.Meta))))

	// Management endpoints
	mux.Handle("GET /v1/routes", authn(require(rbac.PermAppRoutesRead)(http.HandlerFunc(h.List))))
	mux.Handle("POST /v1/routes/smart-links", authn(require(rbac.PermAppRoutesWrite)(http.HandlerFunc(h.CreateSmartLink))))
	mux.Handle("GET /v1/routes/{uuid}", authn(require(rbac.PermAppRoutesRead)(http.HandlerFunc(h.Get))))
	mux.Handle("PUT /v1/routes/{uuid}", authn(require(rbac.PermAppRoutesWrite)(http.HandlerFunc(h.Update))))
	mux.Handle("PATCH /v1/routes/{uuid}/toggle-lock", authn(require(rbac.PermAppRoutesWrite)(http.HandlerFunc(h.ToggleLock))))
	mux.Handle("DELETE /v1/routes/{uuid}", authn(require(rbac.PermAppRoutesWrite)(http.HandlerFunc(h.Delete))))

	mux.Handle("GET /v1/routes/{uuid}/qr", authn(require(rbac.PermAppRoutesRead)(http.HandlerFunc(h.GetQR))))
	mux.Handle("PUT /v1/routes/{uuid}/qr", authn(require(rbac.PermAppRoutesWrite)(http.HandlerFunc(h.PutQR))))
	mux.Handle("GET /v1/routes/{uuid}/qr/render", authn(require(rbac.PermAppRoutesRead)(http.HandlerFunc(h.RenderQR))))
	mux.Handle("POST /v1/routes/{uuid}/qr/preview", authn(require(rbac.PermAppRoutesRead)(http.HandlerFunc(h.PreviewQR))))

	// Public Resolvers
	mux.HandleFunc("GET /v1/r/{path...}", h.Resolve)
	mux.HandleFunc("GET /v1/c/{code}", h.ResolveCard)
	mux.HandleFunc("GET /v1/s/{slug}", h.ResolveSmartLink)
	mux.HandleFunc("POST /v1/s/{slug}/unlock", h.UnlockSmartLink)
}
