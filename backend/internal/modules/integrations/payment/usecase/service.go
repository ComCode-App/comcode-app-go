package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrNotConfigured  = errors.New("payment provider is not configured")
)

type settingsStore interface {
	GetPaymentSettings(ctx context.Context) (db.PaymentSetting, error)
	UpdatePaymentSettings(ctx context.Context, arg db.UpdatePaymentSettingsParams) (db.PaymentSetting, error)
	GetPaymentProviderSettings(ctx context.Context, provider db.PaymentProvider) (db.PaymentProviderSetting, error)
	UpdatePaymentProviderSettings(ctx context.Context, arg db.UpdatePaymentProviderSettingsParams) (db.PaymentProviderSetting, error)
}

type Service struct {
	q   settingsStore
	box *crypto.SecretBox
}

func New(q settingsStore, box *crypto.SecretBox) *Service {
	return &Service{q: q, box: box}
}

type Settings struct {
	ActiveProvider string `json:"active_provider"`
	TestMode       bool   `json:"test_mode"`
}

type PatchSettingsInput struct {
	ActiveProvider *string `json:"active_provider"`
	TestMode       *bool   `json:"test_mode"`
}

type PayTRSettings struct {
	Enabled                bool   `json:"enabled"`
	MerchantID             string `json:"merchant_id"`
	MerchantKeyConfigured  bool   `json:"merchant_key_configured"`
	MerchantSaltConfigured bool   `json:"merchant_salt_configured"`
}

type PatchPayTRInput struct {
	Enabled      *bool   `json:"enabled"`
	MerchantID   *string `json:"merchant_id"`
	MerchantKey  *string `json:"merchant_key"`
	MerchantSalt *string `json:"merchant_salt"`
}

type IyzicoSettings struct {
	Enabled             bool   `json:"enabled"`
	BaseURL             string `json:"base_url"`
	APIKeyConfigured    bool   `json:"api_key_configured"`
	SecretKeyConfigured bool   `json:"secret_key_configured"`
}

type PatchIyzicoInput struct {
	Enabled   *bool   `json:"enabled"`
	BaseURL   *string `json:"base_url"`
	APIKey    *string `json:"api_key"`
	SecretKey *string `json:"secret_key"`
}

func (s *Service) GetSettings(ctx context.Context) (Settings, error) {
	row, err := s.q.GetPaymentSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	return Settings{ActiveProvider: string(row.ActiveProvider), TestMode: row.TestMode}, nil
}

func (s *Service) PatchSettings(ctx context.Context, in PatchSettingsInput) (Settings, error) {
	params := db.UpdatePaymentSettingsParams{}
	if in.ActiveProvider != nil {
		provider := db.ActivePaymentProvider(strings.TrimSpace(*in.ActiveProvider))
		switch provider {
		case db.ActivePaymentProviderNone, db.ActivePaymentProviderPaytr, db.ActivePaymentProviderIyzico:
			params.ActiveProvider = db.NullActivePaymentProvider{ActivePaymentProvider: provider, Valid: true}
		default:
			return Settings{}, fmt.Errorf("%w: invalid active_provider", ErrInvalidRequest)
		}
	}
	if in.TestMode != nil {
		params.TestMode = pgtype.Bool{Bool: *in.TestMode, Valid: true}
	}
	row, err := s.q.UpdatePaymentSettings(ctx, params)
	if err != nil {
		return Settings{}, err
	}
	return Settings{ActiveProvider: string(row.ActiveProvider), TestMode: row.TestMode}, nil
}

func (s *Service) GetPayTR(ctx context.Context) (PayTRSettings, error) {
	row, err := s.q.GetPaymentProviderSettings(ctx, db.PaymentProviderPaytr)
	if err != nil {
		return PayTRSettings{}, err
	}
	return mapPayTR(row), nil
}

func (s *Service) PatchPayTR(ctx context.Context, in PatchPayTRInput) (PayTRSettings, error) {
	params := db.UpdatePaymentProviderSettingsParams{Provider: db.PaymentProviderPaytr}
	if in.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *in.Enabled, Valid: true}
	}
	if in.MerchantID != nil {
		params.MerchantID = pgtype.Text{String: strings.TrimSpace(*in.MerchantID), Valid: true}
	}
	if in.MerchantKey != nil {
		if v := strings.TrimSpace(*in.MerchantKey); v != "" {
			enc, err := s.box.Encrypt(v)
			if err != nil {
				return PayTRSettings{}, err
			}
			params.MerchantKeyEnc = pgtype.Text{String: enc, Valid: true}
		}
	}
	if in.MerchantSalt != nil {
		if v := strings.TrimSpace(*in.MerchantSalt); v != "" {
			enc, err := s.box.Encrypt(v)
			if err != nil {
				return PayTRSettings{}, err
			}
			params.MerchantSaltEnc = pgtype.Text{String: enc, Valid: true}
		}
	}
	row, err := s.q.UpdatePaymentProviderSettings(ctx, params)
	if err != nil {
		return PayTRSettings{}, err
	}
	return mapPayTR(row), nil
}

func (s *Service) GetIyzico(ctx context.Context) (IyzicoSettings, error) {
	row, err := s.q.GetPaymentProviderSettings(ctx, db.PaymentProviderIyzico)
	if err != nil {
		return IyzicoSettings{}, err
	}
	return mapIyzico(row), nil
}

func (s *Service) PatchIyzico(ctx context.Context, in PatchIyzicoInput) (IyzicoSettings, error) {
	params := db.UpdatePaymentProviderSettingsParams{Provider: db.PaymentProviderIyzico}
	if in.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *in.Enabled, Valid: true}
	}
	if in.BaseURL != nil {
		params.BaseUrl = pgtype.Text{String: strings.TrimSpace(*in.BaseURL), Valid: true}
	}
	if in.APIKey != nil {
		if v := strings.TrimSpace(*in.APIKey); v != "" {
			enc, err := s.box.Encrypt(v)
			if err != nil {
				return IyzicoSettings{}, err
			}
			params.ApiKeyEnc = pgtype.Text{String: enc, Valid: true}
		}
	}
	if in.SecretKey != nil {
		if v := strings.TrimSpace(*in.SecretKey); v != "" {
			enc, err := s.box.Encrypt(v)
			if err != nil {
				return IyzicoSettings{}, err
			}
			params.SecretKeyEnc = pgtype.Text{String: enc, Valid: true}
		}
	}
	row, err := s.q.UpdatePaymentProviderSettings(ctx, params)
	if err != nil {
		return IyzicoSettings{}, err
	}
	return mapIyzico(row), nil
}

func mapPayTR(row db.PaymentProviderSetting) PayTRSettings {
	return PayTRSettings{
		Enabled:                row.Enabled,
		MerchantID:             row.MerchantID,
		MerchantKeyConfigured:  row.MerchantKeyEnc.Valid && row.MerchantKeyEnc.String != "",
		MerchantSaltConfigured: row.MerchantSaltEnc.Valid && row.MerchantSaltEnc.String != "",
	}
}

func mapIyzico(row db.PaymentProviderSetting) IyzicoSettings {
	baseURL := row.BaseUrl
	if baseURL == "" {
		baseURL = "https://sandbox-api.iyzipay.com"
	}
	return IyzicoSettings{
		Enabled:             row.Enabled,
		BaseURL:             baseURL,
		APIKeyConfigured:    row.ApiKeyEnc.Valid && row.ApiKeyEnc.String != "",
		SecretKeyConfigured: row.SecretKeyEnc.Valid && row.SecretKeyEnc.String != "",
	}
}
