package entitlement

import (
	"errors"
	"fmt"
)

var ErrPlanLimitExceeded = errors.New("plan limit exceeded")

// QuotaError is returned when a quota-guarded create would exceed the account's plan.
// errors.Is(err, ErrPlanLimitExceeded) holds for it.
type QuotaError struct {
	Feature string
	Limit   int32 // 0 when the plan does not include the feature
	Used    int64
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("plan limit exceeded: %s (%d/%d)", e.Feature, e.Used, e.Limit)
}

func (e *QuotaError) Is(target error) bool { return target == ErrPlanLimitExceeded }

// FeatureError is returned when the account's plan does not include a bool feature.
// errors.Is(err, ErrPlanLimitExceeded) holds for it.
type FeatureError struct {
	Feature string
}

func (e *FeatureError) Error() string {
	return fmt.Sprintf("plan limit exceeded: %s is not included in the plan", e.Feature)
}

func (e *FeatureError) Is(target error) bool { return target == ErrPlanLimitExceeded }
