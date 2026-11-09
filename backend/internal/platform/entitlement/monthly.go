package entitlement

import (
	"context"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// Monthly quotas (TEC-941): int features counted per account and calendar month in
// Europe/Istanbul (account_monthly_usage). A new month starts a new counter row, which is the
// reset. -1 is unlimited, 0 turns the feature off, and a missing value is off (fail-closed).

var istanbul = loadIstanbul()

func loadIstanbul() *time.Location {
	if loc, err := time.LoadLocation("Europe/Istanbul"); err == nil {
		return loc
	}
	// Turkey has kept UTC+3 all year since 2016; used when the zone database is missing.
	return time.FixedZone("TRT", 3*60*60)
}

// MonthStart is the start of t's calendar month in Europe/Istanbul.
func MonthStart(t time.Time) time.Time {
	t = t.In(istanbul)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, istanbul)
}

// MonthlyUsage is one monthly quota of an account in the current month.
type MonthlyUsage struct {
	Key       string
	Limit     int32 // ignored when Unlimited
	Unlimited bool
	// Used counts the month's events (leads received, exports run), locked leads included.
	Used int64
	// Unlocked counts the month's leads the owner may see (at most Limit).
	Unlocked int64
	Period   time.Time
	ResetsAt time.Time
}

// Off reports a feature the plan turns off (0, or no value).
func (m MonthlyUsage) Off() bool { return !m.Unlimited && m.Limit <= 0 }

// Reached reports that the month's quota is used up.
func (m MonthlyUsage) Reached() bool { return !m.Unlimited && m.Used >= int64(m.Limit) }

// UnlockAllowance is how many more of the month's locked leads the plan lets the owner see; -1
// means all of them.
func (m MonthlyUsage) UnlockAllowance() int64 {
	if m.Unlimited {
		return -1
	}
	return max(int64(m.Limit)-m.Unlocked, 0)
}

// QuotaAlert tells the account that a monthly quota reached 80% or 100% of its limit.
type QuotaAlert struct {
	AccountID int64
	Feature   string
	Used      int64
	Limit     int32
	Percent   int
	ResetsAt  time.Time
}

// UsageAlerter delivers quota alerts to the account's owners and admins. It must not fail the
// caller: the event that crossed the threshold already happened.
type UsageAlerter interface {
	QuotaAlert(ctx context.Context, alert QuotaAlert)
}

// SetUsageAlerter turns on the 80% / 100% alerts of monthly quotas.
func (s *Service) SetUsageAlerter(a UsageAlerter) { s.alerter = a }

// MonthlyUsage returns the account's plan limit and this month's counters for a monthly feature.
func (s *Service) MonthlyUsage(ctx context.Context, accountID int64, key string) (MonthlyUsage, error) {
	lim, err := s.limit(ctx, accountID, key)
	if err != nil {
		return MonthlyUsage{}, err
	}
	return s.monthlyUsage(ctx, accountID, key, lim)
}

func (s *Service) monthlyUsage(ctx context.Context, accountID int64, key string, lim *LimitValue) (MonthlyUsage, error) {
	start := MonthStart(s.now())
	u := MonthlyUsage{Key: key, Period: start, ResetsAt: start.AddDate(0, 1, 0)}
	if lim != nil && lim.IntValue != nil {
		u.Limit = *lim.IntValue
		u.Unlimited = u.Limit == Unlimited
	}
	row, err := s.q.GetMonthlyUsage(ctx, db.GetMonthlyUsageParams{
		AccountID: accountID, FeatureKey: key, Period: period(start),
	})
	if err != nil {
		return MonthlyUsage{}, err
	}
	u.Used, u.Unlocked = row.Used, row.Unlocked
	return u, nil
}

// RecordMonthly counts one lead or exchange of the account this month and reports whether it is
// within the quota (granted an unlocked slot); one over the quota is stored locked. An Off
// feature counts nothing and grants nothing: the caller must not store the event.
func (s *Service) RecordMonthly(ctx context.Context, accountID int64, key string) (MonthlyUsage, bool, error) {
	u, err := s.MonthlyUsage(ctx, accountID, key)
	if err != nil || u.Off() {
		return u, false, err
	}
	p := period(u.Period)
	u.Used, err = s.q.IncrementMonthlyUsage(ctx, db.IncrementMonthlyUsageParams{AccountID: accountID, FeatureKey: key, Period: p})
	if err != nil {
		return u, false, err
	}
	limit := u.Limit
	if u.Unlimited {
		limit = Unlimited
	}
	n, err := s.q.GrantMonthlyUnlock(ctx, db.GrantMonthlyUnlockParams{
		AccountID: accountID, FeatureKey: key, Period: p, LimitValue: limit,
	})
	if err != nil {
		return u, false, err
	}
	if n > 0 {
		u.Unlocked++
	}
	s.alert(ctx, accountID, u)
	return u, n > 0, nil
}

// ReserveMonthly counts one export-like use of a monthly feature, or returns a *QuotaError when
// the feature is off or this month's quota is used up. Platform super admins bypass it and
// count nothing. Concurrent calls cannot overshoot: an increment past the limit is undone.
func (s *Service) ReserveMonthly(ctx context.Context, accountID int64, isSuperAdmin bool, key string) (MonthlyUsage, error) {
	if isSuperAdmin {
		return MonthlyUsage{Key: key, Unlimited: true}, nil
	}
	u, err := s.MonthlyUsage(ctx, accountID, key)
	if err != nil {
		return u, err
	}
	if u.Off() || u.Reached() {
		return u, &QuotaError{Feature: key, Limit: max(u.Limit, 0), Used: u.Used}
	}
	p := period(u.Period)
	used, err := s.q.IncrementMonthlyUsage(ctx, db.IncrementMonthlyUsageParams{AccountID: accountID, FeatureKey: key, Period: p})
	if err != nil {
		return u, err
	}
	if !u.Unlimited && used > int64(u.Limit) {
		if err := s.q.DecrementMonthlyUsage(ctx, db.DecrementMonthlyUsageParams{AccountID: accountID, FeatureKey: key, Period: p}); err != nil {
			return u, err
		}
		return u, &QuotaError{Feature: key, Limit: u.Limit, Used: int64(u.Limit)}
	}
	u.Used = used
	s.alert(ctx, accountID, u)
	return u, nil
}

// AddMonthlyUnlocked records n leads of this month unlocked after a plan upgrade.
func (s *Service) AddMonthlyUnlocked(ctx context.Context, accountID int64, key string, n int64) error {
	if n <= 0 {
		return nil
	}
	return s.q.AddMonthlyUnlocked(ctx, db.AddMonthlyUnlockedParams{
		AccountID: accountID, FeatureKey: key, Period: period(MonthStart(s.now())), N: int32(n),
	})
}

func (s *Service) alert(ctx context.Context, accountID int64, u MonthlyUsage) {
	if s.alerter == nil || u.Unlimited {
		return
	}
	if pct := alertPercent(u.Used, u.Limit); pct > 0 {
		s.alerter.QuotaAlert(ctx, QuotaAlert{
			AccountID: accountID, Feature: u.Key, Used: u.Used, Limit: u.Limit, Percent: pct, ResetsAt: u.ResetsAt,
		})
	}
}

// alertPercent is 80 when used just reached 80% of the limit (rounded up) and 100 when it just
// reached the limit; 0 otherwise. Counters move by one, so each threshold fires once a month.
// A limit too small for a separate 80% step only alerts at 100%.
func alertPercent(used int64, limit int32) int {
	if limit <= 0 {
		return 0
	}
	if used == int64(limit) {
		return 100
	}
	if warn := (int64(limit)*80 + 99) / 100; warn < int64(limit) && used == warn {
		return 80
	}
	return 0
}

func period(start time.Time) pgtype.Date {
	return pgtype.Date{Time: time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC), Valid: true}
}
