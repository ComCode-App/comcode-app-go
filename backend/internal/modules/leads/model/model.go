// Package model holds the lead capture (TEC-552) and contact exchange (TEC-567) DTOs.
package model

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ExchangeCopyTemplate is the notification template of the email a visitor gets after an
// exchange (TEC-940): what they shared, the owner's vCard and a KVKK delete link.
const ExchangeCopyTemplate = "leads.exchange_copy"

// ExchangeCopyEvent is the source_event of an exchange's visitor copy email; the email row is
// deleted together with the exchange (migration 000059).
func ExchangeCopyEvent(id uuid.UUID) string { return ExchangeCopyTemplate + ":" + id.String() }

// Lead sources: the kind of route the visitor submitted through.
const (
	SourceProfile   = "profile"
	SourceCard      = "card"
	SourceSmartLink = "smart_link"
)

// Sources lists every valid source_type.
var Sources = []string{SourceProfile, SourceCard, SourceSmartLink}

// Lead kinds: a contact form the owner asked for, or a visitor sharing their details back.
const (
	KindForm     = "form"
	KindExchange = "exchange"
)

// Kinds lists every valid kind.
var Kinds = []string{KindForm, KindExchange}

// ConsentVersion names the KVKK consent text the visitor form shows. Bump it (and the
// frontend `leads.form.consent_text`) whenever the text changes; older versions stay valid for
// forms already open in a browser until they are removed from ConsentVersions.
const ConsentVersion = "kvkk-lead-v1"

// ConsentVersions are the consent texts the API still accepts.
var ConsentVersions = []string{ConsentVersion}

// ExchangeConsentVersion names the KVKK text of the contact exchange (frontend
// `leads.exchange.consent_text`). Same rules as ConsentVersion.
const ExchangeConsentVersion = "kvkk-exchange-v1"

// ExchangeConsentVersions are the exchange consent texts the API still accepts.
var ExchangeConsentVersions = []string{ExchangeConsentVersion}

// Field limits (mirrored in the migration and the visitor form).
const (
	MaxName    = 150
	MaxEmail   = 254
	MaxPhone   = 32
	MaxMessage = 2000
	MaxCompany = 150
	MaxTitle   = 150
)

// Lead is the app projection. Visitor IP and user agent are stored as consent evidence only.
type Lead struct {
	UUID        uuid.UUID  `json:"uuid"`
	SourceType  string     `json:"source_type"`
	RoutePath   string     `json:"route_path"`
	ProfileUUID *uuid.UUID `json:"profile_uuid,omitempty"`
	CardUUID    *uuid.UUID `json:"card_uuid,omitempty"`
	Kind        string     `json:"kind"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Message     string     `json:"message"`
	Company     string     `json:"company"`
	JobTitle    string     `json:"job_title"`
	// SenderProfileUUID and SenderUsername: the ComCode profile a signed-in visitor shared
	// (exchange only). SenderUsername is empty when that profile has no live public page.
	SenderProfileUUID *uuid.UUID `json:"sender_profile_uuid,omitempty"`
	SenderUsername    string     `json:"sender_username,omitempty"`
	Locale            string     `json:"locale"`
	ConsentAt         time.Time  `json:"consent_at"`
	ConsentVersion    string     `json:"consent_version"`
	ReadAt            *time.Time `json:"read_at,omitempty"`
	// Locked: received over the account's monthly quota (TEC-941). The name, email, phone,
	// message and sender are masked until a plan upgrade or a new month unlocks it.
	Locked    bool      `json:"locked"`
	CreatedAt time.Time `json:"created_at"`
}

// Mask hides what a locked lead must not show: who sent it and how to reach them. The message
// goes too, since visitors often repeat their contact details in it. Company and title stay as
// a hint of what the upgrade unlocks.
func (l *Lead) Mask() {
	l.Name, l.Email, l.Phone, l.Message = MaskName(l.Name), MaskEmail(l.Email), MaskPhone(l.Phone), ""
	l.SenderProfileUUID, l.SenderUsername = nil, ""
}

// MaskName keeps the first letter of each word: "Ayşe Yılmaz" -> "A*** Y***".
func MaskName(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r, _ := utf8.DecodeRuneInString(w)
		words[i] = string(r) + "***"
	}
	return strings.Join(words, " ")
}

// MaskEmail keeps the first letter of the local part: "ayse@example.com" -> "a***@***".
func MaskEmail(s string) string {
	if s == "" {
		return ""
	}
	r, _ := utf8.DecodeRuneInString(s)
	return string(r) + "***@***"
}

// MaskPhone keeps the last two digits: "+90 532 111 22 33" -> "*** 33".
func MaskPhone(s string) string {
	digits := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) < 2 {
		return ""
	}
	return "*** " + string(digits[len(digits)-2:])
}

// SubmitInput is the visitor form body of POST /v1/public/leads.
type SubmitInput struct {
	// Path is the public route the visitor came through: a username, `c/{code}` or `s/{slug}`.
	Path string `json:"path"`
	// Via is the card or smart link that opened the profile page (`c/{code}` / `s/{slug}`). When
	// the form is the profile's own, the lead is attributed to it if it opens the same profile.
	Via            string `json:"via"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Message        string `json:"message"`
	Locale         string `json:"locale"`
	Consent        bool   `json:"consent"`
	ConsentVersion string `json:"consent_version"`
}

// ExchangeInput is the body of POST /v1/public/exchanges: a visitor shares their details with
// the owner of the profile page they are on.
type ExchangeInput struct {
	// Path is the profile page's username. Via is the card or smart link the visitor came through
	// (`c/{code}` / `s/{slug}`); the exchange is attributed to it when it opens the same profile.
	Path           string `json:"path"`
	Via            string `json:"via"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Company        string `json:"company"`
	JobTitle       string `json:"job_title"`
	Locale         string `json:"locale"`
	Consent        bool   `json:"consent"`
	ConsentVersion string `json:"consent_version"`
}

// ShareProfileInput is the body of POST /v1/exchanges: a signed-in visitor shares one of the
// profiles of their active account. The details are copied from that profile.
type ShareProfileInput struct {
	Path           string    `json:"path"`
	Via            string    `json:"via"`
	ProfileUUID    uuid.UUID `json:"profile_uuid"`
	Locale         string    `json:"locale"`
	Consent        bool      `json:"consent"`
	ConsentVersion string    `json:"consent_version"`
}

// Visitor carries request facts stored with the consent.
type Visitor struct {
	IP        string
	UserAgent string
}

// SubmitResult is returned to the visitor.
type SubmitResult struct {
	UUID      uuid.UUID `json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
}

// DeleteExchangeInput is the body of POST /v1/public/exchanges/delete: the signed token of the
// "delete my data" link in the visitor copy email.
type DeleteExchangeInput struct {
	Token string `json:"token"`
}

// FormConfig tells a visitor page whether to show the form for a path.
type FormConfig struct {
	Enabled        bool       `json:"enabled"`
	Path           string     `json:"path"`
	SourceType     string     `json:"source_type,omitempty"`
	ProfileUUID    *uuid.UUID `json:"profile_uuid,omitempty"`
	ConsentVersion string     `json:"consent_version"`
	// ExchangeEnabled: the profile page offers contact exchange (TEC-567). Only a profile's own
	// path (its username) reports it.
	ExchangeEnabled        bool   `json:"exchange_enabled"`
	ExchangeConsentVersion string `json:"exchange_consent_version"`
}

// ListFilter narrows the app list.
type ListFilter struct {
	Q          *string
	SourceType *string
	Kind       *string
	Unread     *bool
	Limit      int32
	Offset     int32
}
