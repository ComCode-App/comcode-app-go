package usecase

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
)

type entitlementAdapter struct {
	svc *entitlement.Service
}

// NewEntitlementAdapter bridges entitlement.Service to EntitlementGate.
func NewEntitlementAdapter(svc *entitlement.Service) EntitlementGate {
	return entitlementAdapter{svc: svc}
}

func (a entitlementAdapter) SummaryForAccount(ctx context.Context, accountID int64) (MeSubscriptionSummary, error) {
	summary, err := a.svc.SummaryForAccount(ctx, accountID)
	if err != nil {
		return MeSubscriptionSummary{}, err
	}
	limits := make([]MeSubscriptionLimitEntry, 0, len(summary.Limits))
	for _, lim := range summary.Limits {
		limits = append(limits, MeSubscriptionLimitEntry{
			Key: lim.Key, ValueType: lim.ValueType,
			IntValue: lim.IntValue, BoolValue: lim.BoolValue,
		})
	}
	return MeSubscriptionSummary{
		PlanSlug: summary.PlanSlug,
		PlanName: summary.PlanName,
		Status:   summary.Status,
		Source:   summary.Source,
		Limits:   limits,
	}, nil
}
