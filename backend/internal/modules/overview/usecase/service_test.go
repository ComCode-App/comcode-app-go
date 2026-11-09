package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/repository"
)

type fakeRepo struct {
	calls     int
	err       error
	since     time.Time
	sinceDate time.Time
	firstHour time.Time
	hours     []model.HourCount

	funnel      repository.FunnelCounts
	funnelCalls int
	until       time.Time
}

func (f *fakeRepo) Users(_ context.Context, since time.Time) (model.UserStats, error) {
	f.calls++
	f.since = since
	return model.UserStats{Total: 10, ActiveLast30d: 4, NewLast30d: 2}, f.err
}

func (f *fakeRepo) Profiles(context.Context) (model.ProfileStats, error) {
	return model.ProfileStats{Total: 8, Published: 5}, nil
}

func (f *fakeRepo) Cards(context.Context) (repository.CardCounts, error) {
	return repository.CardCounts{Total: 4, Activated: 3}, nil
}

func (f *fakeRepo) OrderStatusCounts(context.Context, time.Time) (map[string]int64, error) {
	return map[string]int64{"paid": 2, "pending": 1, "legacy_state": 1}, nil
}

func (f *fakeRepo) Subscriptions(context.Context, time.Time) (model.SubscriptionStats, error) {
	return model.SubscriptionStats{Active: 3, Trialing: 1, Canceled: 2, CanceledLast30d: 1}, nil
}

func (f *fakeRepo) MRR(context.Context) (repository.MRRSplit, error) {
	return repository.MRRSplit{
		Active: []model.MRR{{Currency: "TRY", AmountMinor: 183150, Subscriptions: 3}},
		AtRisk: []model.MRR{{Currency: "TRY", AmountMinor: 99900, Subscriptions: 1}},
	}, nil
}

func (f *fakeRepo) ConversionFunnel(_ context.Context, since, until time.Time) (repository.FunnelCounts, error) {
	f.funnelCalls++
	f.since, f.until = since, until
	return f.funnel, f.err
}

func (f *fakeRepo) TopProfiles(_ context.Context, sinceDate time.Time, _ int) ([]model.TopProfile, error) {
	f.sinceDate = sinceDate
	return nil, nil
}

func (f *fakeRepo) EventsByHour(_ context.Context, since time.Time) ([]model.HourCount, error) {
	f.firstHour = since
	return f.hours, nil
}

func (f *fakeRepo) EventsByType(context.Context, time.Time, int) ([]model.EventTypeCount, error) {
	return []model.EventTypeCount{{EventType: "card.view", Count: 7}}, nil
}

var fixedNow = time.Date(2026, 10, 19, 14, 37, 12, 0, time.UTC)

func TestComputeShapesKPIs(t *testing.T) {
	repo := &fakeRepo{hours: []model.HourCount{
		{Bucket: time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC), Count: 3}, // first bucket
		{Bucket: time.Date(2026, 10, 19, 14, 0, 0, 0, time.UTC), Count: 4}, // current hour
	}}
	s := New(repo, nil)
	s.SetClock(func() time.Time { return fixedNow })

	out, err := s.Compute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := fixedNow.Add(-30 * 24 * time.Hour); !repo.since.Equal(want) {
		t.Fatalf("since = %v, want %v", repo.since, want)
	}
	if want := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC); !repo.sinceDate.Equal(want) {
		t.Fatalf("since date = %v, want %v (30 UTC days incl. today)", repo.sinceDate, want)
	}
	if want := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC); !repo.firstHour.Equal(want) {
		t.Fatalf("first hour = %v, want %v", repo.firstHour, want)
	}
	if out.Cards.ActivationRate != 0.75 {
		t.Fatalf("activation rate = %v", out.Cards.ActivationRate)
	}
	if out.Orders.Total != 4 || len(out.Orders.Funnel) != len(model.OrderStatuses)+1 {
		t.Fatalf("funnel = %+v", out.Orders)
	}
	if out.Orders.Funnel[0] != (model.StatusCount{Status: "pending", Count: 1}) ||
		out.Orders.Funnel[1] != (model.StatusCount{Status: "paid", Count: 2}) ||
		out.Orders.Funnel[len(out.Orders.Funnel)-1].Status != "legacy_state" {
		t.Fatalf("funnel order = %+v", out.Orders.Funnel)
	}
	if len(out.Events24h.Hourly) != 24 || out.Events24h.Total != 7 ||
		out.Events24h.Hourly[0].Count != 3 || out.Events24h.Hourly[23].Count != 4 {
		t.Fatalf("hourly = %+v (total %d)", out.Events24h.Hourly, out.Events24h.Total)
	}
	if out.TopProfiles == nil || out.Subscriptions.MRR[0].AmountMinor != 183150 {
		t.Fatalf("top profiles must be [] and MRR kept: %+v", out)
	}
	if len(out.Subscriptions.AtRiskMRR) != 1 || out.Subscriptions.AtRiskMRR[0].AmountMinor != 99900 {
		t.Fatalf("at-risk MRR = %+v", out.Subscriptions.AtRiskMRR)
	}
}

func TestCardStatsWithoutCards(t *testing.T) {
	if got := cardStats(repository.CardCounts{}); got.ActivationRate != 0 {
		t.Fatalf("rate without cards = %v", got.ActivationRate)
	}
}

func TestGetCachesForTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	cache := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test", CacheTTL)
	repo := &fakeRepo{}
	s := New(repo, cache)
	s.SetClock(func() time.Time { return fixedNow })
	ctx := context.Background()

	first, err := s.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 {
		t.Fatalf("repository hit %d times, want 1 (second read from cache)", repo.calls)
	}
	if !second.GeneratedAt.Equal(first.GeneratedAt) || second.Users != first.Users {
		t.Fatalf("cached overview differs: %+v vs %+v", second, first)
	}
	if ttl := mr.TTL("app:test:platform:overview:v2"); ttl != CacheTTL {
		t.Fatalf("ttl = %v", ttl)
	}

	mr.FastForward(CacheTTL + time.Second)
	if _, err := s.Get(ctx); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 2 {
		t.Fatalf("expired entry not recomputed: %d calls", repo.calls)
	}
}

func TestGetWithRedisDownComputes(t *testing.T) {
	mr := miniredis.RunT(t)
	cache := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test", CacheTTL)
	mr.Close()
	repo := &fakeRepo{}
	s := New(repo, cache)
	for range 2 {
		if _, err := s.Get(context.Background()); err != nil {
			t.Fatalf("redis down must not fail the request: %v", err)
		}
	}
	if repo.calls != 2 {
		t.Fatalf("calls = %d", repo.calls)
	}
}

func TestGetWithoutCache(t *testing.T) {
	repo := &fakeRepo{}
	for _, s := range []*Service{New(repo, nil), New(repo, NewRedisCache(nil, "test", CacheTTL))} {
		if _, err := s.Get(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if repo.calls != 2 {
		t.Fatalf("calls = %d", repo.calls)
	}
}

func TestGetPropagatesRepositoryErrors(t *testing.T) {
	repo := &fakeRepo{err: errors.New("boom")}
	if _, err := New(repo, nil).Get(context.Background()); err == nil {
		t.Fatal("want error")
	}
}

func TestConversionFunnelMath(t *testing.T) {
	got := conversionFunnel(repository.FunnelCounts{
		Created:   10,
		Cancelled: 2,
		Stages: map[string]repository.StageCounts{
			// 8 paid, 6 of them with a recorded transition: 3h, 3h, 6h, 6h, 9h, 9h → 6h on average.
			model.StagePaid:         {Reached: 8, Timed: 6, Seconds: 36 * 3600},
			model.StageInProduction: {Reached: 6, Timed: 4, Seconds: 4 * 24 * 3600},
			model.StageShipped:      {Reached: 3, Timed: 3, Seconds: 3 * 72 * 3600},
			// Delivered only through backfilled rows: reached but untimed.
			model.StageDelivered: {Reached: 0},
		},
	})
	if got.Cancelled != 2 || len(got.Stages) != 5 {
		t.Fatalf("funnel = %+v", got)
	}
	type want struct {
		stage          string
		orders         int64
		fromPrev, from float64
		avgHours       float64 // -1: null
	}
	for i, w := range []want{
		{model.StageCreated, 10, 1, 1, -1},
		{model.StagePaid, 8, 0.8, 0.8, 6},
		{model.StageInProduction, 6, 0.75, 0.6, 24},
		{model.StageShipped, 3, 0.5, 0.3, 72},
		{model.StageDelivered, 0, 0, 0, -1},
	} {
		st := got.Stages[i]
		if st.Stage != w.stage || st.Orders != w.orders || st.ConversionFromPrevious != w.fromPrev || st.ConversionFromCreated != w.from {
			t.Fatalf("stage %d = %+v, want %+v", i, st, w)
		}
		if w.avgHours < 0 {
			if st.AvgHoursFromCreated != nil {
				t.Fatalf("stage %s avg = %v, want null", st.Stage, *st.AvgHoursFromCreated)
			}
		} else if st.AvgHoursFromCreated == nil || *st.AvgHoursFromCreated != w.avgHours {
			t.Fatalf("stage %s avg = %v, want %v", st.Stage, st.AvgHoursFromCreated, w.avgHours)
		}
	}
}

func TestConversionFunnelWithoutOrders(t *testing.T) {
	got := conversionFunnel(repository.FunnelCounts{})
	for _, st := range got.Stages {
		if st.Orders != 0 || st.ConversionFromPrevious != 0 || st.ConversionFromCreated != 0 || st.AvgHoursFromCreated != nil {
			t.Fatalf("empty funnel stage = %+v", st)
		}
	}
}

func TestFunnelPeriodAndCache(t *testing.T) {
	mr := miniredis.RunT(t)
	cache := NewRedisCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test", CacheTTL)
	repo := &fakeRepo{funnel: repository.FunnelCounts{Created: 4}}
	s := New(repo, cache)
	s.SetClock(func() time.Time { return fixedNow })
	ctx := context.Background()

	if _, err := s.Funnel(ctx, 14); !errors.Is(err, ErrInvalidPeriod) {
		t.Fatalf("days=14 err = %v, want ErrInvalidPeriod", err)
	}
	out, err := s.Funnel(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !repo.until.Equal(fixedNow) || !repo.since.Equal(fixedNow.Add(-7*24*time.Hour)) {
		t.Fatalf("window = [%v, %v)", repo.since, repo.until)
	}
	if out.PeriodDays != 7 || !out.From.Equal(repo.since) || !out.To.Equal(fixedNow) || out.Stages[0].Orders != 4 {
		t.Fatalf("funnel = %+v", out)
	}
	if _, err := s.Funnel(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Funnel(ctx, 30); err != nil {
		t.Fatal(err)
	}
	if repo.funnelCalls != 2 {
		t.Fatalf("repository hit %d times, want 2 (7 days cached, 30 days computed)", repo.funnelCalls)
	}
	if ttl := mr.TTL("app:test:platform:overview:funnel:v1:7"); ttl != CacheTTL {
		t.Fatalf("ttl = %v", ttl)
	}
}
