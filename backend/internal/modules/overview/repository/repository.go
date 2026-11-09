// Package repository reads the platform overview KPIs. Every method is platform-wide (no account
// scope) by design; the route is gated by platform.overview.read. Analytics figures come from the
// aggregate tables, never from raw event rows.
package repository

import (
	"context"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/model"
)

// CardCounts is the raw card tally; the use case derives the activation rate.
type CardCounts struct {
	Total     int64
	Activated int64
}

// MRRSplit separates the revenue of active subscriptions from that of past_due ones.
type MRRSplit struct {
	Active []model.MRR
	AtRisk []model.MRR
}

// StageCounts is the raw tally of one funnel stage after "created".
type StageCounts struct {
	Reached int64
	// Timed orders have a recorded transition into the stage; Seconds sums their time from
	// creation to it.
	Timed   int64
	Seconds float64
}

// FunnelCounts is the raw conversion funnel; the use case derives rates and averages.
type FunnelCounts struct {
	Created   int64
	Cancelled int64
	// Stages is keyed by model.StagePaid, StageInProduction, StageShipped and StageDelivered.
	Stages map[string]StageCounts
}

// Repository is the overview read port.
type Repository interface {
	Users(ctx context.Context, since time.Time) (model.UserStats, error)
	Profiles(ctx context.Context) (model.ProfileStats, error)
	Cards(ctx context.Context) (CardCounts, error)
	// OrderStatusCounts groups orders created since `since` by current status.
	OrderStatusCounts(ctx context.Context, since time.Time) (map[string]int64, error)
	// Subscriptions fills the status counts; MRR is left empty.
	Subscriptions(ctx context.Context, canceledSince time.Time) (model.SubscriptionStats, error)
	// MRR lists currencies with at least one active (Active) or past_due (AtRisk) subscription.
	MRR(ctx context.Context) (MRRSplit, error)
	// ConversionFunnel reads the orders created in [since, until) and their status history.
	ConversionFunnel(ctx context.Context, since, until time.Time) (FunnelCounts, error)
	// TopProfiles reads daily aggregates from sinceDate (a UTC calendar day) on.
	TopProfiles(ctx context.Context, sinceDate time.Time, limit int) ([]model.TopProfile, error)
	// EventsByHour reads hourly aggregates from `since` on; hours without events are absent.
	EventsByHour(ctx context.Context, since time.Time) ([]model.HourCount, error)
	EventsByType(ctx context.Context, since time.Time, limit int) ([]model.EventTypeCount, error)
}
