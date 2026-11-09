package accounts

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	accountshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

// RegisterRoutes mounts account management: /v1/account acts on the caller's active account
// (account roles decide), /v1/platform/accounts on any account (platform.accounts.* decides).
func RegisterRoutes(mux *http.ServeMux, h *accountshandler.Handler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	app := func(fn http.HandlerFunc) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(rbac.PermAuthSession))
	}
	platform := func(fn http.HandlerFunc, slug string) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(slug))
	}

	mux.Handle("GET /v1/account", app(h.Current))
	mux.Handle("PATCH /v1/account", app(h.Rename))
	mux.Handle("GET /v1/account/entitlements", app(h.Entitlements))
	mux.Handle("GET /v1/account/members", app(h.Members))
	mux.Handle("POST /v1/account/members", app(h.AddMember))
	mux.Handle("PATCH /v1/account/members/{user_uuid}", app(h.ChangeRole))
	mux.Handle("DELETE /v1/account/members/{user_uuid}", app(h.RemoveMember))

	mux.Handle("GET /v1/platform/accounts/meta", platform(h.Meta, rbac.PermPlatformAccountsRead))
	mux.Handle("GET /v1/platform/accounts", platform(h.List, rbac.PermPlatformAccountsRead))
	mux.Handle("POST /v1/platform/accounts", platform(h.CreateCompany, rbac.PermPlatformAccountsWrite))
	mux.Handle("GET /v1/platform/accounts/{uuid}", platform(h.Get, rbac.PermPlatformAccountsRead))
	mux.Handle("PATCH /v1/platform/accounts/{uuid}", platform(h.Patch, rbac.PermPlatformAccountsWrite))
	mux.Handle("POST /v1/platform/accounts/{uuid}/members", platform(h.PlatformAddMember, rbac.PermPlatformAccountsWrite))
	mux.Handle("PATCH /v1/platform/accounts/{uuid}/members/{user_uuid}", platform(h.PlatformChangeRole, rbac.PermPlatformAccountsWrite))
	mux.Handle("DELETE /v1/platform/accounts/{uuid}/members/{user_uuid}", platform(h.PlatformRemoveMember, rbac.PermPlatformAccountsWrite))
}

// RegisterInviteRoutes mounts company email invites (TEC-945). Managing invites needs the
// app.account_invites.* permission plus owner/admin of the active account (checked by the use
// case); the preview is public (the token is the credential) and accepting needs a session.
func RegisterInviteRoutes(mux *http.ServeMux, h *accountshandler.InvitesHandler, tokens *jwt.Manager, loader middleware.IdentityLoader) {
	authn := middleware.Authenticate(tokens, loader)
	perm := func(fn http.HandlerFunc, slug string) http.Handler {
		return middleware.Chain(fn, authn, middleware.RequirePermission(slug))
	}

	mux.Handle("GET /v1/account/invites", perm(h.List, rbac.PermAppAccountInvitesRead))
	mux.Handle("POST /v1/account/invites", perm(h.Create, rbac.PermAppAccountInvitesWrite))
	mux.Handle("POST /v1/account/invites/{uuid}/resend", perm(h.Resend, rbac.PermAppAccountInvitesWrite))
	mux.Handle("DELETE /v1/account/invites/{uuid}", perm(h.Revoke, rbac.PermAppAccountInvitesWrite))

	mux.HandleFunc("GET /v1/invites/{token}", h.Preview)
	mux.Handle("POST /v1/invites/{token}/accept", perm(h.Accept, rbac.PermAuthSession))
}
