// Package webhooks mounts the outbound webhook routes (TEC-603).
package webhooks

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	webhookshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

// RegisterRoutes mounts /v1/webhooks. Reads need app.webhooks.read; writes need
// app.webhooks.write and an owner/admin account role (checked in the usecase).
func RegisterRoutes(mux *http.ServeMux, h *webhookshandler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	read := middleware.RequirePermission(rbac.PermAppWebhooksRead)
	write := middleware.RequirePermission(rbac.PermAppWebhooksWrite)

	mux.Handle("GET /v1/webhooks/events", middleware.Chain(http.HandlerFunc(h.Events), authn, read))
	mux.Handle("GET /v1/webhooks", middleware.Chain(http.HandlerFunc(h.List), authn, read))
	mux.Handle("POST /v1/webhooks", middleware.Chain(http.HandlerFunc(h.Create), authn, write))
	mux.Handle("GET /v1/webhooks/{uuid}", middleware.Chain(http.HandlerFunc(h.Get), authn, read))
	mux.Handle("PUT /v1/webhooks/{uuid}", middleware.Chain(http.HandlerFunc(h.Update), authn, write))
	mux.Handle("DELETE /v1/webhooks/{uuid}", middleware.Chain(http.HandlerFunc(h.Delete), authn, write))
	mux.Handle("POST /v1/webhooks/{uuid}/rotate-secret", middleware.Chain(http.HandlerFunc(h.RotateSecret), authn, write))
	mux.Handle("POST /v1/webhooks/{uuid}/test", middleware.Chain(http.HandlerFunc(h.SendTest), authn, write))
	mux.Handle("GET /v1/webhooks/{uuid}/deliveries", middleware.Chain(http.HandlerFunc(h.ListDeliveries), authn, read))
	mux.Handle("POST /v1/webhooks/{uuid}/deliveries/{deliveryUuid}/resend", middleware.Chain(http.HandlerFunc(h.ResendDelivery), authn, write))
}
