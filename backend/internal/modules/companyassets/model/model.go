// Package model defines company employee card and order contracts.
package model

import (
	"errors"
	"time"

	orders "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/google/uuid"
)

var (
	ErrForbidden = errors.New("company asset operation forbidden")
	ErrNotFound  = errors.New("company asset not found")
	ErrInvalid   = errors.New("invalid company asset input")
	ErrConflict  = errors.New("card is reserved, retired, cancelled or in production")
)

type Filter struct {
	Q, Status, Sort       string
	OfficeUUID, GroupUUID *uuid.UUID
	IsActive              *bool
	Limit, Offset         int32
}

type Card struct {
	UUID        uuid.UUID  `json:"uuid"`
	Name        string     `json:"name"`
	SmartCode   string     `json:"smart_code"`
	CardUID     string     `json:"card_uid"`
	Status      string     `json:"status"`
	IsActive    bool       `json:"is_active"`
	IsLocked    bool       `json:"is_locked"`
	ProfileUUID *uuid.UUID `json:"profile_uuid"`
	RouteUUID   *uuid.UUID `json:"route_uuid"`
	RoutePath   string     `json:"route_path"`
	RouteStatus string     `json:"route_status"`
	IsImmutable bool       `json:"is_immutable"`
	MemberUUID  *uuid.UUID `json:"member_uuid"`
	MemberName  string     `json:"member_name"`
	OfficeUUID  *uuid.UUID `json:"office_uuid"`
	OfficeName  string     `json:"office_name"`
	GroupUUID   *uuid.UUID `json:"group_uuid"`
	GroupName   string     `json:"group_name"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Order struct {
	UUID              uuid.UUID `json:"uuid"`
	OrderNumber       string    `json:"order_number"`
	Status            string    `json:"status"`
	PaymentStatus     string    `json:"payment_status"`
	FulfillmentStatus string    `json:"fulfillment_status"`
	Currency          string    `json:"currency"`
	GrandTotal        string    `json:"grand_total"`
	PlacedByUUID      uuid.UUID `json:"placed_by_uuid"`
	CreatedAt         time.Time `json:"created_at"`
}

// ReorderInput creates one pending/unpaid item per selected member, using current catalog prices.
// The company owns the order; no payment or card provisioning starts here.
type ReorderInput struct {
	MemberUUIDs     []uuid.UUID    `json:"member_uuids"`
	ProductUUID     uuid.UUID      `json:"product_uuid"`
	VariantUUID     *uuid.UUID     `json:"variant_uuid,omitempty"`
	ShippingAddress orders.Address `json:"shipping_address"`
	BillingAddress  orders.Address `json:"billing_address"`
	CustomerNote    string         `json:"customer_note,omitempty"`
}
