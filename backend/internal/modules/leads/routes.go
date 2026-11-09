// Package leads is lead capture (TEC-552): visitor contact forms on public profiles, cards and
// smart links, contact exchange on profile pages (TEC-567), and the account's lead inbox.
package leads

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	leadshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

// RegisterRoutes mounts the visitor, exchange and app lead routes. Export lives in the exports module.
func RegisterRoutes(
	mux *http.ServeMux,
	h *leadshandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}

	// Visitors (no auth). Submissions are limited per client IP in the handler.
	mux.HandleFunc("GET /v1/public/leads/form", h.FormConfig)
	mux.HandleFunc("POST /v1/public/leads", h.Submit)
	mux.HandleFunc("POST /v1/public/exchanges", h.Exchange)
	// KVKK delete link of the visitor copy email (TEC-940); the signed token is the credential.
	mux.HandleFunc("POST /v1/public/exchanges/delete", h.DeleteExchange)

	// A signed-in visitor shares one of their own profiles (TEC-567).
	mux.Handle("POST /v1/exchanges", middleware.Chain(
		http.HandlerFunc(h.ShareProfile), authn, require(rbac.PermAppProfilesRead),
	))

	mux.Handle("GET /v1/leads/meta", middleware.Chain(
		http.HandlerFunc(h.Meta), authn, require(rbac.PermAppLeadsRead),
	))
	mux.Handle("GET /v1/leads", middleware.Chain(
		http.HandlerFunc(h.List), authn, require(rbac.PermAppLeadsRead),
	))
	mux.Handle("GET /v1/leads/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Get), authn, require(rbac.PermAppLeadsRead),
	))
	mux.Handle("POST /v1/leads/{uuid}/read", middleware.Chain(
		http.HandlerFunc(h.MarkRead), authn, require(rbac.PermAppLeadsWrite),
	))
	mux.Handle("DELETE /v1/leads/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Delete), authn, require(rbac.PermAppLeadsWrite),
	))
}
