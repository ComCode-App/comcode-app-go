package handler

import (
	"encoding/json"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/geoip"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Handler exposes administrative controls for the GeoIP engine.
type Handler struct {
	svc *geoip.Service
}

// New creates a GeoIP handler.
func New(svc *geoip.Service) *Handler {
	return &Handler{svc: svc}
}

// Status returns current database loading state and version.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Status(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to get geoip status")
		return
	}
	response.JSON(w, r, http.StatusOK, st)
}

// Update triggers an immediate download, validation, and hot reload.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.UpdateNow(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to update geoip database")
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{
		"success":       res.Success,
		"version":       res.Version,
		"downloaded_at": res.DownloadedAt,
		"bytes":         res.Bytes,
	})
}

// ClearCache flushes in-memory and Redis caches.
func (h *Handler) ClearCache(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.ClearCache(r.Context()); err != nil {
		response.InternalErr(w, r, err, "failed to clear geoip cache")
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "cache_cleared"})
}

// GetConfig returns the current configuration (credentials masked).
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.GetSettings(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to get geoip config")
		return
	}
	response.JSON(w, r, http.StatusOK, st)
}

type patchConfigPayload struct {
	Enabled             *bool   `json:"enabled"`
	Provider            *string `json:"provider"`
	DatabaseType        *string `json:"database_type"`
	DatabasePath        *string `json:"database_path"`
	AutoUpdateEnabled   *bool   `json:"auto_update_enabled"`
	UpdateIntervalHours *int    `json:"update_interval_hours"`
	AccountID           *string `json:"account_id"`
	LicenseKey          *string `json:"license_key"`
	DownloadURLTemplate *string `json:"download_url_template"`
	CacheEnabled        *bool   `json:"cache_enabled"`
	CacheTTL            *int    `json:"cache_ttl"`
	CacheMaxEntries     *int    `json:"cache_max_entries"`
}

// PatchConfig updates GeoIP operational configuration.
func (h *Handler) PatchConfig(w http.ResponseWriter, r *http.Request) {
	var in patchConfigPayload
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}

	current, err := h.svc.GetSettings(r.Context())
	if err != nil {
		current = geoip.DefaultSettings()
	}

	if in.Enabled != nil {
		current.Enabled = *in.Enabled
	}
	if in.Provider != nil && *in.Provider != "" {
		current.Provider = *in.Provider
	}
	if in.DatabaseType != nil && *in.DatabaseType != "" {
		current.DatabaseType = *in.DatabaseType
	}
	if in.DatabasePath != nil && *in.DatabasePath != "" {
		current.DatabasePath = *in.DatabasePath
	}
	if in.AutoUpdateEnabled != nil {
		current.AutoUpdateEnabled = *in.AutoUpdateEnabled
	}
	if in.UpdateIntervalHours != nil && *in.UpdateIntervalHours > 0 {
		current.UpdateIntervalHours = *in.UpdateIntervalHours
	}
	if in.AccountID != nil {
		current.AccountID = *in.AccountID
	}
	if in.LicenseKey != nil {
		current.LicenseKey = *in.LicenseKey
	}
	if in.DownloadURLTemplate != nil {
		current.DownloadURLTemplate = *in.DownloadURLTemplate
	}
	if in.CacheEnabled != nil {
		current.CacheEnabled = *in.CacheEnabled
	}
	if in.CacheTTL != nil && *in.CacheTTL > 0 {
		current.CacheTTL = time.Duration(*in.CacheTTL) * time.Hour
	}
	if in.CacheMaxEntries != nil && *in.CacheMaxEntries > 0 {
		current.CacheMaxEntries = *in.CacheMaxEntries
	}

	updated, err := h.svc.UpdateSettings(r.Context(), current)
	if err != nil {
		response.InternalErr(w, r, err, "failed to update geoip config")
		return
	}

	response.JSON(w, r, http.StatusOK, updated)
}

// Lookup resolves a provided IP or the caller's IP to test the GeoIP provider.
func (h *Handler) Lookup(w http.ResponseWriter, r *http.Request) {
	queryIP := strings.TrimSpace(r.URL.Query().Get("ip"))
	var ip net.IP
	if queryIP != "" {
		ip = net.ParseIP(queryIP)
		if ip == nil {
			response.BadRequest(w, r, response.CodeValidationError, "invalid IP address format")
			return
		}
	} else {
		ip = extractIP(r)
		if ip != nil {
			queryIP = ip.String()
		}
	}

	if ip == nil {
		response.BadRequest(w, r, response.CodeValidationError, "could not determine IP address")
		return
	}

	loc, err := h.svc.Resolve(r.Context(), ip)
	if err != nil {
		response.InternalErr(w, r, err, "failed to resolve IP address")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{
		"ip":           queryIP,
		"country_code": loc.CountryCode,
		"country_name": loc.CountryName,
		"city":         loc.City,
		"timezone":     loc.Timezone,
		"latitude":     loc.Latitude,
		"longitude":    loc.Longitude,
	})
}

func extractIP(r *http.Request) net.IP {
	return clientip.IP(r)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return err
	}
	return nil
}
