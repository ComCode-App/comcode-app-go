// Package model holds the platform card and route support views (TEC-950).
package model

import (
	"time"

	"github.com/google/uuid"
)

// Card activation states shown to support staff.
const (
	ActivationPending = "pending" // provisioned, activation code not used yet
	ActivationActive  = "active"
	ActivationRetired = "retired" // card route retired (order cancelled/refunded or retired by staff)
)

// RouteStatusQuarantine is shown for a deleted smart link whose path is still held (30 days).
const RouteStatusQuarantine = "quarantine"

type AccountRef struct {
	UUID uuid.UUID `json:"uuid"`
	Name string    `json:"name"`
	Type string    `json:"type,omitempty"`
}

type UserRef struct {
	UUID    uuid.UUID `json:"uuid"`
	Email   string    `json:"email"`
	Name    string    `json:"name,omitempty"`
	Surname string    `json:"surname,omitempty"`
}

type ProfileRef struct {
	UUID     uuid.UUID `json:"uuid"`
	Username string    `json:"username,omitempty"`
}

type OrderRef struct {
	UUID   uuid.UUID `json:"uuid"`
	Number string    `json:"order_number"`
	Status string    `json:"status"`
}

type RouteRef struct {
	UUID   uuid.UUID `json:"uuid"`
	Path   string    `json:"path"`
	Status string    `json:"status"`
}

type CardRef struct {
	UUID      uuid.UUID `json:"uuid"`
	SmartCode string    `json:"smart_code"`
}

// Card is one NFC card as platform support sees it.
type Card struct {
	ID               int64       `json:"-"`
	UUID             uuid.UUID   `json:"uuid"`
	Name             string      `json:"name"`
	SmartCode        string      `json:"smart_code"`
	CardUID          string      `json:"card_uid,omitempty"`
	LegacySerial     string      `json:"legacy_serial,omitempty"`
	TargetType       string      `json:"target_type"`
	DirectURL        string      `json:"direct_url,omitempty"`
	IsActive         bool        `json:"is_active"`
	IsLocked         bool        `json:"is_locked"`
	ActivationStatus string      `json:"activation_status"`
	Account          AccountRef  `json:"account"`
	Owner            UserRef     `json:"owner"`
	Profile          *ProfileRef `json:"profile,omitempty"`
	Order            *OrderRef   `json:"order,omitempty"`
	Route            *RouteRef   `json:"route,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// HistoryEntry is one route_history row. Quarantined means its old path is still held.
type HistoryEntry struct {
	OldPath              string     `json:"old_path"`
	NewPath              string     `json:"new_path"`
	Reason               string     `json:"reason"`
	ChangedBy            *UserRef   `json:"changed_by,omitempty"`
	Quarantined          bool       `json:"quarantined"`
	QuarantineUntil      *time.Time `json:"quarantine_until,omitempty"`
	QuarantineReleasedAt *time.Time `json:"quarantine_released_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

// Route is one routes row as platform support sees it, deleted smart links included.
type Route struct {
	ID        int64     `json:"-"`
	UUID      uuid.UUID `json:"uuid"`
	Path      string    `json:"path"`
	RouteType string    `json:"route_type"`
	// Status is the stored status, or "quarantine"/"retired" for a deleted smart link.
	Status               string         `json:"status"`
	IsImmutable          bool           `json:"is_immutable"`
	TargetType           string         `json:"target_type"`
	DirectURL            string         `json:"direct_url,omitempty"`
	Account              *AccountRef    `json:"account,omitempty"`
	Owner                *UserRef       `json:"owner,omitempty"`
	Profile              *ProfileRef    `json:"profile,omitempty"`
	Card                 *CardRef       `json:"card,omitempty"`
	DeletedAt            *time.Time     `json:"deleted_at,omitempty"`
	QuarantineUntil      *time.Time     `json:"quarantine_until,omitempty"`
	QuarantineReleasedAt *time.Time     `json:"quarantine_released_at,omitempty"`
	History              []HistoryEntry `json:"history"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

// ReassignTarget is the profile a card moves to and who will own the card.
type ReassignTarget struct {
	ProfileID   int64
	ProfileUUID uuid.UUID
	AccountID   int64
	OwnerUserID int64
}

// ReleaseResult lists the routes whose hold on a path was released.
type ReleaseResult struct {
	Path       string      `json:"path"`
	RouteUUIDs []uuid.UUID `json:"route_uuids"`
}
