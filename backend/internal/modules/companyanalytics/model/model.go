// Package model defines the company analytics contracts (TEC-574).
package model

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrForbidden: not a company account, no permission, or a role without analytics access.
	ErrForbidden = errors.New("company analytics forbidden")
)

// Event types and route types the metrics are derived from.
const (
	EventProfileView  = "card.view"
	EventContactSave  = "vcard.download"
	EventQRScan       = "card.qr.scan"
	EventRouteScan    = "route.scan"
	RouteTypeCard     = "card_route"
	RouteTypeLegacy   = "legacy_route"
	RouteTypeSmartURL = "custom_smart_link"
)

// clickPrefixes are the public profile interactions counted as contact clicks.
var clickPrefixes = []string{"contact.", "social.", "bank."}

// Metrics are the counters shown for the company and for each employee profile.
type Metrics struct {
	TotalEvents   int64 `json:"total_events"`
	ProfileViews  int64 `json:"profile_views"`
	CardTaps      int64 `json:"card_taps"`
	QRScans       int64 `json:"qr_scans"`
	ContactSaves  int64 `json:"contact_saves"`
	LinkClicks    int64 `json:"link_clicks"`
	ContactClicks int64 `json:"contact_clicks"`
	Leads         int64 `json:"leads"`
}

// AddEvent adds n aggregated events of one subject/event type. Every event counts towards
// TotalEvents; the named counters are subsets of it. Leads are not events (see Leads).
func (m *Metrics) AddEvent(subjectType, routeType, eventType string, n int64) {
	m.TotalEvents += n
	switch {
	case eventType == EventProfileView:
		m.ProfileViews += n
	case eventType == EventContactSave:
		m.ContactSaves += n
	case eventType == EventQRScan:
		m.QRScans += n
	case eventType == EventRouteScan && subjectType == "route" &&
		(routeType == RouteTypeCard || routeType == RouteTypeLegacy):
		m.CardTaps += n
	case eventType == EventRouteScan && subjectType == "route" && routeType == RouteTypeSmartURL:
		m.LinkClicks += n
	default:
		for _, p := range clickPrefixes {
			if strings.HasPrefix(eventType, p) {
				m.ContactClicks += n
				break
			}
		}
	}
}

// Window is the effective date range after the plan's history clamp.
type Window struct {
	From time.Time
	To   time.Time
	// HistoryDays is the plan's analytics.history_days; nil = unlimited.
	HistoryDays *int32
}

// Filter is the shared query of every company analytics endpoint.
type Filter struct {
	From, To              time.Time // UTC dates (00:00)
	OfficeUUID, GroupUUID *uuid.UUID
	// Employees list only.
	Q, Sort       string
	Limit, Offset int32
	// Breakdown only.
	Dimension, EventType string
	TopN                 int32
}

// EventCount is one event type's total in the scope.
type EventCount struct {
	EventType string `json:"event_type"`
	Count     int64  `json:"count"`
}

// Summary is the company (or office-manager scope) total over the range.
type Summary struct {
	From           string       `json:"from"`
	To             string       `json:"to"`
	HistoryDays    *int32       `json:"history_days"`
	Totals         Metrics      `json:"totals"`
	TopEvents      []EventCount `json:"top_events"`
	Profiles       int64        `json:"profiles"`
	ActiveProfiles int64        `json:"active_profiles"`
}

// Point is one UTC day of the time series.
type Point struct {
	Date string `json:"date"`
	Metrics
}

// TimeSeries is the zero-filled daily series over the range.
type TimeSeries struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	HistoryDays *int32  `json:"history_days"`
	Points      []Point `json:"points"`
}

// BreakdownItem is one dimension value.
type BreakdownItem struct {
	Key        string  `json:"key"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// Breakdown is the top-N distribution of a visitor dimension.
type Breakdown struct {
	Dimension string          `json:"dimension"`
	Total     int64           `json:"total"`
	Items     []BreakdownItem `json:"items"`
}

// Employee is one comparison row: a live profile of the company and its linked employee.
type Employee struct {
	ProfileUUID  uuid.UUID  `json:"profile_uuid"`
	ProfileName  string     `json:"profile_name"`
	Title        string     `json:"title"`
	MemberUUID   *uuid.UUID `json:"member_uuid"`
	MemberName   string     `json:"member_name"`
	MemberEmail  string     `json:"member_email"`
	OfficeUUID   *uuid.UUID `json:"office_uuid"`
	OfficeName   string     `json:"office_name"`
	GroupUUID    *uuid.UUID `json:"group_uuid"`
	GroupName    string     `json:"group_name"`
	Metrics      Metrics    `json:"metrics"`
	SharePercent float64    `json:"share_percent"`
}

// Dimensions are the breakdown dimensions stored on event_aggregates_daily.
var Dimensions = []string{"country", "city", "device", "browser", "os", "referer"}

// EmployeeSorts are the sortable fields of the comparison list (prefix "-" for descending).
var EmployeeSorts = []string{
	"name", "total_events", "profile_views", "card_taps", "qr_scans",
	"contact_saves", "link_clicks", "contact_clicks", "leads",
}

// SortValue returns the metric a sort field orders by.
func (m Metrics) SortValue(field string) int64 {
	switch field {
	case "profile_views":
		return m.ProfileViews
	case "card_taps":
		return m.CardTaps
	case "qr_scans":
		return m.QRScans
	case "contact_saves":
		return m.ContactSaves
	case "link_clicks":
		return m.LinkClicks
	case "contact_clicks":
		return m.ContactClicks
	case "leads":
		return m.Leads
	default:
		return m.TotalEvents
	}
}
