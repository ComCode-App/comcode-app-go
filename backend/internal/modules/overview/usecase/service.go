// Package usecase assembles the platform overview (TEC-594) and caches it briefly.
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/repository"
)

// CacheTTL is how long a computed overview is served from Redis.
const CacheTTL = 60 * time.Second

// Service builds the overview from the repository.
type Service struct {
	repo  repository.Repository
	cache Cache
	now   func() time.Time
}

// New creates the service. cache may be nil, or a nil *RedisCache: both always compute.
func New(repo repository.Repository, cache Cache) *Service {
	return &Service{repo: repo, cache: cache, now: time.Now}
}

// SetClock overrides the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// ErrInvalidPeriod is returned for a funnel period outside model.FunnelPeriods.
var ErrInvalidPeriod = errors.New("invalid funnel period")

// Get returns the cached overview when fresh, else computes and caches it.
func (s *Service) Get(ctx context.Context) (model.Overview, error) {
	return cached(ctx, s.cache, overviewCacheKey, s.Compute)
}

// Funnel returns the conversion funnel of the orders opened in the last `days` days, cached like
// the overview. days must be one of model.FunnelPeriods.
func (s *Service) Funnel(ctx context.Context, days int) (model.ConversionFunnel, error) {
	if !slices.Contains(model.FunnelPeriods, days) {
		return model.ConversionFunnel{}, ErrInvalidPeriod
	}
	return cached(ctx, s.cache, fmt.Sprintf(funnelCacheKey, days), func(ctx context.Context) (model.ConversionFunnel, error) {
		return s.ComputeFunnel(ctx, days)
	})
}

// cached serves key from the cache when fresh, else computes and stores it.
func cached[T any](ctx context.Context, cache Cache, key string, compute func(context.Context) (T, error)) (T, error) {
	if cache != nil {
		if b, ok := cache.Get(ctx, key); ok {
			var out T
			if err := json.Unmarshal(b, &out); err == nil {
				return out, nil
			}
		}
	}
	out, err := compute(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	if cache != nil {
		if b, err := json.Marshal(out); err == nil {
			cache.Set(ctx, key, b)
		}
	}
	return out, nil
}

// ComputeFunnel reads the funnel of the last `days` days from the repository (no cache).
func (s *Service) ComputeFunnel(ctx context.Context, days int) (model.ConversionFunnel, error) {
	now := s.now().UTC()
	since := now.Add(-time.Duration(days) * 24 * time.Hour)
	counts, err := s.repo.ConversionFunnel(ctx, since, now)
	if err != nil {
		return model.ConversionFunnel{}, fmt.Errorf("overview funnel: %w", err)
	}
	out := conversionFunnel(counts)
	out.GeneratedAt, out.PeriodDays, out.From, out.To = now, days, since, now
	return out, nil
}

// funnelStages lists the stages after "created" in order.
var funnelStages = []string{model.StagePaid, model.StageInProduction, model.StageShipped, model.StageDelivered}

// conversionFunnel derives the stage-to-stage and overall conversion rates and the average hours
// from creation to each stage.
func conversionFunnel(c repository.FunnelCounts) model.ConversionFunnel {
	out := model.ConversionFunnel{Cancelled: c.Cancelled, Stages: make([]model.FunnelStage, 0, len(funnelStages)+1)}
	created := model.FunnelStage{Stage: model.StageCreated, Orders: c.Created}
	if c.Created > 0 {
		created.ConversionFromPrevious, created.ConversionFromCreated = 1, 1
	}
	out.Stages = append(out.Stages, created)
	prev := c.Created
	for _, name := range funnelStages {
		sc := c.Stages[name]
		st := model.FunnelStage{
			Stage:                  name,
			Orders:                 sc.Reached,
			ConversionFromPrevious: ratio(sc.Reached, prev),
			ConversionFromCreated:  ratio(sc.Reached, c.Created),
			TimedOrders:            sc.Timed,
		}
		if sc.Timed > 0 {
			h := sc.Seconds / float64(sc.Timed) / 3600
			st.AvgHoursFromCreated = &h
		}
		out.Stages = append(out.Stages, st)
		prev = sc.Reached
	}
	return out
}

func ratio(n, d int64) float64 {
	if d <= 0 {
		return 0
	}
	return float64(n) / float64(d)
}

// Compute reads every KPI from the repository (no cache).
func (s *Service) Compute(ctx context.Context) (model.Overview, error) {
	now := s.now().UTC()
	since := now.Add(-model.WindowDays * 24 * time.Hour)
	// Daily aggregates are bucketed by UTC day: today plus the 29 days before it.
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	sinceDate := day.AddDate(0, 0, -(model.WindowDays - 1))
	// The current (partial) hour and the 23 before it.
	firstHour := now.Truncate(time.Hour).Add(-23 * time.Hour)

	out := model.Overview{GeneratedAt: now, WindowDays: model.WindowDays}
	var err error
	if out.Users, err = s.repo.Users(ctx, since); err != nil {
		return out, fmt.Errorf("overview users: %w", err)
	}
	if out.Profiles, err = s.repo.Profiles(ctx); err != nil {
		return out, fmt.Errorf("overview profiles: %w", err)
	}
	cards, err := s.repo.Cards(ctx)
	if err != nil {
		return out, fmt.Errorf("overview cards: %w", err)
	}
	out.Cards = cardStats(cards)
	orders, err := s.repo.OrderStatusCounts(ctx, since)
	if err != nil {
		return out, fmt.Errorf("overview orders: %w", err)
	}
	out.Orders = orderFunnel(orders)
	if out.Subscriptions, err = s.repo.Subscriptions(ctx, since); err != nil {
		return out, fmt.Errorf("overview subscriptions: %w", err)
	}
	mrr, err := s.repo.MRR(ctx)
	if err != nil {
		return out, fmt.Errorf("overview mrr: %w", err)
	}
	out.Subscriptions.MRR, out.Subscriptions.AtRiskMRR = mrr.Active, mrr.AtRisk
	if out.TopProfiles, err = s.repo.TopProfiles(ctx, sinceDate, model.TopProfilesLimit); err != nil {
		return out, fmt.Errorf("overview top profiles: %w", err)
	}
	hours, err := s.repo.EventsByHour(ctx, firstHour)
	if err != nil {
		return out, fmt.Errorf("overview events by hour: %w", err)
	}
	out.Events24h.Total, out.Events24h.Hourly = hourly(firstHour, hours)
	if out.Events24h.ByType, err = s.repo.EventsByType(ctx, firstHour, model.EventTypesLimit); err != nil {
		return out, fmt.Errorf("overview events by type: %w", err)
	}
	if out.TopProfiles == nil {
		out.TopProfiles = []model.TopProfile{}
	}
	if out.Subscriptions.MRR == nil {
		out.Subscriptions.MRR = []model.MRR{}
	}
	if out.Subscriptions.AtRiskMRR == nil {
		out.Subscriptions.AtRiskMRR = []model.MRR{}
	}
	if out.Events24h.ByType == nil {
		out.Events24h.ByType = []model.EventTypeCount{}
	}
	return out, nil
}

func cardStats(c repository.CardCounts) model.CardStats {
	out := model.CardStats{Total: c.Total, Activated: c.Activated}
	if c.Total > 0 {
		out.ActivationRate = float64(c.Activated) / float64(c.Total)
	}
	return out
}

// orderFunnel lists every known status in lifecycle order; unknown statuses still count in the
// total and are appended after the known ones.
func orderFunnel(counts map[string]int64) model.OrderFunnel {
	out := model.OrderFunnel{Funnel: make([]model.StatusCount, 0, len(model.OrderStatuses))}
	known := make(map[string]bool, len(model.OrderStatuses))
	for _, st := range model.OrderStatuses {
		known[st] = true
		out.Funnel = append(out.Funnel, model.StatusCount{Status: st, Count: counts[st]})
		out.Total += counts[st]
	}
	extra := make([]string, 0)
	for st := range counts {
		if !known[st] {
			extra = append(extra, st)
		}
	}
	slices.Sort(extra)
	for _, st := range extra {
		out.Funnel = append(out.Funnel, model.StatusCount{Status: st, Count: counts[st]})
		out.Total += counts[st]
	}
	return out
}

// hourly zero-fills the 24 buckets starting at firstHour.
func hourly(firstHour time.Time, rows []model.HourCount) (int64, []model.HourCount) {
	byHour := make(map[int64]int64, len(rows))
	for _, r := range rows {
		byHour[r.Bucket.Truncate(time.Hour).Unix()] += r.Count
	}
	var total int64
	out := make([]model.HourCount, 24)
	for i := range out {
		b := firstHour.Add(time.Duration(i) * time.Hour)
		c := byHour[b.Unix()]
		out[i] = model.HourCount{Bucket: b, Count: c}
		total += c
	}
	return total, out
}
