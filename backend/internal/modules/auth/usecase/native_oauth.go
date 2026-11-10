package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	oauthproviderusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/oauthprovider/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/oidc"
	"github.com/google/uuid"
)

// Native (mobile) Apple / Google sign-in (TEC-1180, docs/design/mobile.md §2.3).
var (
	// ErrOAuthProviderDisabled: the provider is off or has no native audience configured.
	ErrOAuthProviderDisabled = errors.New("oauth provider is disabled for native sign-in")
	// ErrOAuthInvalidToken: the id_token failed signature or claim checks.
	ErrOAuthInvalidToken = errors.New("oauth id token is invalid")
	// ErrOAuthAccountNotLinked: no linked identity, but the email belongs to an existing
	// account. The user signs in with an existing method and links the provider first.
	ErrOAuthAccountNotLinked = errors.New("oauth identity is not linked to the account with this email")
	// ErrOAuthEmailUnverified: the token has no verified email, so it can neither match an
	// existing account, create one nor be linked.
	ErrOAuthEmailUnverified = errors.New("oauth identity has no verified email")
	// ErrRegistrationDisabled: a new account would be needed, but sign-up is closed.
	ErrRegistrationDisabled = errors.New("registration is disabled")
)

const maxNativeNameLen = 100

// IDTokenVerifier checks a provider id_token (oidc.Verifier).
type IDTokenVerifier interface {
	Verify(ctx context.Context, provider, raw string, audiences []string, nonce string) (oidc.Claims, error)
}

// NativeProviderSettings returns a provider's native audiences (oauthprovider usecase.Service).
type NativeProviderSettings interface {
	NativeConfig(ctx context.Context, provider string) (oauthproviderusecase.NativeConfig, error)
}

// SetNativeOAuth attaches id_token verification for native Apple / Google sign-in. Without it
// the native endpoints answer ErrOAuthProviderDisabled.
func (u *AuthUseCase) SetNativeOAuth(verifier IDTokenVerifier, settings NativeProviderSettings) {
	u.idTokens = verifier
	u.nativeSettings = settings
}

// SetAppleTokens attaches the Apple authorization code exchange (TEC-1181). Without it a
// native Apple authorization_code is ignored.
func (u *AuthUseCase) SetAppleTokens(a *AppleTokens) { u.appleTokens = a }

// AppleTokens returns the attached Apple token helper (nil when off), for the account
// deletion cleanup (TEC-1122) and the identity unlink.
func (u *AuthUseCase) AppleTokens() *AppleTokens { return u.appleTokens }

// StoreNativeAppleToken exchanges the authorization_code of a verified native Apple identity
// linked to userID and stores the refresh token (best effort, never fails; TEC-1181). The
// client id is the bundle id the id_token's aud matched, not the web Services ID.
func (u *AuthUseCase) StoreNativeAppleToken(ctx context.Context, userID int64, id NativeIdentity, code string) {
	if u.appleTokens == nil || id.Claims.Provider != model.OAuthProviderApple {
		return
	}
	u.appleTokens.StoreFromCode(ctx, userID, id.Claims.Subject, id.Claims.Audience, code)
}

// NativeIdentity is a verified native id_token.
type NativeIdentity struct {
	Claims oidc.Claims
	// RegisterAllowed is the provider's register flag at verification time.
	RegisterAllowed bool
}

// NativeSignInInput is the body of POST /v1/auth/oauth/{provider}/native after verification.
type NativeSignInInput struct {
	// GivenName / FamilyName: Apple sends the name to the app on the first authorization
	// only, never in the id_token; used for a new account when the token has none.
	GivenName  string
	FamilyName string
	TOTPCode   string
	// AuthorizationCode (Apple) is exchanged for a refresh token after a successful sign-in
	// (TEC-1181); a failed exchange never fails the sign-in.
	AuthorizationCode string
}

// IsNativeOAuthProvider reports whether provider has native sign-in (apple, google).
func IsNativeOAuthProvider(provider string) bool {
	return provider == model.OAuthProviderApple || provider == model.OAuthProviderGoogle
}

// VerifyNativeIDToken checks an id_token from the app against the provider's native
// audiences. nonce is required: the app generates it per sign-in and hands its SHA-256 to
// the provider SDK, so a token captured elsewhere cannot be replayed without it.
func (u *AuthUseCase) VerifyNativeIDToken(ctx context.Context, provider, idToken, nonce string) (NativeIdentity, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if !IsNativeOAuthProvider(provider) {
		return NativeIdentity{}, ErrNotFound
	}
	if u.idTokens == nil || u.nativeSettings == nil {
		return NativeIdentity{}, ErrOAuthProviderDisabled
	}
	cfg, err := u.nativeSettings.NativeConfig(ctx, provider)
	if err != nil {
		return NativeIdentity{}, err
	}
	if !cfg.Enabled || len(cfg.Audiences) == 0 {
		return NativeIdentity{}, ErrOAuthProviderDisabled
	}
	idToken, nonce = strings.TrimSpace(idToken), strings.TrimSpace(nonce)
	if idToken == "" || nonce == "" {
		return NativeIdentity{}, fmt.Errorf("%w: id_token and nonce are required", ErrInvalidRequest)
	}
	claims, err := u.idTokens.Verify(ctx, provider, idToken, cfg.Audiences, nonce)
	if err != nil {
		if errors.Is(err, oidc.ErrInvalidToken) || errors.Is(err, oidc.ErrUnknownProvider) {
			return NativeIdentity{}, fmt.Errorf("%w: %v", ErrOAuthInvalidToken, err)
		}
		return NativeIdentity{}, err
	}
	return NativeIdentity{Claims: claims, RegisterAllowed: cfg.RegisterAllowed}, nil
}

// SignInWithNativeIdentity signs in with a verified native identity. Tokens always start on
// the personal account.
//
//   - Linked identity: sign in (authenticator 2FA users get ErrMFARequired, then retry with
//     the same id_token plus TOTPCode). The admin "2FA for password sign-in" policy does not
//     apply, as for web OAuth.
//   - Not linked, verified email of an existing account: ErrOAuthAccountNotLinked. The
//     account is never matched by email automatically.
//   - Not linked, unverified or missing email: ErrOAuthEmailUnverified.
//   - Not linked, new verified email: a passwordless account is created and linked when
//     self-registration and the provider's register flag are on (ErrRegistrationDisabled
//     otherwise).
func (u *AuthUseCase) SignInWithNativeIdentity(ctx context.Context, id NativeIdentity, in NativeSignInInput, meta model.SessionMeta) (model.Tokens, error) {
	c := id.Claims
	linked, err := u.repo.GetOAuthAccountByProviderAccount(ctx, c.Provider, c.Subject)
	if err == nil {
		return u.signInLinkedNative(ctx, linked.UserUUID, id, in, meta)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return model.Tokens{}, err
	}
	if !c.EmailVerified || c.Email == "" {
		return model.Tokens{}, ErrOAuthEmailUnverified
	}
	if _, err := u.repo.FindUserByEmail(ctx, c.Email); err == nil {
		return model.Tokens{}, ErrOAuthAccountNotLinked
	} else if !errors.Is(err, repository.ErrNotFound) {
		return model.Tokens{}, err
	}
	if !id.RegisterAllowed || !u.selfRegistrationOpen(ctx) {
		return model.Tokens{}, ErrRegistrationDisabled
	}

	name := firstNonEmpty(c.GivenName, in.GivenName, emailLocalPart(c.Email))
	surname := firstNonEmpty(c.FamilyName, in.FamilyName)
	user, err := u.createPasswordlessUser(ctx, c.Email, clampName(name), clampName(surname), true)
	if errors.Is(err, ErrConflict) {
		// Created concurrently (double submit): sign in only if that request linked this identity.
		if linked, lerr := u.repo.GetOAuthAccountByProviderAccount(ctx, c.Provider, c.Subject); lerr == nil {
			return u.signInLinkedNative(ctx, linked.UserUUID, id, in, meta)
		}
		return model.Tokens{}, ErrOAuthAccountNotLinked
	}
	if err != nil {
		return model.Tokens{}, err
	}
	if _, err := u.repo.CreateOAuthAccount(ctx, model.CreateOAuthAccountInput{
		UserID: user.ID, Provider: c.Provider, ProviderAccountID: c.Subject, Type: "oauth",
	}); err != nil {
		return model.Tokens{}, err
	}
	if err := u.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		return model.Tokens{}, err
	}
	return u.issueNativeTokens(ctx, user, id, in.AuthorizationCode, meta)
}

// issueNativeTokens issues the token pair and, once it exists, stores the Apple refresh token
// for the authorization code (only a successful sign-in uses the code: an MFA round resends it).
func (u *AuthUseCase) issueNativeTokens(ctx context.Context, user model.User, id NativeIdentity, code string, meta model.SessionMeta) (model.Tokens, error) {
	tokens, err := u.issueTokensForUser(ctx, user, meta)
	if err != nil {
		return model.Tokens{}, err
	}
	u.StoreNativeAppleToken(ctx, user.ID, id, code)
	return tokens, nil
}

func (u *AuthUseCase) signInLinkedNative(ctx context.Context, userUUID uuid.UUID, id NativeIdentity, in NativeSignInInput, meta model.SessionMeta) (model.Tokens, error) {
	totpCode := in.TOTPCode
	user, err := u.repo.FindUserByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Tokens{}, ErrInvalidCredentials
		}
		return model.Tokens{}, err
	}
	if user.Status != "active" {
		return model.Tokens{}, ErrUserDisabled
	}
	hasTOTP, err := u.UserHasEnabledTOTP(ctx, user.ID)
	if err != nil {
		return model.Tokens{}, err
	}
	if hasTOTP {
		code := strings.TrimSpace(totpCode)
		if code == "" {
			return model.Tokens{}, ErrMFARequired
		}
		valid, _, err := u.verifyUserTOTPCode(ctx, user.ID, code, true)
		if err != nil {
			return model.Tokens{}, err
		}
		if !valid {
			return model.Tokens{}, ErrInvalidMFACode
		}
	}
	if err := u.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		return model.Tokens{}, err
	}
	return u.issueNativeTokens(ctx, user, id, in.AuthorizationCode, meta)
}

// NativeLinkRequirements enforces what linking a native identity to a signed-in user needs:
// a verified email on the token (an unverified one never links).
func NativeLinkRequirements(id NativeIdentity) error {
	if !id.Claims.EmailVerified || id.Claims.Email == "" {
		return ErrOAuthEmailUnverified
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

func clampName(s string) string {
	if r := []rune(strings.TrimSpace(s)); len(r) > maxNativeNameLen {
		return string(r[:maxNativeNameLen])
	}
	return strings.TrimSpace(s)
}
