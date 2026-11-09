package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// strongSecrets are production-shaped values for every guarded key.
var strongSecrets = map[string]string{
	"DB_PASSWORD":                  "db-Pr0d-pass-9f8e7d6c5b4a",
	"REDIS_PASSWORD":               "redis-Pr0d-pass-1a2b3c4d5e",
	"S3_ACCESS_KEY":                "AKIAPRODACCESS42",
	"S3_SECRET_KEY":                "s3-prod-secret-0123456789abcdef",
	"JWT_ACCESS_SECRET":            "jwt-access-prod-0123456789abcdef0123456789",
	"JWT_REFRESH_SECRET":           "jwt-refresh-prod-0123456789abcdef012345678",
	"AUTH_ADAPTER_SECRET":          "adapter-prod-0123456789abcdef0123456789ab",
	"APP_ENCRYPTION_KEY":           "q83vEjRWeJq83vEjRWeJq83vEjRWeJq83vEjRWeJq8M=",
	"CENTRIFUGO_API_KEY":           "centrifugo-api-prod-0123456789",
	"CENTRIFUGO_TOKEN_HMAC_SECRET": "centrifugo-hmac-prod-0123456789abcdef0123",
	"MEILI_MASTER_KEY":             "meili-master-prod-0123456789",
}

// setEnv sets APP_ENV and every guarded key; values override strongSecrets.
// Feature toggles are pinned so the shell environment cannot leak in.
func setEnv(t *testing.T, appEnv string, values map[string]string) {
	t.Helper()
	t.Setenv("APP_ENV", appEnv)
	t.Setenv("CENTRIFUGO_ENABLED", "true")
	t.Setenv("SEARCH_ENABLED", "true")
	for _, s := range guardedSecrets {
		v, ok := values[s.Key]
		if !ok {
			v = strongSecrets[s.Key]
		}
		t.Setenv(s.Key, v)
	}
}

func TestStrongSecretsCoverEveryGuardedKey(t *testing.T) {
	for _, s := range guardedSecrets {
		v, ok := strongSecrets[s.Key]
		if !ok {
			t.Fatalf("strongSecrets has no value for %s", s.Key)
		}
		if len(v) < s.MinLen {
			t.Fatalf("strongSecrets[%s] is shorter than its MinLen", s.Key)
		}
	}
}

func TestLoadProductionWithStrongSecrets(t *testing.T) {
	for _, env := range []string{EnvStaging, EnvProduction} {
		setEnv(t, env, nil)
		if _, err := Load(); err != nil {
			t.Fatalf("APP_ENV=%s with strong secrets: %v", env, err)
		}
	}
}

// Each guarded secret, left at an empty, default, placeholder or short value
// in production, stops boot and the error names the key but not the value.
func TestLoadProductionRefusesEachGuardedSecret(t *testing.T) {
	for _, s := range guardedSecrets {
		bad := append([]string{}, s.Defaults...)
		bad = append(bad, "change_me", "CHANGEME", strings.Repeat("x", s.MinLen-1))
		for _, v := range bad {
			for _, env := range []string{EnvStaging, EnvProduction} {
				setEnv(t, env, map[string]string{s.Key: v})
				_, err := Load()
				if err == nil {
					t.Fatalf("APP_ENV=%s %s=%q: expected boot error", env, s.Key, v)
				}
				msg := err.Error()
				if !strings.Contains(msg, s.Key) {
					t.Fatalf("APP_ENV=%s %s=%q: error does not name the key: %v", env, s.Key, v, err)
				}
				if len(v) >= 4 && strings.Contains(msg, v) {
					t.Fatalf("APP_ENV=%s %s: error leaks the value: %v", env, s.Key, err)
				}
			}
		}
	}
}

// Unset in production: the key falls back to its public default (or stays
// empty for REDIS_PASSWORD), which must be refused, not silently used.
func TestLoadProductionRefusesUnsetSecrets(t *testing.T) {
	for _, s := range guardedSecrets {
		setEnv(t, EnvProduction, map[string]string{s.Key: ""})
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), s.Key) {
			t.Fatalf("%s unset in production: expected error naming it, got %v", s.Key, err)
		}
	}
}

func TestLoadProductionListsAllOffenders(t *testing.T) {
	values := map[string]string{}
	for _, s := range guardedSecrets {
		values[s.Key] = ""
	}
	setEnv(t, EnvProduction, values)
	_, err := Load()
	if err == nil {
		t.Fatal("expected boot error with every secret at its default")
	}
	for _, s := range guardedSecrets {
		if !strings.Contains(err.Error(), s.Key) {
			t.Errorf("error does not list %s: %v", s.Key, err)
		}
	}
	for _, v := range []string{defaultEncryptionKey, defaultJWTAccessSecret, defaultDBPassword} {
		if strings.Contains(err.Error(), v) {
			t.Fatalf("error leaks a secret value: %v", err)
		}
	}
}

func TestLoadProductionSkipsDisabledFeatures(t *testing.T) {
	setEnv(t, EnvProduction, map[string]string{
		"CENTRIFUGO_API_KEY": "", "CENTRIFUGO_TOKEN_HMAC_SECRET": "", "MEILI_MASTER_KEY": "",
	})
	t.Setenv("CENTRIFUGO_ENABLED", "false")
	t.Setenv("SEARCH_ENABLED", "false")
	if _, err := Load(); err != nil {
		t.Fatalf("disabled Centrifugo/search keys must not be required: %v", err)
	}
}

// Development and test keep booting with the public defaults (make local-dev,
// scripts/e2e.sh, CI).
func TestLoadLocalEnvsAcceptDefaults(t *testing.T) {
	for _, env := range []string{EnvDevelopment, EnvTest} {
		values := map[string]string{}
		for _, s := range guardedSecrets {
			values[s.Key] = ""
		}
		setEnv(t, env, values)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("APP_ENV=%s with fallbacks: %v", env, err)
		}
		if cfg.Encryption.Key != defaultEncryptionKey {
			t.Fatalf("APP_ENV=%s: expected the development encryption key fallback", env)
		}
	}
	// APP_ENV unset defaults to development.
	setEnv(t, "", nil)
	cfg, err := Load()
	if err != nil || cfg.App.Env != EnvDevelopment {
		t.Fatalf("unset APP_ENV: env=%q err=%v", cfg.App.Env, err)
	}
}

func TestLoadRejectsUnknownAppEnv(t *testing.T) {
	for env, hint := range map[string]string{"prod": EnvProduction, "Production": "", "dev": EnvDevelopment, "qa": ""} {
		setEnv(t, env, nil)
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "APP_ENV") {
			t.Fatalf("APP_ENV=%q: expected APP_ENV error, got %v", env, err)
		}
		if hint != "" && !strings.Contains(err.Error(), hint) {
			t.Fatalf("APP_ENV=%q: expected hint %q, got %v", env, hint, err)
		}
	}
}

// Every guarded key's value in .env.example (copied to start an env file) is
// refused outside development.
func TestEnvExampleSecretsAreRefused(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("open .env.example: %v", err)
	}
	defer func() { _ = f.Close() }()
	example := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			example[k] = v
		}
	}
	for _, s := range guardedSecrets {
		v, ok := example[s.Key]
		if !ok {
			t.Errorf(".env.example has no %s", s.Key)
			continue
		}
		setEnv(t, EnvProduction, map[string]string{s.Key: v})
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), s.Key) {
			t.Errorf(".env.example %s value is accepted in production (err=%v)", s.Key, err)
		}
	}
}
