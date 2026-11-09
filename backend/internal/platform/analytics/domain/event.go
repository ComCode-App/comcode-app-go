package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidEventType     = errors.New("invalid event type")
	ErrInvalidSubject       = errors.New("invalid subject: type and id are required")
	ErrEventDisabled        = errors.New("event type is disabled")
	ErrEventInternalOnly    = errors.New("event type is not allowed for public ingestion")
	ErrMissingProperty      = errors.New("missing required property")
	ErrPayloadTooLarge      = errors.New("event payload exceeds maximum allowed size")
	ErrPayloadTooDeep       = errors.New("event properties exceed maximum allowed depth")
	ErrFutureTimestamp      = errors.New("occurred_at timestamp cannot be in the future")
	ErrEventNotFound        = errors.New("event not found")
	ErrDefinitionNotFound   = errors.New("event definition not found")
	ErrDefinitionExists     = errors.New("event definition already exists")
)

// EntityReference uniquely identifies a subject or actor in a polymorphic manner.
type EntityReference struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// EventContext encapsulates client, environment, geo, and device snapshot.
type EventContext struct {
	IPHash         string   `json:"ip_hash,omitempty"`
	CountryCode    string   `json:"country_code,omitempty"`
	CountryName    string   `json:"country_name,omitempty"`
	City           string   `json:"city,omitempty"`
	Timezone       string   `json:"timezone,omitempty"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	DeviceType     string   `json:"device_type,omitempty"`
	DeviceBrand    string   `json:"device_brand,omitempty"`
	DeviceModel    string   `json:"device_model,omitempty"`
	Browser        string   `json:"browser,omitempty"`
	BrowserVersion string   `json:"browser_version,omitempty"`
	OS             string   `json:"os,omitempty"`
	OSVersion      string   `json:"os_version,omitempty"`
	Referer        string   `json:"referer,omitempty"`
	UserAgent      string   `json:"user_agent,omitempty"`
}

// EventMetadata stores technical/pipeline information separate from business properties.
type EventMetadata struct {
	Source        string `json:"source,omitempty"`
	SchemaVersion int    `json:"schema_version,omitempty"`
	DeliveryMode  string `json:"delivery_mode,omitempty"`
	BatchID       string `json:"batch_id,omitempty"`
	IngestedAt    string `json:"ingested_at,omitempty"`
}

// Event is the canonical polymorphic event envelope.
type Event struct {
	ID            int64                  `json:"id,omitempty"`
	UUID          string                 `json:"uuid"`
	Type          string                 `json:"type"`
	SchemaVersion int                    `json:"schema_version"`
	Subject       EntityReference        `json:"subject"`
	Actor         *EntityReference       `json:"actor,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	RequestID     string                 `json:"request_id,omitempty"`
	OccurredAt    time.Time              `json:"occurred_at"`
	CreatedAt     time.Time              `json:"created_at"`
	Properties    map[string]any         `json:"properties"`
	Context       EventContext           `json:"context"`
	Metadata      EventMetadata          `json:"metadata"`
}

// EventDefinition configures an event type's validation and processing rules.
type EventDefinition struct {
	ID                 int64     `json:"id"`
	Key                string    `json:"key"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Category           string    `json:"category"`
	SchemaVersion      int       `json:"schema_version"`
	Enabled            bool      `json:"enabled"`
	IsPublic           bool      `json:"is_public"`
	AggregateEnabled   bool      `json:"aggregate_enabled"`
	RetentionDays      int       `json:"retention_days"`
	RequiredProperties []string  `json:"required_properties"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
