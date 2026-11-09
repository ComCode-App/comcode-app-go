package geoip

import (
	"context"
	"net"
	"time"
)

// GeoLocation holds geographical metadata resolved from an IP address.
type GeoLocation struct {
	CountryCode string   `json:"country_code"`
	CountryName string   `json:"country_name"`
	City        string   `json:"city"`
	Timezone    string   `json:"timezone"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
}

// GeoIPResolver resolves client IP addresses to geographic locations.
type GeoIPResolver interface {
	Resolve(ctx context.Context, ip net.IP) (*GeoLocation, error)
}

// GeoIPDatabase defines the interface for low-level MMDB/database providers.
type GeoIPDatabase interface {
	Lookup(ip net.IP) (*GeoLocation, error)
	Version() string
	Close() error
}

// Settings defines the operational parameters for the GeoIP engine.
type Settings struct {
	Enabled             bool          `json:"enabled"`
	Provider            string        `json:"provider"`
	DatabaseType        string        `json:"database_type"`
	DatabasePath        string        `json:"database_path"`
	AutoUpdateEnabled   bool          `json:"auto_update_enabled"`
	UpdateIntervalHours int           `json:"update_interval_hours"`
	AccountID           string        `json:"account_id,omitempty"`
	LicenseKey          string        `json:"license_key,omitempty"`
	DownloadURLTemplate string        `json:"download_url_template,omitempty"`
	CacheEnabled        bool          `json:"cache_enabled"`
	CacheTTL            time.Duration `json:"cache_ttl"`
	CacheMaxEntries     int           `json:"cache_max_entries"`
	LastUpdateAt        *time.Time    `json:"last_update_at,omitempty"`
	LastUpdateStatus    string        `json:"last_update_status"`
	LastUpdateError     string        `json:"last_update_error,omitempty"`
	DatabaseVersion     string        `json:"database_version,omitempty"`
}

const DefaultDatabasePath = "data/geoip/GeoLite2-City.mmdb"

// DefaultSettings returns safe operational default settings.
func DefaultSettings() Settings {
	return Settings{
		Enabled:             true,
		Provider:            "maxmind",
		DatabaseType:        "geolite2-city",
		DatabasePath:        DefaultDatabasePath,
		AutoUpdateEnabled:   true,
		UpdateIntervalHours: 24,
		CacheEnabled:        true,
		CacheTTL:            24 * time.Hour,
		CacheMaxEntries:     100000,
		LastUpdateStatus:    "pending",
	}
}

// Status returns current runtime status of the GeoIP provider.
type Status struct {
	Enabled          bool       `json:"enabled"`
	Provider         string     `json:"provider"`
	DatabasePath     string     `json:"database_path"`
	DatabaseLoaded   bool       `json:"database_loaded"`
	DatabaseVersion  string     `json:"database_version"`
	LastUpdateAt     *time.Time `json:"last_update_at,omitempty"`
	LastUpdateStatus string     `json:"last_update_status"`
	LastUpdateError  string     `json:"last_update_error,omitempty"`
	CacheSize        int        `json:"cache_size"`
}
