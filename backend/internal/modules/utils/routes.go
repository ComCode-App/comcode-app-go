package utils

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	utilshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/utils/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *utilshandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)

	mux.Handle("POST /v1/utils/link-metadata", middleware.Chain(
		http.HandlerFunc(h.GetLinkMetadata), authn,
	))
}
