// Package model holds the types of company employee profiles (TEC-573): member-profile links,
// bulk action parameters and the employee import.
package model

import (
	"errors"

	"github.com/google/uuid"
)

// Engine resource slugs.
const (
	// ResourceCompanyProfiles is the bulk-engine resource of employee profiles.
	ResourceCompanyProfiles = "app.company_profiles"
	// ResourceCompanyEmployees is the io-engine (import) resource of employees.
	ResourceCompanyEmployees = "app.company_employees"
)

// Bulk action ids of ResourceCompanyProfiles.
const (
	ActionSetContact = "set_contact"
	ActionSetLogo    = "set_logo"
	ActionSetTheme   = "set_theme"
	ActionAddLinks   = "add_links"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("not found")
	ErrInvalid   = errors.New("invalid")
	ErrConflict  = errors.New("conflict")
)

// MemberProfile is one member of the active company with its assignment and linked profile.
type MemberProfile struct {
	UserUUID   uuid.UUID          `json:"user_uuid"`
	Email      string             `json:"email"`
	Name       string             `json:"name"`
	Surname    string             `json:"surname"`
	Role       string             `json:"role"`
	OfficeUUID *uuid.UUID         `json:"office_uuid"`
	GroupUUID  *uuid.UUID         `json:"group_uuid"`
	Profile    *LinkedProfileInfo `json:"profile"`
}

// LinkedProfileInfo summarizes the profile linked to a member.
type LinkedProfileInfo struct {
	UUID      uuid.UUID `json:"uuid"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Title     string    `json:"title"`
	AvatarURL string    `json:"avatar_url"`
}

// MemberProfileFilter narrows the member list (and the bulk "query" target scope).
type MemberProfileFilter struct {
	Q          string
	OfficeUUID *uuid.UUID
	GroupUUID  *uuid.UUID
	HasProfile *bool
	Limit      int32
	Offset     int32
}

// AddressParams is the company address written as the profile's work address.
type AddressParams struct {
	Label      string `json:"label"`
	Street     string `json:"street"`
	Extended   string `json:"extended"`
	City       string `json:"city"`
	Region     string `json:"region"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

// ContactParams of ActionSetContact. A nil field is left unchanged; an empty string clears the
// work entry of that kind. UseOfficeAddress takes address (and phone, unless Phone is set) from
// the office the member is assigned to.
type ContactParams struct {
	CompanyName      *string        `json:"company_name"`
	Address          *AddressParams `json:"address"`
	UseOfficeAddress bool           `json:"use_office_address"`
	Phone            *string        `json:"phone"`
	Email            *string        `json:"email"`
	Website          *string        `json:"website"`
}

// Media fields a logo can be read from and written to.
const (
	MediaAvatar = "avatar"
	MediaCover  = "cover"
)

// Branding slots a logo action can reference (account_branding, TEC-946).
const (
	BrandingLogo  = "logo"
	BrandingCover = "cover"
)

// LogoParams of ActionSetLogo: point each target profile's avatar or cover at the company
// branding logo or cover (a file of the company media library, TEC-946). Nothing is copied:
// the profiles reference the library file, so the library shows them as its usages.
type LogoParams struct {
	Source      string `json:"source"`
	TargetField string `json:"target_field"`
}

// ThemeParams of ActionSetTheme. nil keeps; "" clears (inherit / the theme's layout).
type ThemeParams struct {
	Theme      *string `json:"theme"`
	LayoutMode *string `json:"layout_mode"`
}

// LinkParams is one shared link. Platform is a social platform slug or "custom".
type LinkParams struct {
	Platform string `json:"platform"`
	Label    string `json:"label"`
	URL      string `json:"url"`
}

// LinksParams of ActionAddLinks. Links already on the profile (same URL) are skipped;
// ReplaceSamePlatform replaces the URL of an existing link of the same platform instead of
// adding a second one.
type LinksParams struct {
	Links               []LinkParams `json:"links"`
	ReplaceSamePlatform bool         `json:"replace_same_platform"`
}
