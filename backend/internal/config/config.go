package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"github.com/joho/godotenv"
)

// Config holds process configuration loaded from the environment.
type Config struct {
	Billing    BillingConfig
	App        AppConfig
	Encryption EncryptionConfig
	HTTP       HTTPConfig
	DB         DBConfig
	Redis      RedisConfig
	Storage    StorageConfig
	CORS       CORSConfig
	SMTP       SMTPConfig
	Queue      QueueConfig
	Bulk       BulkConfig
	Centrifugo CentrifugoConfig
	Auth       AuthConfig
	JWT        JWTConfig
	Log        LogConfig
	VAPID      VAPIDConfig
	Expo       ExpoConfig
	Search     SearchConfig
	Observe    ObservabilityConfig
}

// ObservabilityConfig holds optional error reporting and metrics settings.
// Both are off by default (empty SENTRY_DSN / METRICS_ADDR).
type ObservabilityConfig struct {
	// SentryDSN enables Sentry-compatible error reporting (Sentry or GlitchTip).
	SentryDSN         string
	SentryEnvironment string
	SentryRelease     string
	// MetricsAddr starts a Prometheus /metrics listener on this address (e.g. ":9091").
	MetricsAddr string
}

// AuthConfig holds NextAuth adapter integration settings.
type AuthConfig struct {
	AdapterSecret string
	WebAuthnRPID  string
	FrontendURL   string
	// ReviewAccounts maps a lower-cased email to its fixed App Store / Play review sign-in
	// code (AUTH_REVIEW_ACCOUNTS=email:code,...). Empty = disabled (TEC-1182).
	ReviewAccounts map[string]string
}

// VAPIDConfig holds Web Push keys (empty = push disabled).
type VAPIDConfig struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

// ExpoConfig holds the Expo Push Service settings (mobile push, TEC-1187). AccessToken is
// optional: needed only with Expo "enhanced push security"; sent as a Bearer header.
type ExpoConfig struct {
	AccessToken string
}

// JWTConfig holds access JWT and opaque refresh token settings.
type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	// RefreshTTLMobile is the refresh lifetime of mobile app sessions (TEC-1179).
	RefreshTTLMobile time.Duration
}

type AppConfig struct {
	Name string
	Env  string
}

// EncryptionConfig holds at-rest secret encryption material.
type EncryptionConfig struct {
	Key string
}

type HTTPConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	// TrustedProxies lists CIDRs whose X-Forwarded-For is honoured (TRUSTED_PROXIES).
	TrustedProxies string
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
	MaxConns int32
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.Name,
		c.SSLMode,
	)
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type StorageConfig struct {
	Driver    string // s3 (SeaweedFS in compose, or any S3-compatible store); "minio" is a legacy alias
	LocalPath string
	MaxBytes  int64
	S3        ObjectStoreConfig
}

// ObjectStoreConfig holds S3-compatible credentials (SeaweedFS S3 gateway, AWS S3, ...).
type ObjectStoreConfig struct {
	Endpoint      string
	Region        string
	Bucket        string
	AccessKey     string
	SecretKey     string
	UsePathStyle  bool
	PublicBaseURL string
}

type CORSConfig struct {
	AllowedOrigins []string
}

// SMTPConfig configures outbound email (MailHog in development).
type SMTPConfig struct {
	Driver   string
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	// TLS: auto (implicit TLS on 465, STARTTLS when offered elsewhere) | implicit | starttls | none.
	TLS string
}

type LogConfig struct {
	Level  string
	Format string
}

// QueueConfig controls Asynq background jobs.
type QueueConfig struct {
	Enabled         bool
	Concurrency     int
	WorkerInProcess bool
}

// BulkConfig controls bulk action engine thresholds.
type BulkConfig struct {
	SyncMax       int
	RollbackHours int
}

// CentrifugoConfig controls realtime pub/sub via Centrifugo.
type CentrifugoConfig struct {
	Enabled   bool
	APIURL    string
	APIKey    string
	TokenHMAC string
	WSURL     string
	TokenTTL  time.Duration
}

// SearchConfig controls Meilisearch-backed command palette indexing.
type SearchConfig struct {
	Enabled     bool
	Driver      string
	MeiliHost   string
	MeiliKey    string
	IndexPrefix string
}

// Load reads optional .env then environment variables into Config.
func Load() (Config, error) {
	_ = godotenv.Load("../.env", ".env")

	cfg := Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "api"),
			Env:  getEnv("APP_ENV", EnvDevelopment),
		},
		Encryption: EncryptionConfig{
			Key: getEnv("APP_ENCRYPTION_KEY", defaultEncryptionKey),
		},
		HTTP: HTTPConfig{
			Addr:           getEnv("APP_HTTP_ADDR", ":8080"),
			ReadTimeout:    getDuration("HTTP_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:   getDuration("HTTP_WRITE_TIMEOUT", 120*time.Second),
			IdleTimeout:    getDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
			TrustedProxies: getEnv("TRUSTED_PROXIES", clientip.DefaultTrustedProxies),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "127.0.0.1"),
			Port:     getInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "app"),
			Password: getEnv("DB_PASSWORD", defaultDBPassword),
			Name:     getEnv("DB_NAME", "app"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
			MaxConns: int32(getInt("DB_MAX_CONNS", 20)),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getInt("REDIS_DB", 0),
		},
		Storage: loadStorageConfig(),
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		},
		SMTP: SMTPConfig{
			Driver:   strings.ToLower(getEnv("MAIL_DRIVER", "smtp")),
			Host:     getEnv("SMTP_HOST", "127.0.0.1"),
			Port:     getInt("SMTP_PORT", 1025),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     firstNonEmpty(getEnv("MAIL_FROM", ""), getEnv("SMTP_FROM", "noreply@localhost")),
			FromName: getEnv("MAIL_FROM_NAME", "App"),
			TLS:      strings.ToLower(getEnv("SMTP_TLS", "auto")),
		},
		Queue: QueueConfig{
			Enabled:         getBool("QUEUE_ENABLED", true),
			Concurrency:     getInt("QUEUE_CONCURRENCY", 10),
			WorkerInProcess: getBool("QUEUE_WORKER_INPROCESS", true),
		},
		Bulk: BulkConfig{
			SyncMax:       getInt("BULK_SYNC_MAX", 50),
			RollbackHours: getInt("BULK_ROLLBACK_HOURS", 24),
		},
		Centrifugo: CentrifugoConfig{
			Enabled:   getBool("CENTRIFUGO_ENABLED", true),
			APIURL:    getEnv("CENTRIFUGO_API_URL", "http://127.0.0.1:8000"),
			APIKey:    getEnv("CENTRIFUGO_API_KEY", defaultCentrifugoAPIKey),
			TokenHMAC: getEnv("CENTRIFUGO_TOKEN_HMAC_SECRET", defaultCentrifugoTokenHMAC),
			WSURL:     getEnv("CENTRIFUGO_WS_URL", "ws://127.0.0.1:8000/connection/websocket"),
			TokenTTL:  getDuration("CENTRIFUGO_TOKEN_TTL", time.Hour),
		},
		Auth: AuthConfig{
			AdapterSecret: getEnv("AUTH_ADAPTER_SECRET", defaultAdapterSecret),
			WebAuthnRPID:  getEnv("AUTH_WEBAUTHN_RP_ID", "localhost"),
			FrontendURL:   firstNonEmpty(getEnv("PUBLIC_FRONTEND_URL", ""), "http://localhost:3000"),
		},
		JWT: JWTConfig{
			AccessSecret:  getEnv("JWT_ACCESS_SECRET", defaultJWTAccessSecret),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", defaultJWTRefreshSecret),
			AccessTTL:     getDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL:    getDuration("JWT_REFRESH_TTL", 168*time.Hour),
			// Mobile sessions slide on every refresh; 60 days keeps a rarely opened app signed in.
			RefreshTTLMobile: getDuration("JWT_REFRESH_TTL_MOBILE", 60*24*time.Hour),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "debug"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Observe: ObservabilityConfig{
			SentryDSN:         strings.TrimSpace(getEnv("SENTRY_DSN", "")),
			SentryEnvironment: firstNonEmpty(getEnv("SENTRY_ENVIRONMENT", ""), getEnv("APP_ENV", EnvDevelopment)),
			SentryRelease:     getEnv("SENTRY_RELEASE", ""),
			MetricsAddr:       strings.TrimSpace(getEnv("METRICS_ADDR", "")),
		},
		VAPID: VAPIDConfig{
			PublicKey:  getEnv("VAPID_PUBLIC_KEY", ""),
			PrivateKey: getEnv("VAPID_PRIVATE_KEY", ""),
			Subject:    getEnv("VAPID_SUBJECT", "mailto:noreply@example.com"),
		},
		Expo: ExpoConfig{
			AccessToken: getEnv("EXPO_ACCESS_TOKEN", ""),
		},
		Search: SearchConfig{
			Enabled:     getBool("SEARCH_ENABLED", true),
			Driver:      strings.ToLower(getEnv("SEARCH_DRIVER", "meilisearch")),
			MeiliHost:   getEnv("MEILI_HOST", "http://127.0.0.1:7700"),
			MeiliKey:    getEnv("MEILI_MASTER_KEY", defaultMeiliMasterKey),
			IndexPrefix: getEnv("MEILI_INDEX_PREFIX", "app"),
		},
	}

	reviewAccounts, err := parseReviewAccounts(getEnv("AUTH_REVIEW_ACCOUNTS", ""))
	if err != nil {
		return Config{}, err
	}
	cfg.Auth.ReviewAccounts = reviewAccounts

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	billing, err := loadBilling()
	if err != nil {
		return Config{}, err
	}
	cfg.Billing = billing
	return cfg, nil
}

// validate checks the loaded config. Secret defaults are refused outside
// development/test by secretGuard (guard.go).
func (c Config) validate() error {
	if err := validateAppEnv(c.App.Env); err != nil {
		return err
	}
	if c.App.Name == "" {
		return fmt.Errorf("config: APP_NAME is required")
	}
	if c.HTTP.Addr == "" {
		return fmt.Errorf("config: APP_HTTP_ADDR is required")
	}
	if c.DB.Host == "" || c.DB.User == "" || c.DB.Name == "" {
		return fmt.Errorf("config: database settings are incomplete")
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("config: REDIS_ADDR is required")
	}
	if c.Queue.Concurrency <= 0 {
		return fmt.Errorf("config: QUEUE_CONCURRENCY must be positive")
	}
	if c.JWT.AccessTTL <= 0 || c.JWT.RefreshTTL <= 0 || c.JWT.RefreshTTLMobile <= 0 {
		return fmt.Errorf("config: JWT TTLs must be positive")
	}
	if err := c.secretGuard(); err != nil {
		return err
	}
	if c.JWT.AccessSecret == "" {
		return fmt.Errorf("config: JWT_ACCESS_SECRET is required")
	}
	if c.Search.Enabled {
		if c.Search.MeiliHost == "" {
			return fmt.Errorf("config: MEILI_HOST is required when SEARCH_ENABLED=true")
		}
		if c.Search.IndexPrefix == "" {
			return fmt.Errorf("config: MEILI_INDEX_PREFIX is required when SEARCH_ENABLED=true")
		}
	}
	return nil
}

func loadStorageConfig() StorageConfig {
	s3Endpoint := getEnv("S3_ENDPOINT", "http://127.0.0.1:8333")
	s3Bucket := getEnv("S3_BUCKET", "app")
	s3Public := firstNonEmpty(getEnv("S3_PUBLIC_BASE_URL", ""), getEnv("STORAGE_PUBLIC_BASE_URL", ""))
	if s3Public == "" {
		s3Public = strings.TrimRight(s3Endpoint, "/") + "/" + s3Bucket
	}

	return StorageConfig{
		Driver:    strings.ToLower(getEnv("STORAGE_DRIVER", "s3")),
		LocalPath: getEnv("STORAGE_LOCAL_PATH", "./storage/objects"),
		MaxBytes:  getInt64("STORAGE_MAX_BYTES", 10<<20),
		S3: ObjectStoreConfig{
			Endpoint:      s3Endpoint,
			Region:        getEnv("S3_REGION", "us-east-1"),
			Bucket:        s3Bucket,
			AccessKey:     getEnv("S3_ACCESS_KEY", defaultS3AccessKey),
			SecretKey:     getEnv("S3_SECRET_KEY", defaultS3SecretKey),
			UsePathStyle:  getBool("S3_USE_PATH_STYLE", true),
			PublicBaseURL: s3Public,
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func getBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
