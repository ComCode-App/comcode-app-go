package cards

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/handler"
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

	// App User NFC Cards Management
	mux.Handle("GET /v1/cards/meta", authn(require(rbac.PermAppCardsRead)(http.HandlerFunc(h.Meta))))
	mux.Handle("GET /v1/cards", authn(require(rbac.PermAppCardsRead)(http.HandlerFunc(h.List))))
	mux.Handle("POST /v1/cards", authn(require(rbac.PermAppCardsWrite)(http.HandlerFunc(h.Create))))
	mux.Handle("POST /v1/cards/activate", authn(require(rbac.PermAppCardsWrite)(http.HandlerFunc(h.Activate))))
	mux.Handle("GET /v1/cards/{uuid}", authn(require(rbac.PermAppCardsRead)(http.HandlerFunc(h.Get))))
	mux.Handle("PUT /v1/cards/{uuid}", authn(require(rbac.PermAppCardsWrite)(http.HandlerFunc(h.Update))))
	mux.Handle("PATCH /v1/cards/{uuid}/toggle-lock", authn(require(rbac.PermAppCardsWrite)(http.HandlerFunc(h.ToggleLock))))
	mux.Handle("DELETE /v1/cards/{uuid}", authn(require(rbac.PermAppCardsWrite)(http.HandlerFunc(h.Delete))))
}
