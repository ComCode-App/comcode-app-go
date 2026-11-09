// Package companyassets wires company-scoped employee card and order APIs.
package companyassets

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(mux *http.ServeMux, h *handler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	wrap := func(fn http.HandlerFunc, permission string) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(permission))
	}
	mux.Handle("GET /v1/company/cards/meta", wrap(h.CardsMeta, rbac.PermAppCardsRead))
	mux.Handle("GET /v1/company/cards", wrap(h.Cards, rbac.PermAppCardsRead))
	mux.Handle("PUT /v1/company/cards/{uuid}/assignment", wrap(h.AssignCard, rbac.PermAppCardsWrite))
	mux.Handle("PUT /v1/company/cards/{uuid}/active", wrap(h.SetCardActive, rbac.PermAppCardsWrite))
	mux.Handle("GET /v1/company/orders/meta", wrap(h.OrdersMeta, rbac.PermAppOrdersRead))
	mux.Handle("GET /v1/company/orders", wrap(h.Orders, rbac.PermAppOrdersRead))
	mux.Handle("POST /v1/company/orders/reorder", wrap(h.Reorder, rbac.PermAppOrdersWrite))
}
