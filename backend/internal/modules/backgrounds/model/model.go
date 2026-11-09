// Package model holds Background Studio designs saved by accounts and platform presets.
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Background is a saved studio design. IsPreset marks platform gallery items (read-only for apps).
type Background struct {
	UUID     uuid.UUID       `json:"uuid"`
	Name     string          `json:"name"`
	Kind     string          `json:"kind"`
	Document json.RawMessage `json:"document"`
	IsPreset bool            `json:"is_preset"`
	// Premium marks animated designs (shaders with motion, video) that need design.premium (TEC-947).
	Premium bool `json:"premium"`
	// ThumbnailURL is the static stand-in image (TEC-599), shown where the shader cannot run.
	// Nil until generated, and for videos (their poster is the stand-in).
	ThumbnailURL *string   `json:"thumbnail_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UpsertBackgroundInput creates or replaces a design.
type UpsertBackgroundInput struct {
	Name     string          `json:"name"`
	Document json.RawMessage `json:"document"`
}

// MaxPerAccount caps saved designs per account.
const MaxPerAccount = 100

// PublicBackground is what a public page needs about its selected design.
type PublicBackground struct {
	Document json.RawMessage
	// Premium designs render only while the owner's plan has design.premium (TEC-947).
	Premium bool
	// ThumbnailURL is the static stand-in image; "" when there is none.
	ThumbnailURL string
}
