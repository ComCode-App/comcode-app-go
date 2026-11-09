package usecase

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
)

// stubQuotas is a plan-quota checker for tests: keys in deny are over their limit.
type stubQuotas struct {
	deny    map[string]bool
	checked []string
}

func (s *stubQuotas) CheckQuota(_ context.Context, _ int64, _ bool, key string) error {
	s.checked = append(s.checked, key)
	if s.deny[key] {
		return &entitlement.QuotaError{Feature: key, Limit: 1, Used: 1}
	}
	return nil
}
