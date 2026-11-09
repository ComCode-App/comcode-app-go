package model

import (
	"time"

	"github.com/google/uuid"
)

type CardSummary struct {
	UUID               uuid.UUID      `json:"uuid"`
	Name               string         `json:"name"`
	CardUID            string         `json:"card_uid,omitempty"`
	SmartCode          string         `json:"smart_code"`
	TargetType         string         `json:"target_type"` // profile, direct_url, vcard_download, social_hub
	ProfileUUID        *uuid.UUID     `json:"profile_uuid,omitempty"`
	ProfileAvatarURL   string         `json:"profile_avatar_url,omitempty"`
	ProfileLocale      string         `json:"profile_locale,omitempty"`
	DirectURL          string         `json:"direct_url,omitempty"`
	CustomData         map[string]any `json:"custom_data"`
	CardType           string         `json:"card_type"` // physical_nfc, metal_card, wood_card, pvc_card, digital_only
	Material           string         `json:"material"`
	ColorHex           string         `json:"color_hex"`
	IsActive           bool           `json:"is_active"`
	IsLocked           bool           `json:"is_locked"`
	LeadCaptureEnabled bool           `json:"lead_capture_enabled"`
	HasPasscode        bool           `json:"has_passcode"`
	// RouteUUID is the card's c/{code} route; its scans are analytics events of subject "route".
	RouteUUID *uuid.UUID `json:"route_uuid,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CardDetail struct {
	ID                 int64          `json:"-"`
	UUID               uuid.UUID      `json:"uuid"`
	UserID             int64          `json:"-"`
	Name               string         `json:"name"`
	CardUID            string         `json:"card_uid,omitempty"`
	ActivationCode     string         `json:"activation_code,omitempty"`
	SmartCode          string         `json:"smart_code"`
	TargetType         string         `json:"target_type"`
	ProfileID          *int64         `json:"-"`
	ProfileUUID        *uuid.UUID     `json:"profile_uuid,omitempty"`
	ProfileAvatarURL   string         `json:"profile_avatar_url,omitempty"`
	ProfileLocale      string         `json:"profile_locale,omitempty"`
	DirectURL          string         `json:"direct_url,omitempty"`
	CustomData         map[string]any `json:"custom_data"`
	CardType           string         `json:"card_type"`
	Material           string         `json:"material"`
	ColorHex           string         `json:"color_hex"`
	IsActive           bool           `json:"is_active"`
	IsLocked           bool           `json:"is_locked"`
	LeadCaptureEnabled bool           `json:"lead_capture_enabled"`
	HasPasscode        bool           `json:"has_passcode"`
	Passcode           *string        `json:"-"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type UpsertCardInput struct {
	Name               string         `json:"name"`
	CardUID            *string        `json:"card_uid,omitempty"`
	TargetType         string         `json:"target_type"`
	ProfileUUID        *uuid.UUID     `json:"profile_uuid,omitempty"`
	DirectURL          string         `json:"direct_url,omitempty"`
	CustomData         map[string]any `json:"custom_data,omitempty"`
	CardType           string         `json:"card_type,omitempty"`
	Material           string         `json:"material,omitempty"`
	ColorHex           string         `json:"color_hex,omitempty"`
	IsActive           *bool          `json:"is_active,omitempty"`
	IsLocked           *bool          `json:"is_locked,omitempty"`
	LeadCaptureEnabled *bool          `json:"lead_capture_enabled,omitempty"`
	Passcode           *string        `json:"passcode,omitempty"`
}

type ActivateCardInput struct {
	ActivationCode string     `json:"activation_code"`
	Name           string     `json:"name,omitempty"`
	ProfileUUID    *uuid.UUID `json:"profile_uuid,omitempty"`
}

type SmartLinkResolution struct {
	SmartCode          string         `json:"smart_code"`
	CardName           string         `json:"card_name"`
	TargetType         string         `json:"target_type"`
	TargetURL          string         `json:"target_url,omitempty"`
	ProfileUUID        *uuid.UUID     `json:"profile_uuid,omitempty"`
	ProfileAvatarURL   string         `json:"profile_avatar_url,omitempty"`
	DirectURL          string         `json:"direct_url,omitempty"`
	CustomData         map[string]any `json:"custom_data,omitempty"`
	IsLocked           bool           `json:"is_locked"`
	IsActive           bool           `json:"is_active"`
	LeadCaptureEnabled bool           `json:"lead_capture_enabled"`
	RequiresPasscode   bool           `json:"requires_passcode"`
}
