package usecase

import (
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/google/uuid"
)

func TestReceiptNumberIsStable(t *testing.T) {
	id := uuid.MustParse("3f2a9c1b-7d4e-4a10-9b2c-0123456789ab")
	if got := receiptNumber(id); got != "RCP-3F2A9C1B7D4E" {
		t.Fatalf("receiptNumber = %q", got)
	}
}

func TestPaymentDescriptionAndProvider(t *testing.T) {
	sub := AccountPayment{Kind: PaymentKindSubscriptionRenewal, PlanName: "Pro", BillingInterval: "month", Provider: "paytr"}
	if got := paymentDescription(sub, i18n.LocaleTR); got != "Abonelik yenileme · Pro (aylık)" {
		t.Errorf("tr subscription: %q", got)
	}
	if got := paymentDescription(sub, i18n.LocaleEN); got != "Subscription renewal · Pro (monthly)" {
		t.Errorf("en subscription: %q", got)
	}
	order := AccountPayment{Kind: PaymentKindOrderManual, OrderNumber: "ORD-2026-0001", Provider: "Havale"}
	if got := paymentDescription(order, i18n.LocaleTR); got != "Kart siparişi (havale/EFT) · ORD-2026-0001" {
		t.Errorf("tr order: %q", got)
	}
	if got := paymentProviderLabel(sub, i18n.LocaleEN); got != "Credit card (PayTR)" {
		t.Errorf("provider paytr: %q", got)
	}
	if got := paymentProviderLabel(order, i18n.LocaleTR); got != "Havale" {
		t.Errorf("free-text provider must pass through: %q", got)
	}
	if got := paymentProviderLabel(AccountPayment{}, i18n.LocaleTR); got != "Havale / EFT" {
		t.Errorf("empty provider: %q", got)
	}
}
