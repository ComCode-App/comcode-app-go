package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// TEC-590: payment history and PDF receipts of the active account. Receipts are rendered on
// demand from the payment rows (checkout sessions, renewal attempts, order payments); nothing is
// stored. A receipt confirms a payment and says so on the page: it is not an invoice (e-arşiv
// integration is a separate decision).

// Payment kinds in the history (see billing_payments.sql).
const (
	PaymentKindSubscriptionCheckout = "subscription_checkout"
	PaymentKindSubscriptionRenewal  = "subscription_renewal"
	PaymentKindSubscriptionUpgrade  = "subscription_upgrade"
	PaymentKindOrder                = "order"
	PaymentKindOrderManual          = "order_manual"
)

// SetStorage gives receipts the letterhead logo (storage.Driver of the app settings logo).
func (s *Service) SetStorage(driver storage.Driver) { s.storage = driver }

// AccountPayment is one successful charge of the account.
type AccountPayment struct {
	UUID            uuid.UUID  `json:"uuid"`
	Kind            string     `json:"kind"`
	Provider        string     `json:"provider"`
	ProviderRef     string     `json:"provider_ref,omitempty"`
	AmountMinor     int64      `json:"amount_minor"`
	Currency        string     `json:"currency"`
	PaidAt          time.Time  `json:"paid_at"`
	PlanName        string     `json:"plan_name,omitempty"`
	BillingInterval string     `json:"billing_interval,omitempty"`
	OrderNumber     string     `json:"order_number,omitempty"`
	OrderUUID       *uuid.UUID `json:"order_uuid,omitempty"`
	ReceiptNumber   string     `json:"receipt_number"`
}

// PaymentHistory is a page of the account's payments.
type PaymentHistory struct {
	Items  []AccountPayment `json:"items"`
	Total  int64            `json:"total"`
	Limit  int32            `json:"limit"`
	Offset int32            `json:"offset"`
}

// ListPayments returns the active account's payments, newest first. Any member may read them
// (the same rule as the subscription itself).
func (s *Service) ListPayments(ctx context.Context, account authctx.Account, limit, offset int32) (PaymentHistory, error) {
	if account.ID <= 0 {
		return PaymentHistory{}, ErrForbidden
	}
	rows, err := s.q.ListAccountPaymentHistory(ctx, db.ListAccountPaymentHistoryParams{
		AccountID: account.ID, PageLimit: limit, PageOffset: offset,
	})
	if err != nil {
		return PaymentHistory{}, fmt.Errorf("list payments: %w", err)
	}
	out := PaymentHistory{Items: make([]AccountPayment, 0, len(rows)), Limit: limit, Offset: offset}
	for _, row := range rows {
		out.Total = row.Total
		out.Items = append(out.Items, mapAccountPayment(row))
	}
	return out, nil
}

// GetPayment returns one payment of the active account.
func (s *Service) GetPayment(ctx context.Context, account authctx.Account, paymentUUID uuid.UUID) (AccountPayment, error) {
	if account.ID <= 0 {
		return AccountPayment{}, ErrForbidden
	}
	rows, err := s.q.ListAccountPaymentHistory(ctx, db.ListAccountPaymentHistoryParams{
		AccountID:   account.ID,
		PaymentUuid: pgtype.UUID{Bytes: paymentUUID, Valid: true},
		PageLimit:   1,
	})
	if err != nil {
		return AccountPayment{}, fmt.Errorf("get payment: %w", err)
	}
	if len(rows) == 0 {
		return AccountPayment{}, ErrNotFound
	}
	return mapAccountPayment(rows[0]), nil
}

// Receipt is a rendered PDF receipt.
type Receipt struct {
	Filename string
	PDF      []byte
}

// ReceiptPDF renders the letterheaded receipt of one payment of the active account.
func (s *Service) ReceiptPDF(ctx context.Context, account authctx.Account, paymentUUID uuid.UUID, locale string) (Receipt, error) {
	payment, err := s.GetPayment(ctx, account, paymentUUID)
	if err != nil {
		return Receipt{}, err
	}
	loc := i18n.Normalize(locale)
	accountName := ""
	if acc, err := s.q.GetAccountByID(ctx, account.ID); err == nil {
		accountName = acc.Name
	}
	settings, err := s.q.GetAppSettings(ctx)
	if err != nil {
		return Receipt{}, fmt.Errorf("receipt settings: %w", err)
	}
	lh, err := ioengine.LoadLetterheadLogo(ctx, s.storage, settings)
	if err != nil {
		// A missing logo must not block the receipt; the letterhead text still renders.
		s.log.Warn("receipt_logo_unavailable", "error", err)
		lh = ioengine.LetterheadFromSettings(settings)
	}

	doc := ioengine.ReceiptDocument{
		Title:    i18n.Translate(loc, "receipt.title"),
		Number:   payment.ReceiptNumber,
		IssuedAt: time.Now().UTC(),
		Lines: []ioengine.ReceiptLine{
			{Label: i18n.Translate(loc, "receipt.paid_at"), Value: formatReceiptTime(payment.PaidAt, loc)},
			{Label: i18n.Translate(loc, "receipt.account"), Value: accountName},
			{Label: i18n.Translate(loc, "receipt.description"), Value: paymentDescription(payment, loc)},
			{Label: i18n.Translate(loc, "receipt.provider"), Value: paymentProviderLabel(payment, loc)},
			{Label: i18n.Translate(loc, "receipt.reference"), Value: paymentReference(payment)},
		},
		AmountLabel: i18n.Translate(loc, "receipt.amount"),
		Amount:      ioengine.FormatMoney(payment.AmountMinor, payment.Currency, locale),
		Note:        i18n.Translate(loc, "receipt.note"),
	}
	pdf, err := ioengine.EncodeReceiptPDF(doc, locale, &lh)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Filename: payment.ReceiptNumber + ".pdf", PDF: pdf}, nil
}

func mapAccountPayment(row db.ListAccountPaymentHistoryRow) AccountPayment {
	p := AccountPayment{
		UUID:            row.Uuid,
		Kind:            row.Kind,
		Provider:        row.Provider,
		ProviderRef:     row.ProviderRef,
		AmountMinor:     row.AmountMinor,
		Currency:        strings.TrimSpace(row.Currency),
		PlanName:        row.PlanName,
		BillingInterval: row.BillingInterval,
		OrderNumber:     row.OrderNumber,
		ReceiptNumber:   receiptNumber(row.Uuid),
	}
	if row.PaidAt.Valid {
		p.PaidAt = row.PaidAt.Time.UTC()
	}
	if row.OrderUuid.Valid {
		id := uuid.UUID(row.OrderUuid.Bytes)
		p.OrderUUID = &id
	}
	return p
}

// receiptNumber is stable per payment: "RCP-" + the first 12 hex digits of its uuid.
func receiptNumber(id uuid.UUID) string {
	hex := strings.ReplaceAll(id.String(), "-", "")
	return "RCP-" + strings.ToUpper(hex[:12])
}

func paymentDescription(p AccountPayment, loc i18n.Locale) string {
	kind := i18n.Translate(loc, "receipt.kind."+p.Kind)
	switch p.Kind {
	case PaymentKindOrder, PaymentKindOrderManual:
		if p.OrderNumber != "" {
			return kind + " · " + p.OrderNumber
		}
		return kind
	default:
		parts := []string{kind}
		if p.PlanName != "" {
			plan := p.PlanName
			if p.BillingInterval != "" {
				if label := i18n.Translate(loc, "receipt.interval."+p.BillingInterval); label != "receipt.interval."+p.BillingInterval {
					plan += " (" + label + ")"
				}
			}
			parts = append(parts, plan)
		}
		return strings.Join(parts, " · ")
	}
}

func paymentProviderLabel(p AccountPayment, loc i18n.Locale) string {
	provider := strings.ToLower(strings.TrimSpace(p.Provider))
	if provider == "" || provider == "none" {
		provider = "manual"
	}
	key := "receipt.provider." + provider
	if label := i18n.Translate(loc, key); label != key {
		return label
	}
	return p.Provider
}

func paymentReference(p AccountPayment) string {
	if p.ProviderRef != "" {
		return p.ProviderRef
	}
	return p.UUID.String()
}

func formatReceiptTime(t time.Time, loc i18n.Locale) string {
	if t.IsZero() {
		return ""
	}
	if loc == i18n.LocaleEN {
		return t.Format("2006-01-02 15:04 UTC")
	}
	return t.Format("02.01.2006 15:04 UTC")
}
