package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/appleauth"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidRequest  = errors.New("invalid request")
	ErrUnknownProvider = errors.New("unknown oauth provider")
)

var knownProviders = map[string]struct{}{
	"google":   {},
	"facebook": {},
	"apple":    {},
}

// IsKnownProvider reports whether provider is google/facebook/apple.
func IsKnownProvider(provider string) bool {
	_, ok := knownProviders[strings.ToLower(strings.TrimSpace(provider))]
	return ok
}

type settingsStore interface {
	GetOAuthProviderSettings(ctx context.Context, provider string) (db.OauthProviderSetting, error)
	ListOAuthProviderSettings(ctx context.Context) ([]db.OauthProviderSetting, error)
	UpdateOAuthProviderSettings(ctx context.Context, arg db.UpdateOAuthProviderSettingsParams) (db.OauthProviderSetting, error)
}

const providerApple = "apple"

// AppleWebClientSecretTTL is the lifetime of the Apple client_secret JWT
// generated for the web (Auth.js) client from the uploaded signing key. Far
// above the frontend provider cache (TEC-1113, which also never outlives
// client_secret_expires_at) and far below Apple's 6-month cap.
const AppleWebClientSecretTTL = 24 * time.Hour

// Service manages oauth_provider_settings rows.
type Service struct {
	q   settingsStore
	box *crypto.SecretBox
	now func() time.Time
}

// New creates an OAuth provider settings service.
func New(q settingsStore, box *crypto.SecretBox) *Service {
	return &Service{q: q, box: box, now: time.Now}
}

// Settings is the public admin payload (no secret values).
type Settings struct {
	Provider               string `json:"provider"`
	LoginEnabled           bool   `json:"login_enabled"`
	RegisterEnabled        bool   `json:"register_enabled"`
	ClientID               string `json:"client_id"`
	ClientSecretConfigured bool   `json:"client_secret_configured"`
	// Apple only: signing key (.p8) metadata. The key itself is write-only and
	// never returned; the fingerprint is a hash of its public half.
	TeamID                *string `json:"team_id,omitempty"`
	KeyID                 *string `json:"key_id,omitempty"`
	PrivateKeyConfigured  *bool   `json:"private_key_configured,omitempty"`
	PrivateKeyFingerprint *string `json:"private_key_fingerprint,omitempty"`
	// ClientSecretSource is what the web client uses at runtime: "signing_key"
	// (generated from the .p8), "client_secret" (pasted JWT) or "none".
	ClientSecretSource *string `json:"client_secret_source,omitempty"`
}

// Apple client secret sources reported in Settings.ClientSecretSource.
const (
	AppleSecretFromKey    = "signing_key"
	AppleSecretFromPasted = "client_secret"
	AppleSecretNone       = "none"
)

// AdapterOAuthConfig is returned to NextAuth via the internal adapter API.
type AdapterOAuthConfig struct {
	Enabled         bool   `json:"enabled"`
	RegisterAllowed bool   `json:"register_allowed"`
	ClientID        string `json:"client_id"`
	ClientSecret    string `json:"client_secret"`
	// ClientSecretExpiresAt is set when the secret was generated (Apple signing
	// key); callers must not use the secret after it.
	ClientSecretExpiresAt *time.Time `json:"client_secret_expires_at,omitempty"`
}

// PatchInput partial update for provider settings.
type PatchInput struct {
	LoginEnabled    *bool   `json:"login_enabled"`
	RegisterEnabled *bool   `json:"register_enabled"`
	ClientID        *string `json:"client_id"`
	ClientSecret    *string `json:"client_secret"`
	// Apple only.
	TeamID           *string `json:"team_id"`
	KeyID            *string `json:"key_id"`
	PrivateKey       *string `json:"private_key"` // .p8 PEM text, write-only
	RemovePrivateKey *bool   `json:"remove_private_key"`
}

func (in PatchInput) touchesAppleKey() bool {
	return in.TeamID != nil || in.KeyID != nil || in.PrivateKey != nil || in.RemovePrivateKey != nil
}

func normalizeProvider(provider string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	if !IsKnownProvider(p) {
		return "", ErrUnknownProvider
	}
	return p, nil
}

func (s *Service) mapSettings(row db.OauthProviderSetting) Settings {
	out := Settings{
		Provider:               row.Provider,
		LoginEnabled:           row.LoginEnabled,
		RegisterEnabled:        row.RegisterEnabled,
		ClientID:               row.ClientID,
		ClientSecretConfigured: hasStoredSecret(row),
	}
	if row.Provider == providerApple {
		teamID, keyID := row.AppleTeamID, row.AppleKeyID
		configured := hasStoredAppleKey(row)
		fingerprint := ""
		if key, err := s.appleSigningKey(row); err == nil && key != nil {
			fingerprint = appleauth.Fingerprint(key.Key)
		}
		source := AppleSecretNone
		switch {
		case fingerprint != "":
			source = AppleSecretFromKey
		case hasStoredSecret(row):
			source = AppleSecretFromPasted
		}
		out.TeamID, out.KeyID = &teamID, &keyID
		out.PrivateKeyConfigured, out.PrivateKeyFingerprint = &configured, &fingerprint
		out.ClientSecretSource = &source
	}
	return out
}

func hasStoredSecret(row db.OauthProviderSetting) bool {
	return row.ClientSecretEnc.Valid && row.ClientSecretEnc.String != ""
}

func hasStoredAppleKey(row db.OauthProviderSetting) bool {
	return row.ApplePrivateKeyEnc.Valid && row.ApplePrivateKeyEnc.String != ""
}

// appleSigningKey decrypts and parses the stored Apple key. nil, nil when the
// row has no complete key (key + Team ID + Key ID). Errors never carry key text.
func (s *Service) appleSigningKey(row db.OauthProviderSetting) (*appleauth.SigningKey, error) {
	if row.Provider != providerApple || !hasStoredAppleKey(row) ||
		row.AppleTeamID == "" || row.AppleKeyID == "" {
		return nil, nil
	}
	pemText, err := s.box.Decrypt(row.ApplePrivateKeyEnc.String)
	if err != nil {
		return nil, errors.New("decrypt apple private key failed")
	}
	key, err := appleauth.ParsePrivateKey(pemText)
	if err != nil {
		return nil, errors.New("stored apple private key is invalid")
	}
	return &appleauth.SigningKey{TeamID: row.AppleTeamID, KeyID: row.AppleKeyID, Key: key}, nil
}

// hasCredentials: client_id plus a pasted secret or (Apple) a stored signing
// key from which the secret is generated.
func hasCredentials(row db.OauthProviderSetting) bool {
	if strings.TrimSpace(row.ClientID) == "" {
		return false
	}
	if hasStoredSecret(row) {
		return true
	}
	return row.Provider == providerApple && hasStoredAppleKey(row) &&
		row.AppleTeamID != "" && row.AppleKeyID != ""
}

// Get returns masked settings for one provider.
func (s *Service) Get(ctx context.Context, provider string) (Settings, error) {
	p, err := normalizeProvider(provider)
	if err != nil {
		return Settings{}, err
	}
	row, err := s.q.GetOAuthProviderSettings(ctx, p)
	if err != nil {
		return Settings{}, err
	}
	return s.mapSettings(row), nil
}

// IsLoginEnabled reports whether provider sign-in is active (credentials + flag).
func (s *Service) IsLoginEnabled(ctx context.Context, provider string) (bool, error) {
	p, err := normalizeProvider(provider)
	if err != nil {
		return false, err
	}
	row, err := s.q.GetOAuthProviderSettings(ctx, p)
	if err != nil {
		return false, err
	}
	return row.LoginEnabled && hasCredentials(row), nil
}

// IsRegisterEnabled reports whether provider self-register is active (credentials + flag).
func (s *Service) IsRegisterEnabled(ctx context.Context, provider string) (bool, error) {
	p, err := normalizeProvider(provider)
	if err != nil {
		return false, err
	}
	row, err := s.q.GetOAuthProviderSettings(ctx, p)
	if err != nil {
		return false, err
	}
	return row.RegisterEnabled && hasCredentials(row), nil
}

// GetAdapterOAuthConfig returns decrypted OAuth credentials when login or register is enabled.
func (s *Service) GetAdapterOAuthConfig(ctx context.Context, provider string) (AdapterOAuthConfig, error) {
	p, err := normalizeProvider(provider)
	if err != nil {
		return AdapterOAuthConfig{}, err
	}
	row, err := s.q.GetOAuthProviderSettings(ctx, p)
	if err != nil {
		return AdapterOAuthConfig{}, err
	}
	if (!row.LoginEnabled && !row.RegisterEnabled) || !hasCredentials(row) {
		return AdapterOAuthConfig{Enabled: false}, nil
	}
	out := AdapterOAuthConfig{
		Enabled:         true,
		RegisterAllowed: row.RegisterEnabled,
		ClientID:        strings.TrimSpace(row.ClientID),
	}
	// Apple: the uploaded signing key wins over a pasted JWT. A fresh
	// client_secret (sub = Services ID) is generated per call, so it never
	// silently expires like a hand-made one.
	if p == providerApple {
		key, keyErr := s.appleSigningKey(row)
		if keyErr == nil && key != nil {
			secret, exp, err := key.ClientSecret(out.ClientID, AppleWebClientSecretTTL, s.now())
			if err == nil {
				out.ClientSecret = secret
				out.ClientSecretExpiresAt = &exp
				return out, nil
			}
			keyErr = err
		}
		if keyErr != nil && !hasStoredSecret(row) {
			return AdapterOAuthConfig{}, fmt.Errorf("apple client secret: %w", keyErr)
		}
	}
	if !hasStoredSecret(row) {
		return AdapterOAuthConfig{Enabled: false}, nil
	}
	secret, err := s.box.Decrypt(row.ClientSecretEnc.String)
	if err != nil {
		return AdapterOAuthConfig{}, fmt.Errorf("decrypt client secret: %w", err)
	}
	out.ClientSecret = secret
	return out, nil
}

// Patch updates provider settings.
func (s *Service) Patch(ctx context.Context, provider string, in PatchInput) (Settings, error) {
	p, err := normalizeProvider(provider)
	if err != nil {
		return Settings{}, err
	}
	current, err := s.q.GetOAuthProviderSettings(ctx, p)
	if err != nil {
		return Settings{}, err
	}

	params := db.UpdateOAuthProviderSettingsParams{Provider: p}
	if in.LoginEnabled != nil {
		params.LoginEnabled = pgtype.Bool{Bool: *in.LoginEnabled, Valid: true}
	}
	if in.RegisterEnabled != nil {
		params.RegisterEnabled = pgtype.Bool{Bool: *in.RegisterEnabled, Valid: true}
	}
	if in.ClientID != nil {
		params.ClientID = pgtype.Text{String: strings.TrimSpace(*in.ClientID), Valid: true}
	}
	if in.ClientSecret != nil {
		trimmed := strings.TrimSpace(*in.ClientSecret)
		if trimmed != "" {
			enc, err := s.box.Encrypt(trimmed)
			if err != nil {
				return Settings{}, err
			}
			params.ClientSecretEnc = pgtype.Text{String: enc, Valid: true}
		}
	}

	keyAvailable := false
	if p == providerApple {
		keyAvailable, err = s.applyAppleKeyPatch(current, in, &params)
		if err != nil {
			return Settings{}, err
		}
	} else if in.touchesAppleKey() {
		return Settings{}, fmt.Errorf("%w: team_id, key_id and private_key are only supported for apple", ErrInvalidRequest)
	}

	nextLogin := current.LoginEnabled
	if in.LoginEnabled != nil {
		nextLogin = *in.LoginEnabled
	}
	nextRegister := current.RegisterEnabled
	if in.RegisterEnabled != nil {
		nextRegister = *in.RegisterEnabled
	}
	nextClientID := current.ClientID
	if in.ClientID != nil {
		nextClientID = strings.TrimSpace(*in.ClientID)
	}
	hasSecret := hasStoredSecret(current)
	if in.ClientSecret != nil && strings.TrimSpace(*in.ClientSecret) != "" {
		hasSecret = true
	}
	if (nextLogin || nextRegister) && (nextClientID == "" || (!hasSecret && !keyAvailable)) {
		if p == providerApple {
			return Settings{}, fmt.Errorf("%w: client_id and either a signing key (.p8) or client_secret are required to enable Apple sign-in", ErrInvalidRequest)
		}
		return Settings{}, fmt.Errorf("%w: client_id and client_secret are required to enable OAuth", ErrInvalidRequest)
	}

	row, err := s.q.UpdateOAuthProviderSettings(ctx, params)
	if err != nil {
		return Settings{}, err
	}
	return s.mapSettings(row), nil
}

// applyAppleKeyPatch validates the Apple signing key fields, fills params and
// reports whether a complete key is stored after this update. Error messages
// never include the submitted key.
func (s *Service) applyAppleKeyPatch(current db.OauthProviderSetting, in PatchInput, params *db.UpdateOAuthProviderSettingsParams) (bool, error) {
	nextTeam := current.AppleTeamID
	if in.TeamID != nil {
		nextTeam = strings.ToUpper(strings.TrimSpace(*in.TeamID))
		params.AppleTeamID = pgtype.Text{String: nextTeam, Valid: true}
	}
	nextKeyID := current.AppleKeyID
	if in.KeyID != nil {
		nextKeyID = strings.ToUpper(strings.TrimSpace(*in.KeyID))
		params.AppleKeyID = pgtype.Text{String: nextKeyID, Valid: true}
	}
	hasKey := hasStoredAppleKey(current)
	newKey := ""
	if in.PrivateKey != nil {
		newKey = strings.TrimSpace(*in.PrivateKey)
	}
	remove := in.RemovePrivateKey != nil && *in.RemovePrivateKey
	switch {
	case remove && newKey != "":
		return false, fmt.Errorf("%w: private_key and remove_private_key cannot be combined", ErrInvalidRequest)
	case remove:
		params.ClearApplePrivateKey = true
		hasKey = false
	case newKey != "":
		if _, err := appleauth.ParsePrivateKey(newKey); err != nil {
			return false, fmt.Errorf("%w: private_key must be an Apple .p8 key (PEM, PKCS#8, EC P-256)", ErrInvalidRequest)
		}
		enc, err := s.box.Encrypt(newKey)
		if err != nil {
			return false, errors.New("encrypt apple private key failed")
		}
		params.ApplePrivateKeyEnc = pgtype.Text{String: enc, Valid: true}
		hasKey = true
	}
	if nextTeam != "" && !appleauth.ValidID(nextTeam) {
		return false, fmt.Errorf("%w: team_id must be 10 letters or digits", ErrInvalidRequest)
	}
	if nextKeyID != "" && !appleauth.ValidID(nextKeyID) {
		return false, fmt.Errorf("%w: key_id must be 10 letters or digits", ErrInvalidRequest)
	}
	if hasKey && (nextTeam == "" || nextKeyID == "") {
		return false, fmt.Errorf("%w: team_id and key_id are required with a private key", ErrInvalidRequest)
	}
	return hasKey, nil
}
