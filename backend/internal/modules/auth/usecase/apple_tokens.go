package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	oauthproviderusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/oauthprovider/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/appleauth"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/logredact"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
)

// Apple token exchange and revocation (TEC-1181, docs/design/mobile.md §2.3).
//
// A native Apple sign-in or link may carry the app's authorization_code. It is exchanged at
// Apple for a refresh token, which is stored encrypted (oauth_accounts.refresh_token_enc) with
// the client id it was issued to (oauth_accounts.client_id, the iOS bundle id). The token is
// kept only to revoke it: when the Apple identity is unlinked and, later, in the account
// deletion cleanup (TEC-1122). Every Apple call is best effort: a failure is logged and never
// blocks the sign-in, link or unlink. Logs never carry a code, token or key.

// ErrAppleTokenNotConfigured: no signing key (or, for a web token, no web client) to talk to
// Apple's token endpoints.
var ErrAppleTokenNotConfigured = errors.New("apple token credentials are not configured")

// appleCallTimeout bounds one exchange or revoke round trip, independent of the request.
const appleCallTimeout = 10 * time.Second

// AppleTokenAPI is Apple's token endpoints (appleauth.TokenClient).
type AppleTokenAPI interface {
	ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (string, error)
	Revoke(ctx context.Context, clientID, clientSecret, refreshToken string) error
}

// AppleCredentialSource returns the stored Apple signing key and web client (oauthprovider
// usecase.Service).
type AppleCredentialSource interface {
	AppleTokenCredentials(ctx context.Context) (oauthproviderusecase.AppleTokenCredentials, error)
}

// AppleTokenStore reads and writes the stored refresh token of a linked identity
// (repository.Postgres).
type AppleTokenStore interface {
	GetOAuthAccountToken(ctx context.Context, userID int64, provider string) (model.OAuthAccountToken, error)
	SetOAuthAccountRefreshToken(ctx context.Context, userID int64, provider, providerAccountID string, refreshTokenEnc, clientID *string) error
}

// AppleTokens exchanges, stores and revokes Apple refresh tokens.
type AppleTokens struct {
	store AppleTokenStore
	box   SecretBox
	creds AppleCredentialSource
	api   AppleTokenAPI
	log   *slog.Logger
	now   func() time.Time
}

// NewAppleTokens wires the Apple token helper. log may be nil (slog.Default).
func NewAppleTokens(store AppleTokenStore, box SecretBox, creds AppleCredentialSource, api AppleTokenAPI, log *slog.Logger) *AppleTokens {
	if log == nil {
		log = slog.Default()
	}
	return &AppleTokens{store: store, box: box, creds: creds, api: api, log: log, now: time.Now}
}

// StoreFromCode exchanges a native authorization code for a refresh token and stores it on the
// user's Apple identity providerAccountID, with clientID (the bundle id the id_token's aud
// matched). Best effort: does nothing without a code, logs and returns on any failure.
func (a *AppleTokens) StoreFromCode(ctx context.Context, userID int64, providerAccountID, clientID, code string) {
	if a == nil || strings.TrimSpace(code) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), appleCallTimeout)
	defer cancel()
	err := a.storeFromCode(ctx, userID, providerAccountID, strings.TrimSpace(clientID), code)
	switch {
	case err == nil:
		a.log.InfoContext(ctx, "auth_apple_token_stored", "request_id", reqid.From(ctx), "user_id", userID)
	case errors.Is(err, ErrAppleTokenNotConfigured):
		a.log.InfoContext(ctx, "auth_apple_code_exchange_skipped", "request_id", reqid.From(ctx),
			"user_id", userID, "reason", "no_signing_key")
	default:
		a.log.WarnContext(ctx, "auth_apple_code_exchange_failed", "request_id", reqid.From(ctx),
			"user_id", userID, "error", logredact.Error(err))
	}
}

func (a *AppleTokens) storeFromCode(ctx context.Context, userID int64, providerAccountID, clientID, code string) error {
	if a.box == nil || clientID == "" {
		return ErrAppleTokenNotConfigured
	}
	creds, err := a.creds.AppleTokenCredentials(ctx)
	if err != nil {
		return err
	}
	if creds.Key == nil {
		return ErrAppleTokenNotConfigured
	}
	secret, _, err := creds.Key.ClientSecret(clientID, appleauth.TokenClientSecretTTL, a.now())
	if err != nil {
		return err
	}
	refresh, err := a.api.ExchangeCode(ctx, clientID, secret, code)
	if err != nil {
		return err
	}
	enc, err := a.box.Encrypt(refresh)
	if err != nil {
		return errors.New("encrypt apple refresh token failed")
	}
	return a.store.SetOAuthAccountRefreshToken(ctx, userID, model.OAuthProviderApple, providerAccountID, &enc, &clientID)
}

// RevokeForUser revokes the user's stored Apple refresh token, if any, and clears it. It is the
// hook for the account deletion cleanup (TEC-1122 §5.3). nil when there is nothing to revoke.
// The caller decides what a failure means; deletion should log it and carry on.
func (a *AppleTokens) RevokeForUser(ctx context.Context, userID int64) error {
	if a == nil {
		return nil
	}
	tok, err := a.store.GetOAuthAccountToken(ctx, userID, model.OAuthProviderApple)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	revoked, err := a.Revoke(ctx, tok)
	if err != nil || !revoked {
		return err
	}
	err = a.store.SetOAuthAccountRefreshToken(ctx, userID, tok.Provider, tok.ProviderAccountID, nil, tok.ClientID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	return err
}

// Revoke revokes one stored token at Apple. revoked is false (and err nil) when the row has no
// refresh token. A native token (ClientID set) is revoked with a client secret generated for
// that client id; a web token (no ClientID) with the web Services ID and a generated secret,
// or the pasted web client secret when no key is stored.
func (a *AppleTokens) Revoke(ctx context.Context, tok model.OAuthAccountToken) (revoked bool, err error) {
	if a == nil || tok.Provider != model.OAuthProviderApple || tok.RefreshTokenEnc == nil || *tok.RefreshTokenEnc == "" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), appleCallTimeout)
	defer cancel()
	if a.box == nil {
		return false, ErrAppleTokenNotConfigured
	}
	refresh, err := a.box.Decrypt(*tok.RefreshTokenEnc)
	if err != nil || refresh == "" {
		return false, errors.New("decrypt apple refresh token failed")
	}
	creds, err := a.creds.AppleTokenCredentials(ctx)
	if err != nil {
		return false, err
	}
	clientID, secret := "", ""
	if tok.ClientID != nil && strings.TrimSpace(*tok.ClientID) != "" {
		clientID = strings.TrimSpace(*tok.ClientID)
	} else {
		clientID, secret = creds.WebClientID, creds.WebClientSecret
	}
	if clientID == "" {
		return false, ErrAppleTokenNotConfigured
	}
	if creds.Key != nil {
		if secret, _, err = creds.Key.ClientSecret(clientID, appleauth.TokenClientSecretTTL, a.now()); err != nil {
			return false, err
		}
	}
	if secret == "" {
		return false, ErrAppleTokenNotConfigured
	}
	if err := a.api.Revoke(ctx, clientID, secret, refresh); err != nil {
		return false, err
	}
	return true, nil
}

// loadForUnlink reads the user's stored Apple token before the identity row is deleted. ok is
// false when there is none (or it cannot be read; that is logged).
func (a *AppleTokens) loadForUnlink(ctx context.Context, userID int64) (model.OAuthAccountToken, bool) {
	if a == nil {
		return model.OAuthAccountToken{}, false
	}
	tok, err := a.store.GetOAuthAccountToken(ctx, userID, model.OAuthProviderApple)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			a.log.WarnContext(ctx, "auth_apple_token_revoke_failed", "request_id", reqid.From(ctx),
				"user_id", userID, "error", logredact.Error(err))
		}
		return model.OAuthAccountToken{}, false
	}
	return tok, tok.RefreshTokenEnc != nil && *tok.RefreshTokenEnc != ""
}

// revokeLogged revokes tok and logs the outcome; it never fails the caller.
func (a *AppleTokens) revokeLogged(ctx context.Context, tok model.OAuthAccountToken) {
	revoked, err := a.Revoke(ctx, tok)
	switch {
	case err != nil:
		a.log.WarnContext(ctx, "auth_apple_token_revoke_failed", "request_id", reqid.From(ctx),
			"user_id", tok.UserID, "error", logredact.Error(err))
	case revoked:
		a.log.InfoContext(ctx, "auth_apple_token_revoked", "request_id", reqid.From(ctx), "user_id", tok.UserID)
	}
}
