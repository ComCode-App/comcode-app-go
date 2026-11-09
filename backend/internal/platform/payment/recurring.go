package payment

import (
	"context"
	"errors"
	"strings"
)

// ErrRecurringUnsupported requires a customer-initiated payment instead of an automatic charge.
var ErrRecurringUnsupported = errors.New("provider does not support automatic renewal in this integration")

// RecurringCharger must return an error for an indeterminate outcome. A nil error and
// Success=false means a definitive decline; callers may schedule a new attempt only then.
type RecurringCharger interface {
	ChargeRecurring(context.Context, string, SavedCardChargeInput) (CallbackResult, error)
}

func (r *Registry) ChargeRecurring(ctx context.Context, slug string, in SavedCardChargeInput) (CallbackResult, error) {
	// PayTR's wait_callback is not represented by CallbackResult. Never interpret it
	// as a decline or retry it. Enabling it needs a verified callback settlement path.
	if slug != "iyzico" {
		return CallbackResult{}, ErrRecurringUnsupported
	}
	cfg, err := r.reader.ProviderConfig(ctx, slug)
	if err != nil {
		return CallbackResult{}, err
	}
	if !cfg.Enabled {
		return CallbackResult{}, ErrNotConfigured
	}
	p, err := r.providerFromConfig(cfg)
	if err != nil {
		return CallbackResult{}, err
	}
	mode, err := r.TestMode(ctx)
	if err != nil {
		return CallbackResult{}, err
	}
	in.TestMode = mode
	result, err := p.ChargeSavedCard(ctx, in)
	if err != nil {
		return CallbackResult{}, err
	}
	if result.Success && strings.TrimSpace(result.ProviderRef) == "" {
		return CallbackResult{}, ErrUnavailable
	}
	return result, nil
}
