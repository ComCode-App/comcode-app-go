package entitlement

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Quotas is the dependency modules take to guard quota-capped creates (*Service implements it).
type Quotas interface {
	CheckQuota(ctx context.Context, accountID int64, isSuperAdmin bool, key string) error
}

var errQuotasNotConfigured = errors.New("entitlement: quota checker not configured")

// Guard checks key for the account on behalf of the principal in ctx. A nil checker denies:
// a module wired without entitlement must not create unlimited resources.
func Guard(ctx context.Context, q Quotas, accountID int64, key string) error {
	if q == nil {
		return errQuotasNotConfigured
	}
	p, _ := authctx.PrincipalFrom(ctx)
	return q.CheckQuota(ctx, accountID, p.IsSuperAdmin, key)
}

// Features is the dependency modules take to check bool plan features (*Service implements it).
type Features interface {
	CheckBool(ctx context.Context, accountID int64, isSuperAdmin bool, key string) (bool, error)
}

// RequireFeature returns a *FeatureError unless the account's plan enables the bool feature key,
// on behalf of the principal in ctx (super admins bypass). A nil checker denies (fail-closed).
func RequireFeature(ctx context.Context, f Features, accountID int64, key string) error {
	if f == nil {
		return &FeatureError{Feature: key}
	}
	p, _ := authctx.PrincipalFrom(ctx)
	ok, err := f.CheckBool(ctx, accountID, p.IsSuperAdmin, key)
	if err != nil {
		return err
	}
	if !ok {
		return &FeatureError{Feature: key}
	}
	return nil
}

// WriteError writes 403 PLAN_LIMIT_EXCEEDED for plan-limit errors and reports whether it did.
// The detail names the feature so the client can show the right upgrade hint.
func WriteError(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, ErrPlanLimitExceeded) {
		return false
	}
	var qe *QuotaError
	if errors.As(err, &qe) {
		response.ErrorWithDetails(w, r, http.StatusForbidden, response.CodePlanLimitExceeded,
			"Your plan does not allow more of this resource. Upgrade to continue.",
			[]response.Detail{{
				Field:   qe.Feature,
				Code:    response.CodePlanLimitExceeded,
				Message: fmt.Sprintf("limit %d, used %d", qe.Limit, qe.Used),
			}})
		return true
	}
	var fe *FeatureError
	if errors.As(err, &fe) {
		response.ErrorWithDetails(w, r, http.StatusForbidden, response.CodePlanLimitExceeded,
			"Your plan does not include this feature. Upgrade to continue.",
			[]response.Detail{{
				Field:   fe.Feature,
				Code:    response.CodePlanLimitExceeded,
				Message: "not included in the plan",
			}})
		return true
	}
	response.PlanLimitExceeded(w, r, "")
	return true
}
