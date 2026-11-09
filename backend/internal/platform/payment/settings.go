package payment

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
)

// DBSettingsReader loads payment gateway configuration from the database.
type DBSettingsReader struct {
	q   *db.Queries
	box *crypto.SecretBox
}

func NewDBSettingsReader(q *db.Queries, box *crypto.SecretBox) *DBSettingsReader {
	return &DBSettingsReader{q: q, box: box}
}

func (r *DBSettingsReader) ActiveProvider(ctx context.Context) (string, error) {
	row, err := r.q.GetPaymentSettings(ctx)
	if err != nil {
		return "", err
	}
	return string(row.ActiveProvider), nil
}

func (r *DBSettingsReader) TestMode(ctx context.Context) (bool, error) {
	row, err := r.q.GetPaymentSettings(ctx)
	if err != nil {
		return false, err
	}
	return row.TestMode, nil
}

func (r *DBSettingsReader) ProviderConfig(ctx context.Context, provider string) (ProviderConfig, error) {
	row, err := r.q.GetPaymentProviderSettings(ctx, db.PaymentProvider(provider))
	if err != nil {
		return ProviderConfig{}, err
	}
	cfg := ProviderConfig{
		Provider:   string(row.Provider),
		Enabled:    row.Enabled,
		MerchantID: strings.TrimSpace(row.MerchantID),
		BaseURL:    strings.TrimSpace(row.BaseUrl),
	}
	if row.MerchantKeyEnc.Valid && row.MerchantKeyEnc.String != "" {
		v, err := r.box.Decrypt(row.MerchantKeyEnc.String)
		if err != nil {
			return ProviderConfig{}, fmt.Errorf("decrypt merchant key: %w", err)
		}
		cfg.MerchantKey = v
	}
	if row.MerchantSaltEnc.Valid && row.MerchantSaltEnc.String != "" {
		v, err := r.box.Decrypt(row.MerchantSaltEnc.String)
		if err != nil {
			return ProviderConfig{}, fmt.Errorf("decrypt merchant salt: %w", err)
		}
		cfg.MerchantSalt = v
	}
	if row.ApiKeyEnc.Valid && row.ApiKeyEnc.String != "" {
		v, err := r.box.Decrypt(row.ApiKeyEnc.String)
		if err != nil {
			return ProviderConfig{}, fmt.Errorf("decrypt api key: %w", err)
		}
		cfg.APIKey = v
	}
	if row.SecretKeyEnc.Valid && row.SecretKeyEnc.String != "" {
		v, err := r.box.Decrypt(row.SecretKeyEnc.String)
		if err != nil {
			return ProviderConfig{}, fmt.Errorf("decrypt secret key: %w", err)
		}
		cfg.SecretKey = v
	}
	return cfg, nil
}
