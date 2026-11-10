package qrlogin

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
)

// RegisterRoutes mounts the QR sign-in endpoints (TEC-1184).
func RegisterRoutes(mux *http.ServeMux, h *Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)

	// Web tab (anonymous; the browser secret proves tab ownership). Exchange is called by the
	// web server only; the BFF refuses it from browsers.
	mux.HandleFunc("POST /v1/auth/qr/sessions", h.Create)
	mux.HandleFunc("POST /v1/auth/qr/sessions/{id}/state", h.State)
	mux.HandleFunc("POST /v1/auth/qr/exchange", h.Exchange)

	// Signed-in phone.
	mux.Handle("GET /v1/auth/qr/sessions/{id}", middleware.Chain(http.HandlerFunc(h.Get), authn))
	mux.Handle("POST /v1/auth/qr/sessions/{id}/approve", middleware.Chain(http.HandlerFunc(h.Approve), authn))
	mux.Handle("POST /v1/auth/qr/sessions/{id}/reject", middleware.Chain(http.HandlerFunc(h.Reject), authn))
}
