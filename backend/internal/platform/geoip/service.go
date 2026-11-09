package geoip

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/geoip/updater"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

// Service coordinates GeoIP resolution, caching, settings, and atomic hot reloading.
type Service struct {
	q         *db.Queries
	secretBox *crypto.SecretBox
	rdb       *redis.Client
	log       *slog.Logger

	mu        sync.RWMutex
	db        GeoIPDatabase
	memCache  *MemoryCache
	rdCache   *RedisCache
	settings  Settings
	settingsAt time.Time
}

// NewService creates a new GeoIP service.
func NewService(q *db.Queries, secretBox *crypto.SecretBox, rdb *redis.Client, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}

	svc := &Service{
		q:         q,
		secretBox: secretBox,
		rdb:       rdb,
		log:       log,
		memCache:  NewMemoryCache(100000, 24*time.Hour),
		rdCache:   NewRedisCache(rdb, "geoip:cache:", 24*time.Hour),
		db:        NewNoop(),
		settings:  DefaultSettings(),
	}

	// Try loading initial settings and database
	_ = svc.loadSettingsAndDB(context.Background())
	return svc
}

// Resolve looks up the geolocation for an IP address with layered caching.
func (s *Service) Resolve(ctx context.Context, ip net.IP) (*GeoLocation, error) {
	if ip == nil {
		return &GeoLocation{}, nil
	}

	s.mu.RLock()
	enabled := s.settings.Enabled
	cacheEnabled := s.settings.CacheEnabled
	memCache := s.memCache
	rdCache := s.rdCache
	dbProvider := s.db
	s.mu.RUnlock()

	if !enabled {
		return &GeoLocation{}, nil
	}

	ipStr := ip.String()

	// 1. Layer 1: In-memory cache
	if cacheEnabled && memCache != nil {
		if loc, hit := memCache.Get(ipStr); hit {
			return loc, nil
		}
	}

	// 2. Layer 2: Redis cache
	if cacheEnabled && rdCache != nil {
		if loc, err := rdCache.Get(ctx, ip); err == nil && loc != nil {
			if memCache != nil {
				memCache.Set(ipStr, loc, 0)
			}
			return loc, nil
		}
	}

	// 3. Layer 3: Underlying GeoIP Database
	if dbProvider == nil {
		return &GeoLocation{}, nil
	}

	loc, err := dbProvider.Lookup(ip)
	if err != nil {
		return &GeoLocation{}, err
	}
	if loc == nil {
		loc = &GeoLocation{}
	}

	// Store in caches
	if cacheEnabled {
		if memCache != nil {
			memCache.Set(ipStr, loc, 0)
		}
		if rdCache != nil {
			_ = rdCache.Set(ctx, ip, loc, 0)
		}
	}

	return loc, nil
}

// Reload reloads the database provider from disk using current settings without restart.
func (s *Service) Reload(ctx context.Context) error {
	if err := s.loadSettingsAndDB(ctx); err != nil {
		s.log.Error("geoip_reload_failed", "error", err)
		return err
	}
	s.log.Info("geoip_reloaded_successfully", "version", s.db.Version())
	return nil
}

// UpdateNow triggers an on-demand download, verification, and hot reload of the GeoIP database.
func (s *Service) UpdateNow(ctx context.Context) (*updater.Result, error) {
	_ = s.loadSettingsAndDB(ctx)

	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()

	destPath := settings.DatabasePath
	if destPath == "" {
		destPath = DefaultDatabasePath
	}

	opts := updater.UpdateOptions{
		DestinationPath: destPath,
		AccountID:       settings.AccountID,
		LicenseKey:      settings.LicenseKey,
		CustomURL:       settings.DownloadURLTemplate,
	}

	result, err := updater.DownloadAndVerify(ctx, opts)
	now := time.Now().UTC()

	if err != nil {
		errMsg := err.Error()
		if s.q != nil {
			_, _ = s.q.UpdateGeoIPStatus(ctx, db.UpdateGeoIPStatusParams{
				LastUpdateAt:     pgtype.Timestamptz{Time: now, Valid: true},
				LastUpdateStatus: "failed",
				LastUpdateError:  pgtype.Text{String: errMsg, Valid: true},
				DatabaseVersion:  pgtype.Text{String: settings.DatabaseVersion, Valid: settings.DatabaseVersion != ""},
			})
		}
		s.log.Error("geoip_update_failed", "error", err)
		return nil, err
	}

	// DB file is atomically replaced on disk. Now hot reload in-memory reader.
	if s.q != nil {
		_, _ = s.q.UpdateGeoIPStatus(ctx, db.UpdateGeoIPStatusParams{
			LastUpdateAt:     pgtype.Timestamptz{Time: now, Valid: true},
			LastUpdateStatus: "success",
			LastUpdateError:  pgtype.Text{},
			DatabaseVersion:  pgtype.Text{String: result.Version, Valid: true},
		})
	}

	_ = s.Reload(ctx)
	_ = s.ClearCache(ctx)

	s.log.Info("geoip_update_success", "version", result.Version, "bytes", result.Bytes)
	return result, nil
}

// ClearCache flushes memory and redis caches.
func (s *Service) ClearCache(ctx context.Context) error {
	s.mu.Lock()
	if s.memCache != nil {
		s.memCache.Clear()
	}
	rdCache := s.rdCache
	s.mu.Unlock()

	if rdCache != nil {
		_ = rdCache.Clear(ctx)
	}
	return nil
}

// Status returns current operational status of the GeoIP engine.
func (s *Service) Status(ctx context.Context) (*Status, error) {
	s.mu.RLock()
	settings := s.settings
	dbProv := s.db
	memSize := 0
	if s.memCache != nil {
		memSize = s.memCache.Size()
	}
	s.mu.RUnlock()

	isLoaded := dbProv != nil && dbProv.Version() != "noop"

	return &Status{
		Enabled:          settings.Enabled,
		Provider:         settings.Provider,
		DatabasePath:     settings.DatabasePath,
		DatabaseLoaded:   isLoaded,
		DatabaseVersion:  settings.DatabaseVersion,
		LastUpdateAt:     settings.LastUpdateAt,
		LastUpdateStatus: settings.LastUpdateStatus,
		LastUpdateError:  settings.LastUpdateError,
		CacheSize:        memSize,
	}, nil
}

// GetSettings returns sanitized settings.
func (s *Service) GetSettings(ctx context.Context) (Settings, error) {
	s.mu.RLock()
	st := s.settings
	s.mu.RUnlock()

	// Mask secret fields
	st.AccountID = maskSecret(st.AccountID)
	st.LicenseKey = maskSecret(st.LicenseKey)
	return st, nil
}

// UpdateSettings updates GeoIP configuration.
func (s *Service) UpdateSettings(ctx context.Context, newSettings Settings) (*Settings, error) {
	if s.q == nil {
		return nil, errors.New("queries not initialized")
	}

	dbPath := newSettings.DatabasePath
	if dbPath == "" {
		dbPath = DefaultDatabasePath
	}
	provider := newSettings.Provider
	if provider == "" {
		provider = "maxmind"
	}
	dbType := newSettings.DatabaseType
	if dbType == "" {
		dbType = "geolite2-city"
	}
	updateInterval := newSettings.UpdateIntervalHours
	if updateInterval <= 0 {
		updateInterval = 24
	}
	maxEntries := newSettings.CacheMaxEntries
	if maxEntries <= 0 {
		maxEntries = 100000
	}
	ttlHours := int32(newSettings.CacheTTL / time.Hour)
	if ttlHours <= 0 {
		ttlHours = 24
	}

	var encAcc, encLic pgtype.Text
	if newSettings.AccountID != "" && !isMasked(newSettings.AccountID) && s.secretBox != nil {
		enc, err := s.secretBox.Encrypt(newSettings.AccountID)
		if err == nil {
			encAcc = pgtype.Text{String: enc, Valid: true}
		}
	}
	if newSettings.LicenseKey != "" && !isMasked(newSettings.LicenseKey) && s.secretBox != nil {
		enc, err := s.secretBox.Encrypt(newSettings.LicenseKey)
		if err == nil {
			encLic = pgtype.Text{String: enc, Valid: true}
		}
	}

	row, err := s.q.UpdateGeoIPSettings(ctx, db.UpdateGeoIPSettingsParams{
		Enabled:               pgtype.Bool{Bool: newSettings.Enabled, Valid: true},
		Provider:              pgtype.Text{String: provider, Valid: true},
		DatabaseType:          pgtype.Text{String: dbType, Valid: true},
		DatabasePath:          pgtype.Text{String: dbPath, Valid: true},
		AutoUpdateEnabled:     pgtype.Bool{Bool: newSettings.AutoUpdateEnabled, Valid: true},
		UpdateIntervalHours:   pgtype.Int4{Int32: int32(updateInterval), Valid: true},
		AccountIDEncrypted:    encAcc,
		LicenseKeyEncrypted:   encLic,
		DownloadUrlTemplate:   pgtype.Text{String: newSettings.DownloadURLTemplate, Valid: true},
		CacheEnabled:          pgtype.Bool{Bool: newSettings.CacheEnabled, Valid: true},
		CacheTtlHours:         pgtype.Int4{Int32: ttlHours, Valid: true},
		CacheMaxEntries:       pgtype.Int4{Int32: int32(maxEntries), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("update geoip settings: %w", err)
	}

	_ = s.loadSettingsAndDB(ctx)
	st := s.mapRowToSettings(row)
	st.AccountID = maskSecret(st.AccountID)
	st.LicenseKey = maskSecret(st.LicenseKey)
	return &st, nil
}

func (s *Service) loadSettingsAndDB(ctx context.Context) error {
	if s.q == nil {
		return nil
	}

	row, err := s.q.GetGeoIPSettings(ctx)
	if err != nil {
		return err
	}

	st := s.mapRowToSettings(row)

	// Decrypt secrets
	if s.secretBox != nil {
		if row.AccountIDEncrypted.Valid && row.AccountIDEncrypted.String != "" {
			if dec, err := s.secretBox.Decrypt(row.AccountIDEncrypted.String); err == nil {
				st.AccountID = dec
			}
		}
		if row.LicenseKeyEncrypted.Valid && row.LicenseKeyEncrypted.String != "" {
			if dec, err := s.secretBox.Decrypt(row.LicenseKeyEncrypted.String); err == nil {
				st.LicenseKey = dec
			}
		}
	}

	var newDB GeoIPDatabase = NewNoop()
	if st.Enabled && st.Provider == "maxmind" && st.DatabasePath != "" {
		targetPath := updater.ResolveDestinationPath(st.DatabasePath)
		if _, err := os.Stat(targetPath); err == nil {
			if mm, err := NewMaxMind(targetPath); err == nil {
				newDB = mm
				st.DatabaseVersion = mm.Version()
			} else {
				s.log.Warn("geoip_open_maxmind_warning", "path", targetPath, "error", err)
			}
		}
	}

	s.mu.Lock()
	oldDB := s.db
	s.db = newDB
	s.settings = st
	s.settingsAt = time.Now()
	if st.CacheMaxEntries > 0 {
		s.memCache = NewMemoryCache(st.CacheMaxEntries, st.CacheTTL)
	}
	s.mu.Unlock()

	if oldDB != nil && oldDB != newDB {
		_ = oldDB.Close()
	}

	return nil
}

func (s *Service) mapRowToSettings(r db.GeoipSetting) Settings {
	var lastUpdate *time.Time
	if r.LastUpdateAt.Valid {
		t := r.LastUpdateAt.Time
		lastUpdate = &t
	}
	ttlHours := int(r.CacheTtlHours)
	if ttlHours <= 0 {
		ttlHours = 24
	}
	dbPath := r.DatabasePath
	if dbPath == "" {
		dbPath = DefaultDatabasePath
	}
	provider := r.Provider
	if provider == "" {
		provider = "maxmind"
	}
	dbType := r.DatabaseType
	if dbType == "" {
		dbType = "geolite2-city"
	}
	updateInterval := int(r.UpdateIntervalHours)
	if updateInterval <= 0 {
		updateInterval = 24
	}
	maxEntries := int(r.CacheMaxEntries)
	if maxEntries <= 0 {
		maxEntries = 100000
	}

	return Settings{
		Enabled:             r.Enabled,
		Provider:            provider,
		DatabaseType:        dbType,
		DatabasePath:        dbPath,
		AutoUpdateEnabled:   r.AutoUpdateEnabled,
		UpdateIntervalHours: updateInterval,
		DownloadURLTemplate: r.DownloadUrlTemplate.String,
		CacheEnabled:        r.CacheEnabled,
		CacheTTL:            time.Duration(ttlHours) * time.Hour,
		CacheMaxEntries:     maxEntries,
		LastUpdateAt:        lastUpdate,
		LastUpdateStatus:    r.LastUpdateStatus,
		LastUpdateError:     r.LastUpdateError.String,
		DatabaseVersion:     r.DatabaseVersion.String,
	}
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}

func isMasked(s string) bool {
	return len(s) > 4 && s[2:6] == "****"
}
