// Package model holds the verified profile badge types (TEC-549).
package model

import (
	"time"

	"github.com/google/uuid"
)

// Profile verification status as owners and visitors see it.
const (
	StatusNone     = "none"
	StatusPending  = "pending"
	StatusVerified = "verified"
	StatusRejected = "rejected"
)

// Request row status (profile_verification_requests.status).
const (
	RequestPending   = "pending"
	RequestApproved  = "approved"
	RequestRejected  = "rejected"
	RequestRevoked   = "revoked"
	RequestCancelled = "cancelled"
)

// Request origin: the owner asked, or an identity change re-opened review of a verified profile.
const (
	OriginOwner    = "owner"
	OriginReReview = "re_review"
)

// Revoke reasons recorded when a verified profile changes its identity.
const (
	ReasonDisplayNameChanged  = "display_name_changed"
	ReasonOrganizationChanged = "organization_changed"
)

// MaxNoteLength and MaxReasonLength cap free text.
const (
	MaxNoteLength   = 1000
	MaxReasonLength = 500
)

// StatusOf maps a request row status to the profile status.
func StatusOf(requestStatus string) string {
	switch requestStatus {
	case RequestPending:
		return StatusPending
	case RequestApproved:
		return StatusVerified
	case RequestRejected:
		return StatusRejected
	default:
		return StatusNone
	}
}

// OwnerView is GET /v1/profiles/{uuid}/verification: the profile's status and its latest request.
type OwnerView struct {
	Status string `json:"status"`
	// CanRequest is true when the status allows a new request (none or rejected).
	CanRequest bool `json:"can_request"`
	// Request is the latest request; nil when the profile never asked.
	Request *OwnerRequest `json:"request"`
}

// OwnerRequest is the owner-safe part of a request (no reviewer identity).
type OwnerRequest struct {
	UUID         uuid.UUID  `json:"uuid"`
	Status       string     `json:"status"`
	Origin       string     `json:"origin"`
	Note         string     `json:"note"`
	RequestedAt  time.Time  `json:"requested_at"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	ReviewReason string     `json:"review_reason"`
	RevokedAt    *time.Time `json:"revoked_at"`
	RevokeReason string     `json:"revoke_reason"`
}

// RequestInput is the body of POST /v1/profiles/{uuid}/verification.
type RequestInput struct {
	Note string `json:"note"`
}

// ReasonInput is the body of the platform reject / revoke actions.
type ReasonInput struct {
	Reason string `json:"reason"`
}

// Review is one request in the platform review list.
type Review struct {
	UUID             uuid.UUID     `json:"uuid"`
	Status           string        `json:"status"`
	Origin           string        `json:"origin"`
	Note             string        `json:"note"`
	RequestedAt      time.Time     `json:"requested_at"`
	RequestedByEmail string        `json:"requested_by_email"`
	ReviewedAt       *time.Time    `json:"reviewed_at"`
	ReviewReason     string        `json:"review_reason"`
	RevokedAt        *time.Time    `json:"revoked_at"`
	RevokeReason     string        `json:"revoke_reason"`
	ReviewedByEmail  string        `json:"reviewed_by_email"`
	Profile          ReviewProfile `json:"profile"`
	// AccountID is internal (notification recipients); never serialized.
	AccountID int64         `json:"-"`
	Account   ReviewAccount `json:"account"`
}

// ReviewProfile is the profile identity a reviewer checks.
type ReviewProfile struct {
	UUID        uuid.UUID `json:"uuid"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	CompanyName string    `json:"company_name"`
	Title       string    `json:"title"`
	AvatarURL   string    `json:"avatar_url"`
	Username    string    `json:"username"`
}

// ReviewAccount is the account that owns the profile.
type ReviewAccount struct {
	UUID uuid.UUID `json:"uuid"`
	Name string    `json:"name"`
	Type string    `json:"type"`
}

// ListFilter filters the platform review list.
type ListFilter struct {
	// Status is "" (all) or a request status.
	Status string
	Q      string
	Limit  int32
	Offset int32
}
