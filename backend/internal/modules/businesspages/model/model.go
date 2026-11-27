// Package model holds the ComCode Business page types (TEC-1327): a company account's own link
// page at /{username}, made of ordered, switchable blocks.
package model

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrForbidden: not a company account, a missing permission, or a write without owner/admin.
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound: the account has no business page yet, or the visitor path is not a live page.
	ErrNotFound = errors.New("business page not found")
	// ErrInvalid wraps a validation message.
	ErrInvalid = errors.New("invalid business page")
	// ErrUsernameTaken: another profile, card, smart link or page holds the path.
	ErrUsernameTaken = errors.New("username is already taken")
)

// Page status.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Block types.
const (
	BlockHeader       = "header"
	BlockButton       = "button"
	BlockSocial       = "social"
	BlockOffices      = "offices"
	BlockTeam         = "team"
	BlockBankAccounts = "bank_accounts"
)

// BlockTypes lists every block type.
var BlockTypes = []string{BlockHeader, BlockButton, BlockSocial, BlockOffices, BlockTeam, BlockBankAccounts}

// Button targets.
const (
	TargetURL          = "url"
	TargetPhone        = "phone"
	TargetWhatsApp     = "whatsapp"
	TargetEmail        = "email"
	TargetMaps         = "maps"
	TargetGoogleReview = "google_review"
	// TargetLeadForm opens the page's own lead form (lead capture module, TEC-552); no value.
	TargetLeadForm = "lead_form"
)

// ButtonTargets lists every button target.
var ButtonTargets = []string{TargetURL, TargetPhone, TargetWhatsApp, TargetEmail, TargetMaps, TargetGoogleReview, TargetLeadForm}

// Locales a page can be written in.
var Locales = []string{"tr", "en"}

// MaxBlocks caps the blocks of one page.
const MaxBlocks = 50

// PageText is the page's name and one-line description in one locale.
type PageText struct {
	Name    string `json:"name"`
	Tagline string `json:"tagline"`
}

// BlockText is a block's texts in one locale (a button's title and subtitle, a section title).
type BlockText struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

// HeaderConfig picks the logo and cover from the company media library; nil = company branding.
type HeaderConfig struct {
	LogoMediaUUID  *uuid.UUID `json:"logo_media_uuid"`
	CoverMediaUUID *uuid.UUID `json:"cover_media_uuid"`
}

// ButtonConfig is an action button: where it goes and its icon.
type ButtonConfig struct {
	Target string `json:"target"`
	// Value is the URL, phone number, e-mail address or map address; empty for lead_form.
	Value string `json:"value"`
	// Icon is a short icon name for the UI ("" = the target's default).
	Icon string `json:"icon"`
}

// SocialLink is one icon of the social row.
type SocialLink struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

// SocialConfig is the social icon row.
type SocialConfig struct {
	Links []SocialLink `json:"links"`
}

// BankAccountsConfig picks the company bank accounts to publish.
type BankAccountsConfig struct {
	BankAccountUUIDs []uuid.UUID `json:"bank_account_uuids"`
}

// Block is one block of the page as the owner edits it.
type Block struct {
	UUID         uuid.UUID            `json:"uuid"`
	Type         string               `json:"type"`
	Enabled      bool                 `json:"enabled"`
	Config       json.RawMessage      `json:"config"`
	Translations map[string]BlockText `json:"translations"`
}

// Page is the owner's view of the business page.
type Page struct {
	UUID          uuid.UUID           `json:"uuid"`
	Status        string              `json:"status"`
	Username      string              `json:"username"`
	Theme         string              `json:"theme"`
	PrimaryLocale string              `json:"primary_locale"`
	Translations  map[string]PageText `json:"translations"`
	Blocks        []Block             `json:"blocks"`
	PublishedAt   *time.Time          `json:"published_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

// BlockInput is one block in a save; the order of the list is the page order.
type BlockInput struct {
	Type string `json:"type"`
	// Enabled: nil = true.
	Enabled      *bool                `json:"enabled"`
	Config       json.RawMessage      `json:"config"`
	Translations map[string]BlockText `json:"translations"`
}

// SaveInput replaces the page: settings and the full block list.
type SaveInput struct {
	// Username is required for the first save; afterwards a different value renames the page.
	Username      string              `json:"username"`
	Theme         string              `json:"theme"`
	PrimaryLocale string              `json:"primary_locale"`
	Translations  map[string]PageText `json:"translations"`
	Blocks        []BlockInput        `json:"blocks"`
}

// PublicButton is an action button resolved for visitors.
type PublicButton struct {
	Target string `json:"target"`
	Value  string `json:"value"`
	Icon   string `json:"icon"`
}

// PublicSocialLink is one social icon with its platform icon.
type PublicSocialLink struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
	IconURL  string `json:"icon_url"`
}

// PublicOffice is one company office with its address.
type PublicOffice struct {
	Name     string `json:"name"`
	City     string `json:"city"`
	District string `json:"district"`
	Address  string `json:"address"`
	Phone    string `json:"phone"`
}

// PublicTeamMember links to a published employee profile.
type PublicTeamMember struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Title     string `json:"title"`
	AvatarURL string `json:"avatar_url"`
}

// PublicBankAccount is a published company bank account.
type PublicBankAccount struct {
	BankName       string `json:"bank_name"`
	BankIconURL    string `json:"bank_icon_url"`
	BankBrandColor string `json:"bank_brand_color"`
	AccountHolder  string `json:"account_holder"`
	IBAN           string `json:"iban"`
	Currency       string `json:"currency"`
	Title          string `json:"title"`
}

// PublicBlock is an enabled block resolved for visitors. Only the field of its type is set.
type PublicBlock struct {
	Type         string               `json:"type"`
	Translations map[string]BlockText `json:"translations"`
	Button       *PublicButton        `json:"button,omitempty"`
	Social       []PublicSocialLink   `json:"social,omitempty"`
	Offices      []PublicOffice       `json:"offices,omitempty"`
	Team         []PublicTeamMember   `json:"team,omitempty"`
	BankAccounts []PublicBankAccount  `json:"bank_accounts,omitempty"`
}

// PublicPage is what GET /v1/public/business/{username} returns. A renamed username returns only
// Username and RedirectTo.
type PublicPage struct {
	Username      string              `json:"username"`
	RedirectTo    string              `json:"redirect_to,omitempty"`
	PageUUID      uuid.UUID           `json:"page_uuid"`
	CompanyName   string              `json:"company_name"`
	PrimaryLocale string              `json:"primary_locale"`
	Translations  map[string]PageText `json:"translations"`
	// Theme is the page's own theme, else the company branding theme, else "".
	Theme        string `json:"theme"`
	PrimaryColor string `json:"primary_color"`
	// LogoURL and CoverURL come from the header block, else the company branding.
	LogoURL  string        `json:"logo_url"`
	CoverURL string        `json:"cover_url"`
	Blocks   []PublicBlock `json:"blocks"`
}
