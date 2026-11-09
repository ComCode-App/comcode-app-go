// Package model holds the platform overview (TEC-594) payload: platform-wide business KPIs for
// the /platform home.
package model

import (
	"time"

	"github.com/google/uuid"
)

// WindowDays is the look-back of the "last 30 days" figures.
const WindowDays = 30

// TopProfilesLimit caps the most viewed profiles list.
const TopProfilesLimit = 10

// EventTypesLimit caps the per event type breakdown of the last 24 hours.
const EventTypesLimit = 8

// ProfileViewEvent is the event the public profile page records on every render.
const ProfileViewEvent = "card.view"

// OrderStatuses is the order lifecycle in funnel order (docs/COMMERCE_AND_ORDERS.md).
var OrderStatuses = []string{
	"pending", "paid", "design_pending", "design_approved", "in_production",
	"ready_to_ship", "shipped", "delivered", "cancelled", "refunded",
}

// Overview is the GET /v1/platform/overview payload.
type Overview struct {
	GeneratedAt   time.Time         `json:"generated_at"`
	WindowDays    int               `json:"window_days"`
	Users         UserStats         `json:"users"`
	Profiles      ProfileStats      `json:"profiles"`
	Cards         CardStats         `json:"cards"`
	Orders        OrderFunnel       `json:"orders"`
	Subscriptions SubscriptionStats `json:"subscriptions"`
	TopProfiles   []TopProfile      `json:"top_profiles"`
	Events24h     EventVolume       `json:"events_last_24h"`
}

// UserStats counts non-deleted users.
type UserStats struct {
	Total int64 `json:"total"`
	// ActiveLast30d signed in during the window.
	ActiveLast30d int64 `json:"active_last_30d"`
	NewLast30d    int64 `json:"new_last_30d"`
}

// ProfileStats counts non-deleted profiles; published ones have a live username page.
type ProfileStats struct {
	Total     int64 `json:"total"`
	Published int64 `json:"published"`
}

// CardStats counts non-deleted NFC cards.
type CardStats struct {
	Total     int64 `json:"total"`
	Activated int64 `json:"activated"`
	// ActivationRate is activated / total in [0, 1]; 0 when there are no cards.
	ActivationRate float64 `json:"activation_rate"`
}

// OrderFunnel groups orders created in the window by their current status.
type OrderFunnel struct {
	Total int64 `json:"total"`
	// Funnel lists every status of OrderStatuses in order, zero when absent.
	Funnel []StatusCount `json:"funnel"`
}

// StatusCount is one funnel step.
type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// SubscriptionStats counts subscriptions by status and the monthly recurring revenue.
type SubscriptionStats struct {
	Active          int64 `json:"active"`
	Trialing        int64 `json:"trialing"`
	PastDue         int64 `json:"past_due"`
	Canceled        int64 `json:"canceled"`
	CanceledLast30d int64 `json:"canceled_last_30d"`
	// MRR has one entry per currency (plans are TRY only today): active subscriptions only.
	MRR []MRR `json:"mrr"`
	// AtRiskMRR is the monthly revenue of past_due subscriptions, per currency. It is not part of
	// MRR (TEC-944).
	AtRiskMRR []MRR `json:"at_risk_mrr"`
}

// MRR is the monthly recurring revenue of one currency in minor units (kuruş).
type MRR struct {
	Currency string `json:"currency"`
	// AmountMinor sums active monthly plans and yearly plans / 12.
	AmountMinor int64 `json:"amount_minor"`
	// ManualAmountMinor is the part from manual (admin-created) subscriptions; the rest is from
	// online payments.
	ManualAmountMinor int64 `json:"manual_amount_minor"`
	Subscriptions     int64 `json:"subscriptions"`
}

// TopProfile is one of the most viewed profiles of the window.
type TopProfile struct {
	UUID           uuid.UUID `json:"uuid"`
	Name           string    `json:"name"`
	CompanyName    string    `json:"company_name"`
	Username       string    `json:"username"`
	Views          int64     `json:"views"`
	UniqueSessions int64     `json:"unique_sessions"`
}

// EventVolume is the event count of the last 24 hourly buckets (current hour included).
type EventVolume struct {
	Total int64 `json:"total"`
	// Hourly has 24 points, oldest first, zero for hours without events.
	Hourly []HourCount      `json:"hourly"`
	ByType []EventTypeCount `json:"by_type"`
}

// HourCount is one hourly bucket.
type HourCount struct {
	Bucket time.Time `json:"bucket"`
	Count  int64     `json:"count"`
}

// EventTypeCount is the volume of one event type.
type EventTypeCount struct {
	EventType string `json:"event_type"`
	Count     int64  `json:"count"`
}

// Conversion funnel (TEC-944), GET /v1/platform/overview/funnel.

// FunnelPeriods are the selectable look-backs of the conversion funnel, in days. A fixed set keeps
// the query bounded and the cache small.
var FunnelPeriods = []int{7, 30, 90, 365}

// DefaultFunnelDays is the period used when none is given.
const DefaultFunnelDays = 30

// Funnel stages in order. "created" is every order opened in the period; each later stage counts
// the orders that reached that status or a later one of the lifecycle.
const (
	StageCreated      = "created"
	StagePaid         = "paid"
	StageInProduction = "in_production"
	StageShipped      = "shipped"
	StageDelivered    = "delivered"
)

// ConversionFunnel is the GET /v1/platform/overview/funnel payload.
type ConversionFunnel struct {
	GeneratedAt time.Time `json:"generated_at"`
	PeriodDays  int       `json:"period_days"`
	// From and To bound the creation time of the counted orders: [From, To).
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	// Cancelled counts the period's orders that are now cancelled or refunded.
	Cancelled int64         `json:"cancelled"`
	Stages    []FunnelStage `json:"stages"`
}

// FunnelStage is one step of the conversion funnel.
type FunnelStage struct {
	Stage  string `json:"stage"`
	Orders int64  `json:"orders"`
	// ConversionFromPrevious is Orders / the previous stage's orders in [0, 1]; 0 when the
	// previous stage is empty. The created stage is 1 (or 0 without orders).
	ConversionFromPrevious float64 `json:"conversion_from_previous"`
	// ConversionFromCreated is Orders / created orders in [0, 1].
	ConversionFromCreated float64 `json:"conversion_from_created"`
	// AvgHoursFromCreated is the mean time from order creation to the first recorded entry into
	// the stage; null for the created stage and when no entry was recorded (backfilled history).
	AvgHoursFromCreated *float64 `json:"avg_hours_from_created"`
	// TimedOrders is how many orders the average is based on.
	TimedOrders int64 `json:"timed_orders"`
}
