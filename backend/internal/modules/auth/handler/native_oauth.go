package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Native Apple / Google sign-in limits (TEC-1180, docs/design/mobile.md §2.6), per 15-minute
// window. The per-subject cap also bounds authenticator code guesses for one identity.
const (
	nativeSignInIPLimit      = 30
	nativeSignInSubjectLimit = 10
	nativeLinkUserLimit      = 10
	nativeLinkIPLimit        = 30
)

// Error codes of the native sign-in endpoints.
const (
	codeOAuthProviderDisabled = "OAUTH_PROVIDER_DISABLED"
	codeOAuthInvalidToken     = "OAUTH_INVALID_TOKEN"
	codeOAuthAccountNotLinked = "OAUTH_ACCOUNT_NOT_LINKED"
	codeOAuthEmailUnverified  = "OAUTH_EMAIL_UNVERIFIED"
	codeRegistrationDisabled  = "REGISTRATION_DISABLED"
)

type nativeTokenBody struct {
	IDToken string `json:"id_token"`
	Nonce   string `json:"nonce"`
	// AuthorizationCode (Apple, optional) is exchanged at Apple for a refresh token, stored
	// encrypted to revoke it on unlink / account deletion (TEC-1181). Best effort: a failed
	// exchange never fails the request. Ignored for Google.
	AuthorizationCode string `json:"authorization_code"`
}

// NativeOAuthSignIn is POST /v1/auth/oauth/{provider}/native: an id_token from the Apple / Google
// SDK in the app for a token pair, like login.
func (h *Handler) NativeOAuthSignIn(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.TrimSpace(r.PathValue("provider")))
	if !usecase.IsNativeOAuthProvider(provider) {
		response.NotFound(w, r, "Unknown OAuth provider")
		return
	}
	var in struct {
		nativeTokenBody
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		TOTPCode   string `json:"totp_code"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	meta := sessionMeta(r)
	if !h.allowRates(w, r, rateCheck{"oauth_native_ip", meta.IP, nativeSignInIPLimit}) {
		return
	}
	id, err := h.uc.VerifyNativeIDToken(r.Context(), provider, in.IDToken, in.Nonce)
	if err != nil {
		writeNativeError(w, r, err)
		return
	}
	if !h.allowRates(w, r, rateCheck{"oauth_native_sub", provider + ":" + id.Claims.Subject, nativeSignInSubjectLimit}) {
		return
	}
	tokens, err := h.uc.SignInWithNativeIdentity(r.Context(), id, usecase.NativeSignInInput{
		GivenName: in.GivenName, FamilyName: in.FamilyName, TOTPCode: in.TOTPCode,
		AuthorizationCode: in.AuthorizationCode,
	}, meta)
	if err != nil {
		writeNativeError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, tokens)
}

// NativeLinkIdentity is POST /v1/auth/identities/{provider}/native: links the Apple / Google
// identity of an id_token to the signed-in user and returns the updated identity list.
func (h *Handler) NativeLinkIdentity(w http.ResponseWriter, r *http.Request) {
	p, ok := authctx.PrincipalFrom(r.Context())
	if !ok {
		response.Unauthorized(w, r, "Authentication is required")
		return
	}
	provider := strings.ToLower(strings.TrimSpace(r.PathValue("provider")))
	if !usecase.IsNativeOAuthProvider(provider) {
		response.NotFound(w, r, "Unknown OAuth provider")
		return
	}
	if p.ImpersonatorUserID != nil {
		response.Forbidden(w, r, "Sign-in methods cannot be linked while impersonating")
		return
	}
	var in nativeTokenBody
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if !h.allowRates(w, r,
		rateCheck{"oauth_native_link_user", p.UserID.String(), nativeLinkUserLimit},
		rateCheck{"oauth_native_link_ip", sessionMeta(r).IP, nativeLinkIPLimit},
	) {
		return
	}
	if h.oauth == nil {
		response.Internal(w, r, "OAuth integration is not configured")
		return
	}
	id, err := h.uc.VerifyNativeIDToken(r.Context(), provider, in.IDToken, in.Nonce)
	if err == nil {
		err = usecase.NativeLinkRequirements(id)
	}
	if err == nil {
		err = h.oauth.LinkOAuthAccount(r.Context(), model.LinkOAuthAccountInput{
			UserID: p.UserID.String(), Provider: provider, ProviderAccountID: id.Claims.Subject, Type: "oauth",
		})
	}
	if err != nil {
		writeNativeError(w, r, err)
		return
	}
	h.uc.StoreNativeAppleToken(r.Context(), p.UserInternal, id, in.AuthorizationCode)
	if h.activity != nil {
		uid := p.UserInternal
		h.activity.Record(r.Context(), &uid, "auth.identity.linked", "auth.identities."+provider, nil,
			map[string]any{"provider": provider, "client": sessionMeta(r).ClientType}, r)
	}
	list, err := h.oauth.ListIdentities(r.Context(), p.UserID)
	if err != nil {
		writeUsecaseError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, list)
}

// writeNativeError maps the native sign-in errors and falls back to writeUsecaseError.
func writeNativeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, usecase.ErrOAuthProviderDisabled):
		response.RecordFailure(w, err)
		response.Error(w, r, http.StatusForbidden, codeOAuthProviderDisabled, "Native sign-in is not enabled for this provider")
	case errors.Is(err, usecase.ErrOAuthInvalidToken):
		response.RecordFailure(w, err)
		response.Error(w, r, http.StatusUnauthorized, codeOAuthInvalidToken, "The provider token is invalid or expired")
	case errors.Is(err, usecase.ErrOAuthAccountNotLinked):
		response.RecordFailure(w, err)
		response.Error(w, r, http.StatusConflict, codeOAuthAccountNotLinked, "An account with this email exists. Sign in with your existing method and link this provider from your profile")
	case errors.Is(err, usecase.ErrOAuthEmailUnverified):
		response.RecordFailure(w, err)
		response.Error(w, r, http.StatusForbidden, codeOAuthEmailUnverified, "The provider did not confirm a verified email for this identity")
	case errors.Is(err, usecase.ErrRegistrationDisabled):
		response.RecordFailure(w, err)
		response.Error(w, r, http.StatusForbidden, codeRegistrationDisabled, "Registration is disabled")
	case errors.Is(err, usecase.ErrConflict):
		response.RecordFailure(w, err)
		response.Conflict(w, r, response.CodeConflict, "This identity is linked to another account, or another identity of this provider is already linked")
	default:
		writeUsecaseError(w, r, err)
	}
}
