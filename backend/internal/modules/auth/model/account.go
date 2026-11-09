package model

import "github.com/google/uuid"

// AccountMembership is an account the user belongs to, with the user's role in it.
type AccountMembership struct {
	ID     int64     `json:"-"`
	UUID   uuid.UUID `json:"uuid"`
	Type   string    `json:"type"`
	Name   string    `json:"name"`
	Slug   string    `json:"slug"`
	Status string    `json:"status"`
	Role   string    `json:"role"`
}

// MeAccount is an account entry on /v1/auth/me and /v1/auth/accounts.
type MeAccount struct {
	AccountMembership
	Active bool `json:"active"`
}

// AccountSwitchResult is returned by POST /v1/auth/accounts/switch.
type AccountSwitchResult struct {
	Tokens
	Session SessionSwitch     `json:"session"`
	Account AccountMembership `json:"account"`
}
