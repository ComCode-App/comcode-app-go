package model

import "time"

// SocialPlatformSummary is the item in lists and dropdowns.
type SocialPlatformSummary struct {
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Category     string `json:"category"`
	URLPattern   string `json:"url_pattern"`
	IconURL      string `json:"icon_url"`
	Placeholder  string `json:"placeholder"`
	Description  string `json:"description"`
	DisplayOrder int    `json:"display_order"`
	IsActive     bool   `json:"is_active"`
}

// SocialPlatformDetail is the full resource payload.
type SocialPlatformDetail struct {
	ID           int64     `json:"-"`
	UUID         string    `json:"uuid"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Category     string    `json:"category"`
	URLPattern   string    `json:"url_pattern"`
	IconURL      string    `json:"icon_url"`
	Placeholder  string    `json:"placeholder"`
	Description  string    `json:"description"`
	DisplayOrder int       `json:"display_order"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UpsertSocialPlatformInput is used to create or update a social platform.
type UpsertSocialPlatformInput struct {
	Name         string `json:"name"`
	Slug         string `json:"slug,omitempty"`
	Category     string `json:"category"`
	URLPattern   string `json:"url_pattern"`
	IconURL      string `json:"icon_url,omitempty"`
	Placeholder  string `json:"placeholder"`
	Description  string `json:"description"`
	DisplayOrder int    `json:"display_order"`
	IsActive     bool   `json:"is_active"`
}
