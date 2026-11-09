package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound  = errors.New("company structure not found")
	ErrForbidden = errors.New("company structure access denied")
	ErrInvalid   = errors.New("invalid company structure request")
	ErrConflict  = errors.New("company structure is in use or already exists")
)

type OfficeInput struct {
	Name      string `json:"name"`
	City      string `json:"city"`
	Address   string `json:"address"`
	Phone     string `json:"phone"`
	SortOrder int32  `json:"sort_order"`
}
type Office struct {
	OfficeInput
	UUID      uuid.UUID `json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type GroupInput struct {
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	OfficeUUID *uuid.UUID `json:"office_uuid"`
}
type Group struct {
	GroupInput
	UUID      uuid.UUID `json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GroupFilter narrows group lists. OfficeUUID and CompanyWide are mutually exclusive;
// the zero value lists every group of the account.
type GroupFilter struct {
	OfficeUUID  *uuid.UUID
	CompanyWide bool
}
type Assignment struct {
	UserUUID   uuid.UUID  `json:"user_uuid"`
	OfficeUUID *uuid.UUID `json:"office_uuid"`
	GroupUUID  *uuid.UUID `json:"group_uuid"`
}
type SectorInput struct {
	Slug   string `json:"slug"`
	NameTR string `json:"name_tr"`
	NameEN string `json:"name_en"`
}
type Sector struct {
	SectorInput
	UUID      uuid.UUID `json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
