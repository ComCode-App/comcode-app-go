package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Public page modules an owner can show or hide (presentation_config.visible_modules).
const (
	ModuleBio          = "bio"
	ModuleContacts     = "contacts"
	ModuleSocialLinks  = "social_links"
	ModuleAddresses    = "addresses"
	ModuleBankAccounts = "bank_accounts"
)

// PublicModules lists every module in display order; it is also the default selection.
var PublicModules = []string{ModuleBio, ModuleContacts, ModuleSocialLinks, ModuleAddresses, ModuleBankAccounts}

// PublicThemes is the allow-list for presentation_config.theme (TEC-601). It mirrors the
// frontend catalog in features/public-profile/lib/themes.ts; the tokens live there.
var PublicThemes = []string{"classic_light", "dark_minimal", "compact_card", "bold_gradient", "elegant_serif", "glass"}

// PremiumThemes are the themes that need the design.premium entitlement (TEC-947), each mapped
// to the basic theme a public page shows instead while the plan lacks it. Mirrors `premium` in
// themes.json (themes.test.mjs checks both).
var PremiumThemes = map[string]string{"bold_gradient": "classic_light", "elegant_serif": "classic_light", "glass": "dark_minimal"}

// PublicLayoutModes is the allow-list for presentation_config.layout_mode. An empty layout
// mode means "the theme's own layout".
var PublicLayoutModes = []string{"standard", "compact", "minimal"}

// PublicPageSettings is the owner's view of a profile's public page.
type PublicPageSettings struct {
	Username       string      `json:"username"`
	Enabled        bool        `json:"enabled"`
	VisibleModules []string    `json:"visible_modules"`
	BankAccounts   []uuid.UUID `json:"bank_account_uuids"`
	// BackgroundUUID is the Background Studio design behind the page (nil = default).
	BackgroundUUID *uuid.UUID `json:"background_uuid"`
	// LeadCaptureEnabled shows the visitor contact form (presentation_config.lead_capture, TEC-552).
	LeadCaptureEnabled bool `json:"lead_capture_enabled"`
	// ContactExchangeEnabled lets visitors share their own details back
	// (presentation_config.contact_exchange, TEC-567).
	ContactExchangeEnabled bool `json:"contact_exchange_enabled"`
	// Theme is the profile's own theme (presentation_config.theme); "" = inherit (TEC-601).
	Theme string `json:"theme"`
	// LayoutMode overrides the theme's layout (presentation_config.layout_mode); "" = theme default.
	LayoutMode string `json:"layout_mode"`
	// InheritedTheme is the account-level (company) theme used when Theme is empty; "" = none.
	InheritedTheme string `json:"inherited_theme"`
	// InheritedLayoutMode is the company layout used when LayoutMode is empty; "" = none.
	InheritedLayoutMode string `json:"inherited_layout_mode"`
	// BrandingLocked means the company look (account_branding, TEC-946) overrides this
	// profile's own theme, layout and cover on the public page.
	BrandingLocked bool `json:"branding_locked"`
}

// AccountBranding is the company look of an account (account_branding, TEC-946) as the public
// page applies it. URLs are API-owned media URLs; empty fields are unset.
type AccountBranding struct {
	Theme        string
	LayoutMode   string
	PrimaryColor string
	LogoURL      string
	CoverURL     string
	// Locked: the company's set fields win over the profile's; otherwise they are defaults.
	Locked bool
}

// UpdatePublicPageInput changes a profile's public page. Nil fields are left unchanged.
type UpdatePublicPageInput struct {
	Username        *string      `json:"username"`
	Enabled         *bool        `json:"enabled"`
	VisibleModules  *[]string    `json:"visible_modules"`
	BankAccounts    *[]uuid.UUID `json:"bank_account_uuids"`
	Background      OptionalUUID `json:"background_uuid"`
	LeadCapture     *bool        `json:"lead_capture_enabled"`
	ContactExchange *bool        `json:"contact_exchange_enabled"`
	// Theme and LayoutMode: "" clears (inherit / theme default), nil keeps.
	Theme      *string `json:"theme"`
	LayoutMode *string `json:"layout_mode"`
}

// OptionalUUID tells "absent" (unchanged) from null (clear) and a value (set) in a PATCH-like body.
type OptionalUUID struct {
	Set   bool
	Value *uuid.UUID
}

func (o *OptionalUUID) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var id uuid.UUID
	if err := json.Unmarshal(b, &id); err != nil {
		return err
	}
	o.Value = &id
	return nil
}

// PublicTranslation is the visitor-safe part of a translation (no private note).
type PublicTranslation struct {
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
	IsDefault   bool   `json:"is_default"`
}

// PublicContact is one contact entry shown to visitors.
type PublicContact struct {
	Category  string `json:"category"`
	Type      string `json:"type"`
	Label     string `json:"label"`
	Value     string `json:"value"`
	IsPrimary bool   `json:"is_primary"`
}

// PublicAddress is one address shown to visitors.
type PublicAddress struct {
	Type       string `json:"type"`
	Label      string `json:"label"`
	Street     string `json:"street"`
	Extended   string `json:"extended"`
	City       string `json:"city"`
	Region     string `json:"region"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

// PublicSocialLink is one active social link.
type PublicSocialLink struct {
	Platform string `json:"platform"`
	Label    string `json:"label"`
	URL      string `json:"url"`
	IconURL  string `json:"icon_url"`
}

// PublicBankAccount is a bank account the owner chose to publish.
type PublicBankAccount struct {
	BankName       string `json:"bank_name"`
	BankIconURL    string `json:"bank_icon_url"`
	BankBrandColor string `json:"bank_brand_color"`
	AccountHolder  string `json:"account_holder"`
	IBAN           string `json:"iban"`
	Currency       string `json:"currency"`
	Title          string `json:"title"`
}

// PublicProfile is what GET /v1/public/p/{username} returns. Hidden modules come back empty.
// When the requested username was renamed, only Username and RedirectTo are set.
type PublicProfile struct {
	Username      string    `json:"username"`
	RedirectTo    string    `json:"redirect_to,omitempty"`
	ProfileUUID   uuid.UUID `json:"profile_uuid"`
	PrimaryLocale string    `json:"primary_locale"`
	AvatarURL     string    `json:"avatar_url"`
	CoverURL      string    `json:"cover_url"`
	// Verified is the platform-approved verified badge (TEC-549).
	Verified       bool                `json:"verified"`
	VisibleModules []string            `json:"visible_modules"`
	Translations   []PublicTranslation `json:"translations"`
	Contacts       []PublicContact     `json:"contacts"`
	Addresses      []PublicAddress     `json:"addresses"`
	SocialLinks    []PublicSocialLink  `json:"social_links"`
	BankAccounts   []PublicBankAccount `json:"bank_accounts"`
	// Background is the Background Studio document to render behind the page (omitted = default).
	Background json.RawMessage `json:"background,omitempty"`
	// BackgroundThumbnailURL is the design's static stand-in image, shown on devices that cannot
	// run its shader (TEC-599). Omitted when there is none.
	BackgroundThumbnailURL string `json:"background_thumbnail_url,omitempty"`
	// Theme is the resolved theme: the company's when branding is locked, else the profile's own,
	// else the company's default, else "".
	Theme string `json:"theme"`
	// LayoutMode is the resolved layout override; "" = the theme's layout.
	LayoutMode string `json:"layout_mode"`
	// PrimaryColor is the company's brand color (#rrggbb, TEC-946); "" = the theme's.
	PrimaryColor string `json:"primary_color"`
	// CompanyLogoURL is the company logo from its branding; "" = none.
	CompanyLogoURL string `json:"company_logo_url"`
	// PrimaryAction is the main button chosen by the smart link that opened the page
	// (save_contact, call, email, whatsapp, website); "" = save contact (TEC-556).
	PrimaryAction string `json:"primary_action,omitempty"`
}
