package events

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	eventshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/events/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

// RegisterRoutes registers endpoints for event ingestion and definition management.
func RegisterRoutes(
	mux *http.ServeMux,
	h *eventshandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	manageEvents := middleware.RequirePermission(rbac.PermPlatformEventsManage)

	// Public event ingestion endpoint (per-IP limit in the handler, see SetRateLimiter)
	mux.HandleFunc("POST /v1/events", h.Ingest)

	// Admin event definitions endpoints
	mux.Handle("GET /v1/platform/events/definitions", middleware.Chain(
		http.HandlerFunc(h.ListDefinitions), authn, manageEvents,
	))
	mux.Handle("POST /v1/platform/events/definitions", middleware.Chain(
		http.HandlerFunc(h.CreateDefinition), authn, manageEvents,
	))
	mux.Handle("GET /v1/platform/events/definitions/{id}", middleware.Chain(
		http.HandlerFunc(h.GetDefinition), authn, manageEvents,
	))
	mux.Handle("PATCH /v1/platform/events/definitions/{id}", middleware.Chain(
		http.HandlerFunc(h.PatchDefinition), authn, manageEvents,
	))
	mux.Handle("DELETE /v1/platform/events/definitions/{id}", middleware.Chain(
		http.HandlerFunc(h.DeleteDefinition), authn, manageEvents,
	))
}
