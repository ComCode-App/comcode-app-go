package model

import (
	"time"

	"github.com/google/uuid"
)

// ProfileSummary is a lightweight representation for profile lists.
type ProfileSummary struct {
	UUID           uuid.UUID `json:"uuid"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Title          string    `json:"title"`
	CompanyName    string    `json:"company_name"`
	AvatarURL      string    `json:"avatar_url"`
	PrimaryLocale  string    `json:"primary_locale"`
	IsPrimary      bool      `json:"is_primary"`
	CompletionRate int       `json:"completion_rate"`
	// Username is the public path (comcode.../{username}); empty until one is assigned.
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProfileTranslationDTO represents localized content for a profile.
type ProfileTranslationDTO struct {
	Locale      string `json:"locale"` // tr, en
	Prefix      string `json:"prefix"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	MiddleName  string `json:"middle_name"`
	Suffix      string `json:"suffix"`
	Nickname    string `json:"nickname"`
	CompanyName string `json:"company_name"`
	Department  string `json:"department"`
	Title       string `json:"title"`
	Role        string `json:"role"`
	Bio         string `json:"bio"`
	Note        string `json:"note"`
	IsDefault   bool   `json:"is_default"`
}

// AddressDTO represents a postal address on a profile.
type AddressDTO struct {
	UUID         uuid.UUID `json:"uuid"`
	Type         string    `json:"type"` // work, home, other
	Label        string    `json:"label"`
	Street       string    `json:"street"`
	Extended     string    `json:"extended"`
	City         string    `json:"city"`
	Region       string    `json:"region"`
	PostalCode   string    `json:"postal_code"`
	Country      string    `json:"country"`
	IsPrimary    bool      `json:"is_primary"`
	DisplayOrder int       `json:"display_order"`
}

// ContactDetailDTO represents an email, phone, or url entry.
type ContactDetailDTO struct {
	UUID         uuid.UUID `json:"uuid"`
	Category     string    `json:"category"` // phone, email, url, whatsapp, other
	Type         string    `json:"type"`     // work, home, mobile, fax, main, other, preferred
	Label        string    `json:"label"`
	Value        string    `json:"value"`
	IsPrimary    bool      `json:"is_primary"`
	DisplayOrder int       `json:"display_order"`
}

// SocialLinkDTO represents a social media profile link.
type SocialLinkDTO struct {
	UUID         uuid.UUID `json:"uuid"`
	PlatformID   *int64    `json:"platform_id,omitempty"`
	Platform     string    `json:"platform"` // platform slug or custom
	Label        string    `json:"label"`
	URL          string    `json:"url"`
	IconURL      string    `json:"icon_url"`
	DisplayOrder int       `json:"display_order"`
	IsActive     bool      `json:"is_active"`
}

// ProfileDetail is the full profile aggregate including translations and nested lists.
type ProfileDetail struct {
	UUID           uuid.UUID `json:"uuid"`
	UserUUID       uuid.UUID `json:"user_uuid"`
	PrimaryLocale  string    `json:"primary_locale"`
	Birthday       *string   `json:"birthday,omitempty"` // YYYY-MM-DD
	Gender         string    `json:"gender"`
	AvatarURL      string    `json:"avatar_url"`
	CoverURL       string    `json:"cover_url"`
	IsPrimary      bool      `json:"is_primary"`
	CompletionRate int       `json:"completion_rate"`
	// CompletionSuggestions lists what is missing for 100%, highest gain first (TEC-549).
	CompletionSuggestions []CompletionSuggestion `json:"completion_suggestions"`
	// VerificationStatus is none | pending | verified | rejected (TEC-549).
	VerificationStatus string                  `json:"verification_status"`
	Translations       []ProfileTranslationDTO `json:"translations"`
	Addresses          []AddressDTO            `json:"addresses"`
	ContactDetails     []ContactDetailDTO      `json:"contact_details"`
	SocialLinks        []SocialLinkDTO         `json:"social_links"`
	// Username is the public path; filled by Get and Create (empty when not assigned yet).
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CompletionSuggestion is one actionable step that raises the completion score. The rules live
// in the profiles/completion package.
type CompletionSuggestion struct {
	// Key identifies the rule (name, avatar, title, company, phone, email, address, social).
	Key string `json:"key"`
	// Points is what completing the step adds to the score.
	Points int `json:"points"`
	// Section is the profile editor tab to open (identity, media, contacts, social, addresses).
	Section string `json:"section"`
	// I18nKey is the frontend translation key of the suggestion copy.
	I18nKey string `json:"i18n_key"`
}

// UpsertTranslationInput is the translation payload.
type UpsertTranslationInput struct {
	Locale      string `json:"locale"`
	Prefix      string `json:"prefix"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	MiddleName  string `json:"middle_name"`
	Suffix      string `json:"suffix"`
	Nickname    string `json:"nickname"`
	CompanyName string `json:"company_name"`
	Department  string `json:"department"`
	Title       string `json:"title"`
	Role        string `json:"role"`
	Bio         string `json:"bio"`
	Note        string `json:"note"`
	IsDefault   bool   `json:"is_default"`
}

// CreateAddressInput represents address payload on create/update.
type CreateAddressInput struct {
	Type         string `json:"type"`
	Label        string `json:"label"`
	Street       string `json:"street"`
	Extended     string `json:"extended"`
	City         string `json:"city"`
	Region       string `json:"region"`
	PostalCode   string `json:"postal_code"`
	Country      string `json:"country"`
	IsPrimary    bool   `json:"is_primary"`
	DisplayOrder int    `json:"display_order"`
}

// CreateContactDetailInput represents contact detail payload on create/update.
type CreateContactDetailInput struct {
	Category     string `json:"category"`
	Type         string `json:"type"`
	Label        string `json:"label"`
	Value        string `json:"value"`
	IsPrimary    bool   `json:"is_primary"`
	DisplayOrder int    `json:"display_order"`
}

// CreateSocialLinkInput represents social link payload on create/update.
type CreateSocialLinkInput struct {
	PlatformID   *int64 `json:"platform_id,omitempty"`
	Platform     string `json:"platform"`
	Label        string `json:"label"`
	URL          string `json:"url"`
	IconURL      string `json:"icon_url"`
	DisplayOrder int    `json:"display_order"`
	IsActive     bool   `json:"is_active"`
}

// UpsertProfileInput is the payload for creating or updating a profile.
type UpsertProfileInput struct {
	PrimaryLocale  string                     `json:"primary_locale"`
	Birthday       *string                    `json:"birthday"` // YYYY-MM-DD
	Gender         string                     `json:"gender"`
	AvatarURL      string                     `json:"avatar_url"`
	CoverURL       string                     `json:"cover_url"`
	IsPrimary      bool                       `json:"is_primary"`
	Translations   []UpsertTranslationInput   `json:"translations"`
	Addresses      []CreateAddressInput       `json:"addresses"`
	ContactDetails []CreateContactDetailInput `json:"contact_details"`
	SocialLinks    []CreateSocialLinkInput    `json:"social_links"`
}

// ParsedVCardProfile represents decoded vCard data ready for form autofill/autocomplete.
type ParsedVCardProfile struct {
	DisplayName    string                     `json:"display_name"`
	Prefix         string                     `json:"prefix"`
	FirstName      string                     `json:"first_name"`
	LastName       string                     `json:"last_name"`
	MiddleName     string                     `json:"middle_name"`
	Suffix         string                     `json:"suffix"`
	Nickname       string                     `json:"nickname"`
	CompanyName    string                     `json:"company_name"`
	Department     string                     `json:"department"`
	Title          string                     `json:"title"`
	Role           string                     `json:"role"`
	Bio            string                     `json:"bio"`
	Note           string                     `json:"note"`
	Birthday       *string                    `json:"birthday,omitempty"`
	Gender         string                     `json:"gender"`
	AvatarDataURL  string                     `json:"avatar_data_url,omitempty"`
	AvatarURL      string                     `json:"avatar_url,omitempty"`
	AvatarTempKey  string                     `json:"avatar_temp_key,omitempty"`
	ContactDetails []CreateContactDetailInput `json:"contact_details"`
	Addresses      []CreateAddressInput       `json:"addresses"`
	SocialLinks    []CreateSocialLinkInput    `json:"social_links"`
}
