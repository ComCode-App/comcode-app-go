package profiles

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	profileshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

func RegisterRoutes(
	mux *http.ServeMux,
	h *profileshandler.Handler,
	tokens *jwt.Manager,
	loader middleware.IdentityLoader,
) {
	authn := middleware.Authenticate(tokens, loader)
	require := func(slug string) func(http.Handler) http.Handler {
		return middleware.RequirePermission(slug)
	}

	// Meta before /{uuid}
	mux.Handle("GET /v1/profiles/meta", middleware.Chain(
		http.HandlerFunc(h.Meta), authn, require(rbac.PermAppProfilesRead),
	))
	mux.Handle("GET /v1/profiles", middleware.Chain(
		http.HandlerFunc(h.List), authn, require(rbac.PermAppProfilesRead),
	))
	mux.Handle("POST /v1/profiles", middleware.Chain(
		http.HandlerFunc(h.Create), authn, require(rbac.PermAppProfilesWrite),
	))
	mux.Handle("POST /v1/profiles/vcard/parse", middleware.Chain(
		http.HandlerFunc(h.ParseVCard), authn, require(rbac.PermAppProfilesRead),
	))
	mux.Handle("GET /v1/profiles/{uuid}/vcard", middleware.Chain(
		http.HandlerFunc(h.DownloadVCard), authn, require(rbac.PermAppProfilesRead),
	))
	mux.Handle("POST /v1/profiles/{uuid}/avatar", middleware.Chain(
		http.HandlerFunc(h.UploadAvatar), authn, require(rbac.PermAppProfilesWrite),
	))
	mux.Handle("POST /v1/profiles/{uuid}/cover", middleware.Chain(
		http.HandlerFunc(h.UploadCover), authn, require(rbac.PermAppProfilesWrite),
	))
	mux.Handle("POST /v1/profiles/{uuid}/primary", middleware.Chain(
		http.HandlerFunc(h.SetPrimary), authn, require(rbac.PermAppProfilesWrite),
	))
	mux.Handle("GET /v1/profiles/{uuid}/public-page", middleware.Chain(
		http.HandlerFunc(h.GetPublicPage), authn, require(rbac.PermAppProfilesRead),
	))
	mux.Handle("PUT /v1/profiles/{uuid}/public-page", middleware.Chain(
		http.HandlerFunc(h.UpdatePublicPage), authn, require(rbac.PermAppProfilesWrite),
	))
	// Visitor endpoints for comcode.../{username}: no auth.
	mux.HandleFunc("GET /v1/public/p/{username}", h.PublicProfile)
	mux.HandleFunc("GET /v1/public/p/{username}/vcard", h.PublicVCard)
	mux.Handle("GET /v1/profiles/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Get), authn, require(rbac.PermAppProfilesRead),
	))
	mux.Handle("PUT /v1/profiles/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Update), authn, require(rbac.PermAppProfilesWrite),
	))
	mux.Handle("DELETE /v1/profiles/{uuid}", middleware.Chain(
		http.HandlerFunc(h.Delete), authn, require(rbac.PermAppProfilesWrite),
	))
}
