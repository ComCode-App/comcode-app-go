package usecase

import (
	"context"
	"math"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
)

// Monthly quotas (TEC-941). Lead forms and exchanges each have a monthly quota: past it they are
// still stored but locked, and the owner sees them masked. Unlocking is one-way: a new month
// unlocks every earlier locked lead, a plan upgrade the month's oldest ones up to the new quota.
// A quota of 0 turns the form or the exchange off. Exports have their own monthly quota.

// Quotas is the monthly quota dependency (*entitlement.Service implements it).
type Quotas interface {
	MonthlyUsage(ctx context.Context, accountID int64, key string) (entitlement.MonthlyUsage, error)
	RecordMonthly(ctx context.Context, accountID int64, key string) (entitlement.MonthlyUsage, bool, error)
	ReserveMonthly(ctx context.Context, accountID int64, isSuperAdmin bool, key string) (entitlement.MonthlyUsage, error)
	AddMonthlyUnlocked(ctx context.Context, accountID int64, key string, n int64) error
}

// SetQuotas wires the monthly quotas. Without them lead forms and exchanges are off and exports
// are refused (fail-closed).
func (s *Service) SetQuotas(q Quotas) *Service {
	s.quotas = q
	return s
}

// quotaKey is the monthly feature of a lead kind.
func quotaKey(kind string) string {
	if kind == model.KindExchange {
		return entitlement.FeatureExchangeMonthlyMax
	}
	return entitlement.FeatureLeadsMonthlyMax
}

// quotaOn reports whether the account's plan has the form (or exchange) on.
func (s *Service) quotaOn(ctx context.Context, accountID int64, kind string) (bool, error) {
	if s.quotas == nil {
		return false, nil
	}
	u, err := s.quotas.MonthlyUsage(ctx, accountID, quotaKey(kind))
	return err == nil && !u.Off(), err
}

// record counts a new lead of the kind and reports whether it is stored locked. ErrFormUnavailable
// when the plan turns the kind off.
func (s *Service) record(ctx context.Context, accountID int64, kind string) (bool, error) {
	if s.quotas == nil {
		return false, ErrFormUnavailable
	}
	u, granted, err := s.quotas.RecordMonthly(ctx, accountID, quotaKey(kind))
	if err != nil {
		return false, err
	}
	if u.Off() {
		return false, ErrFormUnavailable
	}
	return !granted, nil
}

// unlock applies the unlock rules before the account's leads are read or exported. It is
// fail-soft: on error the leads stay locked (masked) and the read goes on.
func (s *Service) unlock(ctx context.Context, accountID int64) {
	if s.quotas == nil {
		return
	}
	has, err := s.repo.HasLocked(ctx, accountID)
	if err != nil || !has {
		if err != nil {
			s.log.Warn("lead_unlock_failed", "error", err)
		}
		return
	}
	start := entitlement.MonthStart(s.now())
	if _, err := s.repo.UnlockBefore(ctx, accountID, start); err != nil {
		s.log.Warn("lead_unlock_failed", "error", err)
		return
	}
	for _, kind := range model.Kinds {
		key := quotaKey(kind)
		u, err := s.quotas.MonthlyUsage(ctx, accountID, key)
		if err != nil {
			s.log.Warn("lead_unlock_failed", "error", err)
			return
		}
		allowance := u.UnlockAllowance()
		if allowance == 0 {
			continue
		}
		maxRows := int32(math.MaxInt32)
		if allowance > 0 && allowance < math.MaxInt32 {
			maxRows = int32(allowance)
		}
		n, err := s.repo.UnlockMonth(ctx, accountID, kind, start, maxRows)
		if err == nil {
			err = s.quotas.AddMonthlyUnlocked(ctx, accountID, key, n)
		}
		if err != nil {
			s.log.Warn("lead_unlock_failed", "error", err)
			return
		}
	}
}

// PrepareExport takes one lead export from the account's monthly quota (a *entitlement.QuotaError
// when it is off or used up) and unlocks what the plan allows, so the file masks only leads that
// are still locked. Super admins bypass the quota.
func (s *Service) PrepareExport(ctx context.Context, p authctx.Principal) error {
	if s.quotas == nil {
		return &entitlement.QuotaError{Feature: entitlement.FeatureLeadsExportMonthlyMax}
	}
	if _, err := s.quotas.ReserveMonthly(ctx, p.Account.ID, p.IsSuperAdmin, entitlement.FeatureLeadsExportMonthlyMax); err != nil {
		return err
	}
	s.unlock(ctx, p.Account.ID)
	return nil
}
