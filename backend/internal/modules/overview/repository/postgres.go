package repository

import (
	"context"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/model"
	"github.com/jackc/pgx/v5/pgtype"
)

// Postgres implements Repository with sqlc.
type Postgres struct {
	q db.Querier
}

// NewPostgres creates the sqlc-backed repository.
func NewPostgres(q db.Querier) *Postgres { return &Postgres{q: q} }

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func (r *Postgres) Users(ctx context.Context, since time.Time) (model.UserStats, error) {
	row, err := r.q.PlatformOverviewUsers(ctx, ts(since))
	if err != nil {
		return model.UserStats{}, err
	}
	return model.UserStats{Total: row.Total, ActiveLast30d: row.Active, NewLast30d: row.NewUsers}, nil
}

func (r *Postgres) Profiles(ctx context.Context) (model.ProfileStats, error) {
	row, err := r.q.PlatformOverviewProfiles(ctx)
	if err != nil {
		return model.ProfileStats{}, err
	}
	return model.ProfileStats{Total: row.Total, Published: row.Published}, nil
}

func (r *Postgres) Cards(ctx context.Context) (CardCounts, error) {
	row, err := r.q.PlatformOverviewCards(ctx)
	if err != nil {
		return CardCounts{}, err
	}
	return CardCounts{Total: row.Total, Activated: row.Activated}, nil
}

func (r *Postgres) OrderStatusCounts(ctx context.Context, since time.Time) (map[string]int64, error) {
	rows, err := r.q.PlatformOverviewOrderStatusCounts(ctx, ts(since))
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Status] += row.Count
	}
	return out, nil
}

func (r *Postgres) Subscriptions(ctx context.Context, canceledSince time.Time) (model.SubscriptionStats, error) {
	row, err := r.q.PlatformOverviewSubscriptionCounts(ctx, ts(canceledSince))
	if err != nil {
		return model.SubscriptionStats{}, err
	}
	return model.SubscriptionStats{
		Active: row.Active, Trialing: row.Trialing, PastDue: row.PastDue,
		Canceled: row.Canceled, CanceledLast30d: row.CanceledSince,
	}, nil
}

func (r *Postgres) MRR(ctx context.Context) (MRRSplit, error) {
	rows, err := r.q.PlatformOverviewMRR(ctx)
	if err != nil {
		return MRRSplit{}, err
	}
	out := MRRSplit{Active: []model.MRR{}, AtRisk: []model.MRR{}}
	for _, row := range rows {
		if row.Subscriptions > 0 {
			out.Active = append(out.Active, model.MRR{
				Currency: row.Currency, AmountMinor: row.MrrMinor,
				ManualAmountMinor: row.ManualMrrMinor, Subscriptions: row.Subscriptions,
			})
		}
		if row.AtRiskSubscriptions > 0 {
			out.AtRisk = append(out.AtRisk, model.MRR{
				Currency: row.Currency, AmountMinor: row.AtRiskMrrMinor,
				ManualAmountMinor: row.AtRiskManualMrrMinor, Subscriptions: row.AtRiskSubscriptions,
			})
		}
	}
	return out, nil
}

func (r *Postgres) ConversionFunnel(ctx context.Context, since, until time.Time) (FunnelCounts, error) {
	row, err := r.q.PlatformOverviewConversionFunnel(ctx, db.PlatformOverviewConversionFunnelParams{
		Since: ts(since), Until: ts(until),
	})
	if err != nil {
		return FunnelCounts{}, err
	}
	return FunnelCounts{
		Created:   row.Created,
		Cancelled: row.Cancelled,
		Stages: map[string]StageCounts{
			model.StagePaid:         {Reached: row.Paid, Timed: row.PaidTimed, Seconds: row.PaidSeconds},
			model.StageInProduction: {Reached: row.Produced, Timed: row.ProducedTimed, Seconds: row.ProducedSeconds},
			model.StageShipped:      {Reached: row.Shipped, Timed: row.ShippedTimed, Seconds: row.ShippedSeconds},
			model.StageDelivered:    {Reached: row.Delivered, Timed: row.DeliveredTimed, Seconds: row.DeliveredSeconds},
		},
	}, nil
}

func (r *Postgres) TopProfiles(ctx context.Context, sinceDate time.Time, limit int) ([]model.TopProfile, error) {
	rows, err := r.q.PlatformOverviewTopProfiles(ctx, db.PlatformOverviewTopProfilesParams{
		EventType:  model.ProfileViewEvent,
		SinceDate:  pgtype.Date{Time: sinceDate, Valid: true},
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]model.TopProfile, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.TopProfile{
			UUID:           row.Uuid,
			Name:           strings.TrimSpace(row.FirstName + " " + row.LastName),
			CompanyName:    row.CompanyName,
			Username:       row.Username,
			Views:          row.Views,
			UniqueSessions: row.UniqueSessions,
		})
	}
	return out, nil
}

func (r *Postgres) EventsByHour(ctx context.Context, since time.Time) ([]model.HourCount, error) {
	rows, err := r.q.PlatformOverviewEventsByHour(ctx, ts(since))
	if err != nil {
		return nil, err
	}
	out := make([]model.HourCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.HourCount{Bucket: row.BucketHour.Time.UTC(), Count: row.Count})
	}
	return out, nil
}

func (r *Postgres) EventsByType(ctx context.Context, since time.Time, limit int) ([]model.EventTypeCount, error) {
	rows, err := r.q.PlatformOverviewEventsByType(ctx, db.PlatformOverviewEventsByTypeParams{
		Since:      ts(since),
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]model.EventTypeCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.EventTypeCount{EventType: row.EventType, Count: row.Count})
	}
	return out, nil
}
