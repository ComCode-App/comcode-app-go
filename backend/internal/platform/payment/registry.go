package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidRequest = errors.New("invalid payment request")

// Registry resolves the active configured payment provider.
type Registry struct {
	reader SettingsReader
}

func NewRegistry(reader SettingsReader) *Registry {
	return &Registry{reader: reader}
}

func (r *Registry) ActiveProvider(ctx context.Context) (Provider, error) {
	slug, err := r.reader.ActiveProvider(ctx)
	if err != nil {
		Logger().Error("registry_active_provider_read_failed", "error", err)
		return nil, err
	}
	if slug == "" || slug == "none" {
		Logger().Warn("registry_no_active_provider", "slug", slug)
		return nil, fmt.Errorf("%w: no active payment provider", ErrNotConfigured)
	}
	cfg, err := r.reader.ProviderConfig(ctx, slug)
	if err != nil {
		Logger().Error("registry_provider_config_failed", "provider", slug, "error", err)
		return nil, err
	}
	if !cfg.Enabled {
		Logger().Warn("registry_provider_disabled", "provider", slug)
		return nil, fmt.Errorf("%w: provider %s is disabled", ErrNotConfigured, slug)
	}
	provider, err := r.providerFromConfig(cfg)
	if err != nil {
		Logger().Error("registry_provider_build_failed", "provider", slug, "error", err)
		return nil, err
	}
	Logger().Debug("registry_active_provider", "provider", slug, "base_url", cfg.BaseURL)
	return provider, nil
}

func (r *Registry) ProviderBySlug(ctx context.Context, slug string) (Provider, error) {
	cfg, err := r.reader.ProviderConfig(ctx, slug)
	if err != nil {
		Logger().Error("registry_provider_config_failed", "provider", slug, "error", err)
		return nil, err
	}
	provider, err := r.providerFromConfig(cfg)
	if err != nil {
		Logger().Error("registry_provider_build_failed", "provider", slug, "error", err)
		return nil, err
	}
	return provider, nil
}

func (r *Registry) TestMode(ctx context.Context) (bool, error) {
	return r.reader.TestMode(ctx)
}

func (r *Registry) providerFromConfig(cfg ProviderConfig) (Provider, error) {
	switch strings.ToLower(cfg.Provider) {
	case "paytr":
		if cfg.MerchantID == "" || cfg.MerchantKey == "" || cfg.MerchantSalt == "" {
			return nil, fmt.Errorf("%w: paytr credentials incomplete", ErrNotConfigured)
		}
		return NewPayTRProvider(cfg), nil
	case "iyzico":
		if cfg.APIKey == "" || cfg.SecretKey == "" {
			return nil, fmt.Errorf("%w: iyzico credentials incomplete", ErrNotConfigured)
		}
		return NewIyzicoProvider(cfg), nil
	default:
		return nil, fmt.Errorf("%w: unknown provider %s", ErrNotConfigured, cfg.Provider)
	}
}
