package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	RouteTypeCanonicalProfile = "canonical_profile"
	RouteTypeCustomSmartLink  = "custom_smart_link"
	RouteTypeCardRoute        = "card_route"
	RouteTypeLegacyRoute      = "legacy_route"
	RouteTypeSystemReserved   = "system_reserved"
)

const (
	RouteStatusReserved = "reserved"
	RouteStatusActive   = "active"
	RouteStatusLocked   = "locked"
	RouteStatusRetired  = "retired"
)

const (
	TargetTypeProfile       = "profile"
	TargetTypeDirectURL     = "direct_url"
	TargetTypeVCardDownload = "vcard_download"
	TargetTypeSocialHub     = "social_hub"
	TargetTypeCustomModules = "custom_modules"
)

// PresentationConfig defines how the target entity is presented without modifying the target.
type PresentationConfig struct {
	Theme          string         `json:"theme,omitempty"`
	LayoutMode     string         `json:"layout_mode,omitempty"`
	VisibleModules []string       `json:"visible_modules,omitempty"`
	PrimaryAction  map[string]any `json:"primary_action,omitempty"`
	LeadCapture    map[string]any `json:"lead_capture,omitempty"`
	Security       map[string]any `json:"security,omitempty"`
	CustomCSS      string         `json:"custom_css,omitempty"`
	CustomData     map[string]any `json:"custom_data,omitempty"`
}

// LegacyMetadata captures immutable lineage and provenance for migrated routes.
type LegacyMetadata struct {
	LegacySystem     string     `json:"legacy_system,omitempty"`
	LegacyID         string     `json:"legacy_id,omitempty"`
	OriginalHash     string     `json:"original_hash,omitempty"`
	OriginalQRURL    string     `json:"original_qr_url,omitempty"`
	CardSerialNumber string     `json:"card_serial_number,omitempty"`
	MigrationBatch   string     `json:"migration_batch,omitempty"`
	MigratedAt       *time.Time `json:"migrated_at,omitempty"`
	IsLockedLegacy   bool       `json:"is_locked_legacy,omitempty"`
}

type RouteSummary struct {
	UUID        uuid.UUID  `json:"uuid"`
	Path        string     `json:"path"`
	RouteType   string     `json:"route_type"`
	Status      string     `json:"status"`
	IsImmutable bool       `json:"is_immutable"`
	TargetType  string     `json:"target_type"`
	ProfileUUID *uuid.UUID `json:"profile_uuid,omitempty"`
	CardUUID    *uuid.UUID `json:"card_uuid,omitempty"`
	DirectURL   string     `json:"direct_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type RouteDetail struct {
	ID                 int64          `json:"-"`
	UUID               uuid.UUID      `json:"uuid"`
	Path               string         `json:"path"`
	RouteType          string         `json:"route_type"`
	Status             string         `json:"status"`
	IsImmutable        bool           `json:"is_immutable"`
	TargetType         string         `json:"target_type"`
	TargetID           *uuid.UUID     `json:"target_id,omitempty"`
	ProfileID          *int64         `json:"-"`
	ProfileUUID        *uuid.UUID     `json:"profile_uuid,omitempty"`
	ProfileAvatarURL   string         `json:"profile_avatar_url,omitempty"`
	ProfileLocale      string         `json:"profile_locale,omitempty"`
	ProfileUsername    string         `json:"profile_username,omitempty"`
	CardID             *int64         `json:"-"`
	CardUUID           *uuid.UUID     `json:"card_uuid,omitempty"`
	CardName           string         `json:"card_name,omitempty"`
	CardIsLocked       bool           `json:"card_is_locked,omitempty"`
	CardIsActive       bool           `json:"card_is_active,omitempty"`
	CardLeadCapture    bool           `json:"card_lead_capture_enabled,omitempty"`
	CardHasPasscode    bool           `json:"card_has_passcode,omitempty"`
	DirectURL          string         `json:"direct_url,omitempty"`
	AccountID          *int64         `json:"-"`
	OwnerUserID        *int64         `json:"-"`
	PresentationConfig map[string]any `json:"presentation_config"`
	LegacyMetadata     map[string]any `json:"legacy_metadata"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type CreateSmartLinkInput struct {
	Slug               string         `json:"slug"`
	TargetType         string         `json:"target_type"`
	ProfileUUID        *uuid.UUID     `json:"profile_uuid,omitempty"`
	DirectURL          string         `json:"direct_url,omitempty"`
	PresentationConfig map[string]any `json:"presentation_config,omitempty"`
}

type CreateReservedRouteInput struct {
	Path               string         `json:"path"`
	RouteType          string         `json:"route_type"`
	Status             string         `json:"status"`
	IsImmutable        bool           `json:"is_immutable"`
	TargetType         string         `json:"target_type"`
	ProfileID          *int64         `json:"profile_id,omitempty"`
	CardID             *int64         `json:"card_id,omitempty"`
	DirectURL          string         `json:"direct_url,omitempty"`
	AccountID          *int64         `json:"account_id,omitempty"`
	OwnerUserID        *int64         `json:"owner_user_id,omitempty"`
	PresentationConfig map[string]any `json:"presentation_config,omitempty"`
	LegacyMetadata     map[string]any `json:"legacy_metadata,omitempty"`
}

type UpdateRouteInput struct {
	Path               *string        `json:"path,omitempty"`
	TargetType         *string        `json:"target_type,omitempty"`
	ProfileUUID        *uuid.UUID     `json:"profile_uuid,omitempty"`
	DirectURL          *string        `json:"direct_url,omitempty"`
	Status             *string        `json:"status,omitempty"`
	PresentationConfig map[string]any `json:"presentation_config,omitempty"`
}

type RouteResolution struct {
	RouteUUID          uuid.UUID      `json:"route_uuid"`
	Path               string         `json:"path"`
	RouteType          string         `json:"route_type"`
	Status             string         `json:"status"`
	TargetType         string         `json:"target_type"`
	TargetURL          string         `json:"target_url,omitempty"`
	DirectURL          string         `json:"direct_url,omitempty"`
	ProfileUUID        *uuid.UUID     `json:"profile_uuid,omitempty"`
	ProfileAvatarURL   string         `json:"profile_avatar_url,omitempty"`
	ProfileUsername    string         `json:"profile_username,omitempty"`
	CardUUID           *uuid.UUID     `json:"card_uuid,omitempty"`
	CardName           string         `json:"card_name,omitempty"`
	IsLocked           bool           `json:"is_locked"`
	IsActive           bool           `json:"is_active"`
	RequiresPasscode   bool           `json:"requires_passcode"`
	// LeadCaptureEnabled: the visitor lands on the profile page with the lead form (TEC-552).
	LeadCaptureEnabled bool           `json:"lead_capture_enabled"`
	PresentationConfig map[string]any `json:"presentation_config"`
	LegacyMetadata     map[string]any `json:"legacy_metadata,omitempty"`
}

// Primary actions a smart link can put first on the profile page (presentation_config.primary_action.type).
var PrimaryActions = []string{"save_contact", "call", "email", "whatsapp", "website"}

// PasscodeHashKey is where presentation_config.security keeps a smart link's hashed passcode.
const PasscodeHashKey = "passcode_hash"

// PasscodeHash returns the stored passcode hash of a presentation_config; "" = no passcode.
func PasscodeHash(cfg map[string]any) string {
	sec, ok := cfg["security"].(map[string]any)
	if !ok {
		return ""
	}
	h, _ := sec[PasscodeHashKey].(string)
	return h
}

// LinkPresentation is how a smart link presents the profile it targets (?via=s/{slug}, TEC-556).
type LinkPresentation struct {
	ProfileUUID    uuid.UUID
	Theme          string
	LayoutMode     string
	VisibleModules []string // nil = the profile's own selection
	PrimaryAction  string
	// SocialHub links show the profile's social links only.
	SocialHub bool
}

// LeadCaptureOn reports whether presentation_config.lead_capture.enabled is true.
func LeadCaptureOn(cfg map[string]any) bool {
	lc, ok := cfg["lead_capture"].(map[string]any)
	if !ok {
		return false
	}
	on, _ := lc["enabled"].(bool)
	return on
}

// Reasons recorded in route_history.
const (
	PathChangeUsername        = "username_change"
	PathChangeSmartLinkRename = "smart_link_rename"
)

// PathChange is one route_history row (TEC-558).
type PathChange struct {
	RouteID         int64
	RouteType       string
	AccountID       *int64
	OldPath         string
	NewPath         string
	ChangedByUserID *int64
	Reason          string
}
