package entitlement

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/jackc/pgx/v5"
)

// Unlimited is the int limit value meaning "no cap". A feature missing from the plan is not
// available (fail-closed); see docs/TENANCY_AND_ACCESS.md#entitlement.
const Unlimited int32 = -1

// ResolvedPlan is the effective plan for an account after subscription resolution.
type ResolvedPlan struct {
	PlanID   int64
	PlanUUID string
	PlanSlug string
	PlanName string
	Source   string // subscription | free
}

// LimitValue is a typed plan limit.
type LimitValue struct {
	Key       string `json:"key"`
	ValueType string `json:"value_type"`
	IntValue  *int32 `json:"int_value,omitempty"`
	BoolValue *bool  `json:"bool_value,omitempty"`
}

// Summary is attached to /v1/auth/me for client upgrade hints.
type Summary struct {
	PlanSlug string       `json:"plan_slug"`
	PlanName string       `json:"plan_name"`
	Status   string       `json:"status"`
	Source   string       `json:"source"`
	Limits   []LimitValue `json:"limits"`
}

// Service resolves account subscriptions and plan limits.
type Service struct {
	q       *db.Queries
	alerter UsageAlerter
	now     func() time.Time
}

// New creates an entitlement service.
func New(q *db.Queries) *Service {
	return &Service{q: q, now: time.Now}
}

// ResolvePlan returns the effective plan of an account: its active subscription, otherwise free.
func (s *Service) ResolvePlan(ctx context.Context, accountID int64) (ResolvedPlan, error) {
	sub, err := s.q.GetActiveSubscriptionByAccountID(ctx, accountID)
	if err == nil {
		return ResolvedPlan{
			PlanID:   sub.PlanID,
			PlanUUID: sub.PlanUuid.String(),
			PlanSlug: sub.PlanSlug,
			PlanName: sub.PlanName,
			Source:   "subscription",
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ResolvedPlan{}, err
	}
	plan, err := s.q.GetPlanBySlug(ctx, FreePlanSlug)
	if err != nil {
		return ResolvedPlan{}, err
	}
	return ResolvedPlan{
		PlanID:   plan.ID,
		PlanUUID: plan.Uuid.String(),
		PlanSlug: plan.Slug,
		PlanName: plan.Name,
		Source:   "free",
	}, nil
}

// ListLimits returns plan limits for a resolved plan id.
func (s *Service) ListLimits(ctx context.Context, planID int64) ([]LimitValue, error) {
	rows, err := s.q.ListPlanLimitsForPlanID(ctx, planID)
	if err != nil {
		return nil, err
	}
	out := make([]LimitValue, 0, len(rows))
	for _, row := range rows {
		lv := LimitValue{Key: row.FeatureKey, ValueType: string(row.ValueType)}
		if row.ValueInt.Valid {
			v := row.ValueInt.Int32
			lv.IntValue = &v
		}
		if row.ValueBool.Valid {
			v := row.ValueBool.Bool
			lv.BoolValue = &v
		}
		out = append(out, lv)
	}
	return out, nil
}

// SummaryForAccount builds the /me subscription snapshot for an account.
func (s *Service) SummaryForAccount(ctx context.Context, accountID int64) (Summary, error) {
	plan, err := s.ResolvePlan(ctx, accountID)
	if err != nil {
		return Summary{}, err
	}
	limits, err := s.ListLimits(ctx, plan.PlanID)
	if err != nil {
		return Summary{}, err
	}
	status := "active"
	if plan.Source == "free" {
		status = "free"
	}
	return Summary{
		PlanSlug: plan.PlanSlug,
		PlanName: plan.PlanName,
		Status:   status,
		Source:   plan.Source,
		Limits:   limits,
	}, nil
}

func (s *Service) limit(ctx context.Context, accountID int64, key string) (*LimitValue, error) {
	plan, err := s.ResolvePlan(ctx, accountID)
	if err != nil {
		return nil, err
	}
	limits, err := s.ListLimits(ctx, plan.PlanID)
	if err != nil {
		return nil, err
	}
	for i := range limits {
		if limits[i].Key == key {
			return &limits[i], nil
		}
	}
	return nil, nil
}

// CheckBool reports whether a bool feature is enabled for the account. A feature missing from the
// plan is disabled. Platform super admins bypass plan limits.
func (s *Service) CheckBool(ctx context.Context, accountID int64, isSuperAdmin bool, key string) (bool, error) {
	if isSuperAdmin {
		return true, nil
	}
	lim, err := s.limit(ctx, accountID, key)
	if err != nil || lim == nil || lim.BoolValue == nil {
		return false, err
	}
	return *lim.BoolValue, nil
}

// CheckIntUsage returns ErrPlanLimitExceeded unless current usage is below the account's limit.
// A feature missing from the plan allows nothing; Unlimited (-1) allows everything.
func (s *Service) CheckIntUsage(ctx context.Context, accountID int64, isSuperAdmin bool, key string, current int32) error {
	if isSuperAdmin {
		return nil
	}
	lim, err := s.limit(ctx, accountID, key)
	if err != nil {
		return err
	}
	return checkIntLimit(lim, int64(current))
}

func checkIntLimit(lim *LimitValue, current int64) error {
	if lim == nil || lim.IntValue == nil {
		return ErrPlanLimitExceeded
	}
	if *lim.IntValue == Unlimited {
		return nil
	}
	if current >= int64(*lim.IntValue) {
		return ErrPlanLimitExceeded
	}
	return nil
}

// usage counts the resource a quota feature caps.
func (s *Service) usage(ctx context.Context, accountID int64, key string) (int64, error) {
	switch key {
	case FeatureProfilesMax:
		return s.q.CountAccountProfilesUsage(ctx, accountID)
	case FeatureSmartLinksMax:
		return s.q.CountAccountSmartLinksUsage(ctx, accountID)
	case FeatureCardsMax:
		return s.q.CountAccountActiveCardsUsage(ctx, accountID)
	default:
		return 0, fmt.Errorf("entitlement: %s is not a quota feature", key)
	}
}

// CheckQuota returns a *QuotaError (errors.Is ErrPlanLimitExceeded) unless the account may create
// one more resource capped by key. It counts current usage itself, so callers only name the
// feature. Platform super admins bypass plan limits.
//
// Count-then-insert is not serialised: two concurrent creates can overshoot by one. That is an
// accepted trade-off for plan quotas, not a security boundary.
func (s *Service) CheckQuota(ctx context.Context, accountID int64, isSuperAdmin bool, key string) error {
	if isSuperAdmin {
		return nil
	}
	used, err := s.usage(ctx, accountID, key)
	if err != nil {
		return err
	}
	lim, err := s.limit(ctx, accountID, key)
	if err != nil {
		return err
	}
	if err := checkIntLimit(lim, used); err != nil {
		qe := &QuotaError{Feature: key, Used: used}
		if lim != nil && lim.IntValue != nil {
			qe.Limit = *lim.IntValue
		}
		return qe
	}
	return nil
}

// HistoryWindow returns how many days back the account may query analytics; unlimited is true
// for -1 or for super admins. A plan without the feature gets 0 days (today only).
func (s *Service) HistoryWindow(ctx context.Context, accountID int64, isSuperAdmin bool) (days int32, unlimited bool, err error) {
	if isSuperAdmin {
		return 0, true, nil
	}
	lim, err := s.limit(ctx, accountID, FeatureAnalyticsHistoryDays)
	if err != nil {
		return 0, false, err
	}
	if lim == nil || lim.IntValue == nil {
		return 0, false, nil
	}
	if *lim.IntValue == Unlimited {
		return 0, true, nil
	}
	return *lim.IntValue, false, nil
}

// FeatureUsage is one row of the account entitlement overview.
type FeatureUsage struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Description *string `json:"description,omitempty"`
	ValueType   string  `json:"value_type"`
	// Included is false when the plan does not define the feature (fail-closed: unavailable).
	Included  bool   `json:"included"`
	IntValue  *int32 `json:"int_value,omitempty"`
	BoolValue *bool  `json:"bool_value,omitempty"`
	Unlimited bool   `json:"unlimited"`
	// Used is set for quota features (cards, profiles, smart links) and, for monthly features,
	// counts this month's use (locked leads included).
	Used *int64 `json:"used,omitempty"`
	// Reached is true when no more quota-capped resources can be created (monthly: further
	// leads arrive locked, exports are refused).
	Reached bool `json:"reached"`
	// Monthly marks a quota that resets at ResetsAt (start of next month, Europe/Istanbul).
	Monthly  bool       `json:"monthly"`
	ResetsAt *time.Time `json:"resets_at,omitempty"`
}

// Overview is the active account's plan with every catalog feature and its usage.
type Overview struct {
	PlanSlug string         `json:"plan_slug"`
	PlanName string         `json:"plan_name"`
	Status   string         `json:"status"`
	Source   string         `json:"source"`
	Features []FeatureUsage `json:"features"`
}

// OverviewForAccount builds the plan/usage overview shown in the app (limits and upgrade hints).
func (s *Service) OverviewForAccount(ctx context.Context, accountID int64) (Overview, error) {
	summary, err := s.SummaryForAccount(ctx, accountID)
	if err != nil {
		return Overview{}, err
	}
	defs, err := s.q.ListFeatureDefinitions(ctx)
	if err != nil {
		return Overview{}, err
	}
	labels := make(map[string]db.FeatureDefinition, len(defs))
	for _, d := range defs {
		labels[d.Key] = d
	}
	byKey := make(map[string]LimitValue, len(summary.Limits))
	for _, l := range summary.Limits {
		byKey[l.Key] = l
	}
	out := Overview{
		PlanSlug: summary.PlanSlug,
		PlanName: summary.PlanName,
		Status:   summary.Status,
		Source:   summary.Source,
		Features: make([]FeatureUsage, 0, len(Catalog())),
	}
	for _, f := range Catalog() {
		row := FeatureUsage{Key: f.Key, Label: f.Key, ValueType: f.ValueType}
		if d, ok := labels[f.Key]; ok {
			row.Label = d.Label
			if d.Description.Valid {
				desc := d.Description.String
				row.Description = &desc
			}
		}
		if l, ok := byKey[f.Key]; ok {
			row.Included = l.IntValue != nil || l.BoolValue != nil
			row.IntValue, row.BoolValue = l.IntValue, l.BoolValue
			row.Unlimited = l.IntValue != nil && *l.IntValue == Unlimited
		}
		if f.Quota {
			used, err := s.usage(ctx, accountID, f.Key)
			if err != nil {
				return Overview{}, err
			}
			row.Used = &used
			row.Reached = !row.Unlimited && (row.IntValue == nil || used >= int64(*row.IntValue))
		}
		if f.Monthly {
			l := byKey[f.Key]
			u, err := s.monthlyUsage(ctx, accountID, f.Key, &l)
			if err != nil {
				return Overview{}, err
			}
			row.Monthly, row.ResetsAt, row.Used = true, &u.ResetsAt, &u.Used
			row.Reached = u.Off() || u.Reached()
		}
		out.Features = append(out.Features, row)
	}
	return out, nil
}
