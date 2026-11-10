package pushtext

import (
	"regexp"
	"strings"
	"testing"
)

// pushEligible are the notification codes ComCode delivers in-app to a user. Each must have a
// lock-screen text; a new in-app code needs an entry (or is deliberately left out of push).
var pushEligible = []string{
	"leads.received", "leads.exchange_received",
	"profiles.verification.approved", "profiles.verification.rejected",
	"profiles.verification.revoked", "profiles.verification.re_review",
	"entitlements.quota_warning", "entitlements.quota_reached",
	"subscription.renewed", "payment.succeeded", "payment.failed",
	"subscription.past_due", "subscription.expired", "subscription.canceled",
	"orders.status.pending", "orders.status.paid", "orders.status.design_pending",
	"orders.status.design_approved", "orders.status.in_production", "orders.status.ready_to_ship",
	"orders.status.shipped", "orders.status.delivered", "orders.status.cancelled", "orders.status.refunded",
	"exports.ready", "imports.applied", "imports.failed", "bulk.completed", "bulk.failed",
	"auth.welcome", "auth.profile_updated", "notifications.test",
}

// Security emails and secret-bearing emails are never pushed.
var neverPushed = []string{
	"auth.password_reset", "auth.email_verification", "auth.login_code", "auth.refresh_reuse",
	"accounts.invite", "leads.exchange_copy",
}

// A push text must not be able to carry personal data: no placeholders of any template syntax,
// no digits (codes, amounts, phone numbers), no emails or links.
var unsafeText = regexp.MustCompile(`\{|\}|%[sdv]|\$|@|[0-9]|https?:|<`)

func TestEveryPushEligibleCodeHasTurkishAndEnglishText(t *testing.T) {
	for _, code := range pushEligible {
		for _, locale := range []string{"tr", "en"} {
			txt, ok := For(code, locale)
			if !ok {
				t.Errorf("%s: no push text", code)
				continue
			}
			if strings.TrimSpace(txt.Title) == "" || strings.TrimSpace(txt.Body) == "" {
				t.Errorf("%s/%s: empty title or body: %+v", code, locale, txt)
			}
		}
		tr, _ := For(code, "tr")
		en, _ := For(code, "en")
		if tr == en {
			t.Errorf("%s: Turkish and English texts are the same", code)
		}
	}
}

func TestCatalogHasNoPersonalDataVariables(t *testing.T) {
	codes := Codes()
	if len(codes) < len(pushEligible) {
		t.Fatalf("catalog has %d codes, want at least %d", len(codes), len(pushEligible))
	}
	for _, code := range codes {
		for _, locale := range []string{"tr", "en"} {
			txt, _ := For(code, locale)
			for _, s := range []string{txt.Title, txt.Body} {
				if unsafeText.MatchString(s) {
					t.Errorf("%s/%s: %q may carry variables or personal data", code, locale, s)
				}
			}
		}
	}
}

func TestSecurityCodesAreNeverPushed(t *testing.T) {
	for _, code := range neverPushed {
		if Known(code) {
			t.Errorf("%s must never be pushed", code)
		}
		if _, ok := For(code, "tr"); ok {
			t.Errorf("%s has a push text", code)
		}
	}
	if Known("") || Known("custom.admin_code") {
		t.Fatal("codes without an entry must not be pushed")
	}
}

func TestLocaleFallsBackToTurkish(t *testing.T) {
	tr, _ := For("leads.received", "tr")
	for _, loc := range []string{"", "de", "TR"} {
		if got, _ := For("leads.received", loc); got != tr {
			t.Errorf("locale %q = %+v, want Turkish", loc, got)
		}
	}
	if en, _ := For("leads.received", "EN"); en.Title != "New contact request" {
		t.Errorf("EN = %+v", en)
	}
}
