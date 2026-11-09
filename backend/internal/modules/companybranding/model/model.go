// Package model holds the company media library and company branding contracts (TEC-946).
package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrForbidden = errors.New("company branding operation forbidden")
	ErrNotFound  = errors.New("company media not found")
	ErrInvalid   = errors.New("invalid company branding input")
	// ErrInUse means a file is still the branding logo/cover or a profile's avatar/cover.
	ErrInUse = errors.New("company media is in use")
	// ErrLimitReached means the library holds MaxMediaPerAccount files.
	ErrLimitReached = errors.New("company media limit reached")
)

// Library file kinds. logo, cover and image are images; document is a PDF.
const (
	KindLogo     = "logo"
	KindCover    = "cover"
	KindImage    = "image"
	KindDocument = "document"
)

// Kinds lists every library kind.
var Kinds = []string{KindLogo, KindCover, KindImage, KindDocument}

// Upload limits.
const (
	MaxImageBytes      = 5 << 20
	MaxDocumentBytes   = 10 << 20
	MaxMediaPerAccount = 500
)

// Usage types: where a library file is used.
const (
	UsageBrandingLogo  = "branding_logo"
	UsageBrandingCover = "branding_cover"
	UsageProfileAvatar = "profile_avatar"
	UsageProfileCover  = "profile_cover"
)

// Usage is one place a library file is used. Profile fields are set for profile usages.
type Usage struct {
	Type        string     `json:"type"`
	ProfileUUID *uuid.UUID `json:"profile_uuid,omitempty"`
	ProfileName string     `json:"profile_name,omitempty"`
}

// Media is one library file. URL is API-owned; the browser never builds it from a key.
type Media struct {
	UUID           uuid.UUID  `json:"uuid"`
	Kind           string     `json:"kind"`
	Name           string     `json:"name"`
	ContentType    string     `json:"content_type"`
	SizeBytes      int64      `json:"size_bytes"`
	URL            string     `json:"url"`
	UploadedByUUID *uuid.UUID `json:"uploaded_by_uuid"`
	CreatedAt      time.Time  `json:"created_at"`
	Usage          []Usage    `json:"usage"`
}

// Filter narrows the library list.
type Filter struct {
	Kind          string
	Limit, Offset int32
}

// UploadInput is one file upload.
type UploadInput struct {
	Kind     string
	Name     string
	Filename string
	Size     int64
}

// MediaRef is a library file picked by the branding.
type MediaRef struct {
	UUID uuid.UUID `json:"uuid"`
	URL  string    `json:"url"`
}

// Branding is the company look (account_branding). Empty strings and nil refs are unset.
type Branding struct {
	Theme        string     `json:"theme"`
	LayoutMode   string     `json:"layout_mode"`
	PrimaryColor string     `json:"primary_color"`
	Logo         *MediaRef  `json:"logo"`
	Cover        *MediaRef  `json:"cover"`
	Locked       bool       `json:"locked"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

// UpdateBrandingInput replaces the branding. Logo and cover are library file uuids (null = none).
type UpdateBrandingInput struct {
	Theme          string     `json:"theme"`
	LayoutMode     string     `json:"layout_mode"`
	PrimaryColor   string     `json:"primary_color"`
	LogoMediaUUID  *uuid.UUID `json:"logo_media_uuid"`
	CoverMediaUUID *uuid.UUID `json:"cover_media_uuid"`
	Locked         bool       `json:"locked"`
}

// InUseError carries the usages that block a delete.
type InUseError struct {
	Usage []Usage
}

func (e *InUseError) Error() string { return ErrInUse.Error() }
func (e *InUseError) Unwrap() error { return ErrInUse }
