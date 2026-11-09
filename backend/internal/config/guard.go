package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Values APP_ENV may take. Anything else stops the process at boot, so a typo
// ("prod") can never quietly run with half the production checks.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// IsLocal reports whether the process runs with development defaults
// (development or test). Staging and production get the full secret guard.
func (a AppConfig) IsLocal() bool {
	return a.Env == EnvDevelopment || a.Env == EnvTest
}

func validateAppEnv(env string) error {
	switch env {
	case EnvDevelopment, EnvTest, EnvStaging, EnvProduction:
		return nil
	}
	hint := map[string]string{
		"dev": EnvDevelopment, "local": EnvDevelopment,
		"prod": EnvProduction, "live": EnvProduction,
		"stage": EnvStaging, "stg": EnvStaging,
	}[strings.ToLower(strings.TrimSpace(env))]
	msg := fmt.Sprintf("config: APP_ENV=%q is not one of %s, %s, %s, %s",
		env, EnvDevelopment, EnvTest, EnvStaging, EnvProduction)
	if hint != "" {
		msg += fmt.Sprintf(" (did you mean %q?)", hint)
	}
	return errors.New(msg)
}

// Public development values: Load's fallbacks, .env.example and the
// compose.local.yml defaults. secretGuard refuses every one of them outside
// development/test. Keep all public secret literals here.
const (
	defaultDBPassword          = "change_me"
	defaultS3AccessKey         = "app-dev-s3-access"
	defaultS3SecretKey         = "app-dev-s3-secret-change-me"
	defaultJWTAccessSecret     = "app-dev-access-secret-change-me-32b"
	defaultJWTRefreshSecret    = "app-dev-refresh-secret-change-me-32b"
	defaultAdapterSecret       = "app-dev-auth-adapter-secret-change-me"
	defaultEncryptionKey       = "app-dev-encryption-key-32bytes!!"
	defaultCentrifugoAPIKey    = "app-centrifugo-api-key-change-me"
	defaultCentrifugoTokenHMAC = "app-centrifugo-token-hmac-change-me"
	defaultMeiliMasterKey      = "app-dev-meili-master-key-change-me"
)

// commonPlaceholders are refused for every guarded secret: values people paste
// from docs or leave in a copied env file.
var commonPlaceholders = []string{
	"change_me", "changeme", "change-me", "secret", "password", "minioadmin",
}

// guardedSecret is one secret env key the backend reads. TEC-1092 (env schema)
// will tie this table to the `secret:` keys of deploy/env/schema, so a secret
// added there cannot skip the guard.
type guardedSecret struct {
	Key   string
	Value func(Config) string
	// MinLen is the shortest value accepted outside development/test.
	MinLen int
	// Defaults are this key's public development values (refused outside
	// development/test, together with commonPlaceholders).
	Defaults []string
	// Applies limits the check to configs that use the secret (nil = always).
	Applies func(Config) bool
}

// guardedSecrets is the single table of secrets checked at boot.
//
// Payment provider credentials (iyzico, PayTR) are not env vars in this app:
// they are stored in payment_provider_settings, encrypted with
// APP_ENCRYPTION_KEY, so guarding that key covers them.
var guardedSecrets = []guardedSecret{
	{Key: "DB_PASSWORD", Value: func(c Config) string { return c.DB.Password }, MinLen: 12, Defaults: []string{defaultDBPassword}},
	// Development Redis has no password (empty is its default).
	{Key: "REDIS_PASSWORD", Value: func(c Config) string { return c.Redis.Password }, MinLen: 12},
	{Key: "S3_ACCESS_KEY", Value: func(c Config) string { return c.Storage.S3.AccessKey }, MinLen: 8, Defaults: []string{defaultS3AccessKey}},
	{Key: "S3_SECRET_KEY", Value: func(c Config) string { return c.Storage.S3.SecretKey }, MinLen: 16, Defaults: []string{defaultS3SecretKey}},
	{Key: "JWT_ACCESS_SECRET", Value: func(c Config) string { return c.JWT.AccessSecret }, MinLen: 32, Defaults: []string{defaultJWTAccessSecret}},
	{Key: "JWT_REFRESH_SECRET", Value: func(c Config) string { return c.JWT.RefreshSecret }, MinLen: 32, Defaults: []string{defaultJWTRefreshSecret}},
	{Key: "AUTH_ADAPTER_SECRET", Value: func(c Config) string { return c.Auth.AdapterSecret }, MinLen: 32, Defaults: []string{defaultAdapterSecret}},
	// Encrypts OAuth/TOTP/payment/webhook credentials at rest: a public key
	// would expose all of them.
	{Key: "APP_ENCRYPTION_KEY", Value: func(c Config) string { return c.Encryption.Key }, MinLen: 32, Defaults: []string{defaultEncryptionKey}},
	{
		Key: "CENTRIFUGO_API_KEY", Value: func(c Config) string { return c.Centrifugo.APIKey },
		MinLen: 16, Defaults: []string{defaultCentrifugoAPIKey}, Applies: func(c Config) bool { return c.Centrifugo.Enabled },
	},
	{
		Key: "CENTRIFUGO_TOKEN_HMAC_SECRET", Value: func(c Config) string { return c.Centrifugo.TokenHMAC },
		MinLen: 32, Defaults: []string{defaultCentrifugoTokenHMAC}, Applies: func(c Config) bool { return c.Centrifugo.Enabled },
	},
	{
		// Meilisearch itself requires a master key of at least 16 bytes in production.
		Key: "MEILI_MASTER_KEY", Value: func(c Config) string { return c.Search.MeiliKey },
		MinLen: 16, Defaults: []string{defaultMeiliMasterKey}, Applies: func(c Config) bool { return c.Search.Enabled },
	},
}

// secretGuard refuses empty, short or public development secrets outside
// development/test and names every offending key at once (never a value).
func (c Config) secretGuard() error {
	if c.App.IsLocal() {
		return nil
	}
	var problems []string
	for _, s := range guardedSecrets {
		if s.Applies != nil && !s.Applies(c) {
			continue
		}
		v := s.Value(c)
		switch {
		case strings.TrimSpace(v) == "":
			problems = append(problems, s.Key+" is required")
		case slices.Contains(s.Defaults, v) || slices.Contains(commonPlaceholders, strings.ToLower(v)):
			problems = append(problems, s.Key+" still has a public default value")
		case len(v) < s.MinLen:
			problems = append(problems, fmt.Sprintf("%s must be at least %d characters", s.Key, s.MinLen))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("config: refusing to start with APP_ENV=%s: %s (generate secrets with `openssl rand -base64 32`, see docs/DEVELOPMENT.md)",
		c.App.Env, strings.Join(problems, "; "))
}
