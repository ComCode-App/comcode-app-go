package payment

import (
	"context"
	"errors"
)

var (
	ErrNotConfigured = errors.New("payment provider is not configured")
	ErrUnavailable   = errors.New("payment provider is unavailable")
)

// CheckoutInput is passed to a provider when starting checkout.
type CheckoutInput struct {
	SessionUUID string
	MerchantOID string
	AmountMinor int64
	Currency    string
	PlanName    string
	UserEmail   string
	UserName    string
	UserPhone   string
	UserAddress string
	ClientIP    string
	SuccessURL  string
	FailURL     string
	CallbackURL string
	TestMode    bool
	StoreCard   bool
	SavedUToken string
	SavedCToken string
	// Physical marks a physical goods purchase (card orders, TEC-943) instead of a subscription.
	Physical bool
}

// CardAddInput starts a card-registration flow.
type CardAddInput struct {
	SessionUUID string
	MerchantOID string
	UserEmail   string
	UserName    string
	UserPhone   string
	UserAddress string
	ClientIP    string
	SuccessURL  string
	FailURL     string
	CallbackURL string
	TestMode    bool
}

// SavedCardChargeInput charges a saved card for subscription.
type SavedCardChargeInput struct {
	MerchantOID string
	AmountMinor int64
	Currency    string
	UserEmail   string
	UserName    string
	UserPhone   string
	UserAddress string
	ClientIP    string
	TestMode    bool
	UToken      string
	CToken      string
	PlanName    string
}

// SavedCardView is a masked saved card for API responses.
type SavedCardView struct {
	Last4    string
	Brand    string
	ExpMonth *int32
	ExpYear  *int32
}

// ProviderCard is provider-side card metadata during sync.
type ProviderCard struct {
	CardRef     string
	CustomerRef string
	Last4       string
	Brand       string
	ExpMonth    *int32
	ExpYear     *int32
}

// CheckoutResult is returned after provider initialization.
type CheckoutResult struct {
	ProviderToken string
	CheckoutURL   string
	IframeToken   string
}

// CallbackInput carries verified callback data.
type CallbackInput struct {
	MerchantOID string
	Status      string
	ProviderRef string
	TotalAmount string
	RawPayload  map[string]string
}

// CallbackResult indicates whether payment succeeded.
type CallbackResult struct {
	Success       bool
	MerchantOID   string
	ProviderRef   string
	UToken        string
	CToken        string
	CardLast4     string
	CardBrand     string
	IsCardSession bool
}

// Provider implements gateway-specific checkout and callback verification.
type Provider interface {
	Slug() string
	InitCheckout(ctx context.Context, in CheckoutInput) (CheckoutResult, error)
	InitCardAdd(ctx context.Context, in CardAddInput) (CheckoutResult, error)
	ChargeSavedCard(ctx context.Context, in SavedCardChargeInput) (CallbackResult, error)
	ListSavedCards(ctx context.Context, customerRef string) ([]ProviderCard, error)
	DeleteSavedCard(ctx context.Context, customerRef, cardRef string) error
	VerifyCallback(ctx context.Context, payload map[string]string) (CallbackResult, error)
	// Refund returns part or all of a captured payment (TEC-935). An error wrapping
	// ErrRefundRejected means the provider answered and refunded nothing; any other error leaves
	// the outcome unknown.
	Refund(ctx context.Context, in RefundInput) (RefundResult, error)
}

// ErrRefundRejected marks a refund the provider declined: nothing was refunded.
var ErrRefundRejected = errors.New("refund rejected by payment provider")

// RefundInput refunds AmountMinor of the payment identified by MerchantOID (PayTR) or
// PaymentRef (iyzico paymentId).
type RefundInput struct {
	MerchantOID string
	PaymentRef  string
	// ReferenceNo is unique per refund: PayTR reference_no (a retry with the same value is not
	// refunded twice) and iyzico conversationId.
	ReferenceNo string
	AmountMinor int64
	Currency    string
	ClientIP    string
	Reason      string
}

// RefundResult is the provider's reference for a completed refund.
type RefundResult struct {
	ProviderRef string
}

// SettingsReader loads runtime gateway configuration.
type SettingsReader interface {
	ActiveProvider(ctx context.Context) (string, error)
	ProviderConfig(ctx context.Context, provider string) (ProviderConfig, error)
	TestMode(ctx context.Context) (bool, error)
}

// ProviderConfig holds decrypted credentials for a provider.
type ProviderConfig struct {
	Provider     string
	Enabled      bool
	MerchantID   string
	BaseURL      string
	MerchantKey  string
	MerchantSalt string
	APIKey       string
	SecretKey    string
}
