// Package platformsupport mounts the platform card and route support screens' API (TEC-950).
// Reads need platform.cards.read / platform.routes.read; actions need the matching write
// permission and a step-up grant.
package platformsupport

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/stepup"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *handler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
	stepUp *stepup.Service,
) {
	var requireStepUp func(http.Handler) http.Handler
	if stepUp != nil {
		requireStepUp = middleware.RequireStepUp(stepUp)
	} else {
		requireStepUp = func(next http.Handler) http.Handler { return next }
	}
	Register(mux, h, middleware.Authenticate(tokens, loader), requireStepUp)
}

// Register mounts the routes behind the given authentication and step-up middleware.
func Register(mux *http.ServeMux, h *handler.Handler, authn, requireStepUp func(http.Handler) http.Handler) {
	read := func(slug string, fn http.HandlerFunc) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(slug))
	}
	write := func(slug string, fn http.HandlerFunc) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(slug), requireStepUp)
	}

	mux.Handle("GET /v1/platform/cards", read(rbac.PermPlatformCardsRead, h.SearchCards))
	mux.Handle("GET /v1/platform/cards/{uuid}", read(rbac.PermPlatformCardsRead, h.GetCard))
	mux.Handle("POST /v1/platform/cards/{uuid}/retire", write(rbac.PermPlatformCardsWrite, h.RetireCard))
	mux.Handle("POST /v1/platform/cards/{uuid}/reassign", write(rbac.PermPlatformCardsWrite, h.ReassignCard))

	mux.Handle("GET /v1/platform/routes", read(rbac.PermPlatformRoutesRead, h.SearchRoutes))
	mux.Handle("POST /v1/platform/routes/quarantine/release", write(rbac.PermPlatformRoutesWrite, h.ReleaseQuarantine))
	mux.Handle("GET /v1/platform/routes/{uuid}", read(rbac.PermPlatformRoutesRead, h.GetRoute))
}
