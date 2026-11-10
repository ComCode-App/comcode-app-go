// Package pushtext holds the lock-screen texts of mobile push notifications (TEC-1187).
//
// An in-app title or body may carry personal data (a lead's name, an order number, an amount).
// A push is shown on a locked screen and passes through Expo, Apple and Google, so it only ever
// uses the fixed tr/en texts below: no template variables, no names, emails, codes or amounts.
// A notification code without an entry here is not pushed at all. Security emails (sign-in
// codes, password reset, verification, invitations) are email-only and never get an entry.
package pushtext

import "strings"

// Text is one localized push.
type Text struct {
	Title string
	Body  string
}

type pair struct{ tr, en Text }

// orderStatuses are the order notification codes (orders.status.<status>, migration 000047).
var orderStatuses = map[string]pair{
	"pending":         {Text{"Siparişiniz alındı", "Siparişiniz oluşturuldu."}, Text{"Order received", "Your order has been placed."}},
	"paid":            {Text{"Ödeme alındı", "Siparişinizin ödemesi alındı."}, Text{"Payment received", "The payment for your order was received."}},
	"design_pending":  {Text{"Tasarım bekleniyor", "Siparişiniz için tasarım onayı bekleniyor."}, Text{"Design pending", "Your order is waiting for design approval."}},
	"design_approved": {Text{"Tasarım onaylandı", "Siparişinizin tasarımı onaylandı."}, Text{"Design approved", "The design of your order was approved."}},
	"in_production":   {Text{"Üretimde", "Siparişiniz üretime alındı."}, Text{"In production", "Your order is in production."}},
	"ready_to_ship":   {Text{"Kargoya hazır", "Siparişiniz kargoya hazırlanıyor."}, Text{"Ready to ship", "Your order is ready to ship."}},
	"shipped":         {Text{"Kargoya verildi", "Siparişiniz kargoya verildi."}, Text{"Shipped", "Your order has been shipped."}},
	"delivered":       {Text{"Teslim edildi", "Siparişiniz teslim edildi."}, Text{"Delivered", "Your order has been delivered."}},
	"cancelled":       {Text{"Sipariş iptal edildi", "Siparişiniz iptal edildi."}, Text{"Order cancelled", "Your order was cancelled."}},
	"refunded":        {Text{"İade yapıldı", "Siparişinizin iadesi yapıldı."}, Text{"Refunded", "Your order was refunded."}},
}

var catalog = map[string]pair{
	// Leads (owners and admins of the account).
	"leads.received": {
		Text{"Yeni iletişim talebi", "Yeni bir iletişim talebi aldınız."},
		Text{"New contact request", "You received a new contact request."},
	},
	"leads.exchange_received": {
		Text{"Yeni kartvizit değişimi", "Biri sizinle iletişim bilgilerini paylaştı."},
		Text{"New contact exchange", "Someone shared their contact details with you."},
	},
	// Profile verification reviews.
	"profiles.verification.approved": {
		Text{"Profil doğrulandı", "Bir profilin doğrulama başvurusu onaylandı."},
		Text{"Profile verified", "A profile verification request was approved."},
	},
	"profiles.verification.rejected": {
		Text{"Doğrulama reddedildi", "Bir profilin doğrulama başvurusu reddedildi."},
		Text{"Verification rejected", "A profile verification request was rejected."},
	},
	"profiles.verification.revoked": {
		Text{"Doğrulama kaldırıldı", "Bir profilin doğrulama rozeti kaldırıldı."},
		Text{"Verification revoked", "A profile's verification badge was removed."},
	},
	"profiles.verification.re_review": {
		Text{"Doğrulama yeniden incelemede", "Bir profilin doğrulaması yeniden incelemeye alındı."},
		Text{"Verification under review", "A profile's verification is under review again."},
	},
	// Plan quotas.
	"entitlements.quota_warning": {
		Text{"Kota sınırına yaklaşıldı", "Planınızın aylık kullanım sınırına yaklaştınız."},
		Text{"Quota almost reached", "You are close to your plan's monthly limit."},
	},
	"entitlements.quota_reached": {
		Text{"Kota sınırına ulaşıldı", "Planınızın aylık kullanım sınırına ulaştınız."},
		Text{"Quota reached", "You reached your plan's monthly limit."},
	},
	// Billing (no amounts on the lock screen).
	"subscription.renewed": {
		Text{"Abonelik yenilendi", "Aboneliğiniz yenilendi."},
		Text{"Subscription renewed", "Your subscription was renewed."},
	},
	"payment.succeeded": {
		Text{"Ödeme alındı", "Abonelik ödemeniz alındı."},
		Text{"Payment received", "Your subscription payment was received."},
	},
	"payment.failed": {
		Text{"Ödeme alınamadı", "Abonelik ödemeniz alınamadı. Faturalama sayfasını kontrol edin."},
		Text{"Payment failed", "Your subscription payment failed. Check the billing page."},
	},
	"subscription.past_due": {
		Text{"Ödeme gecikti", "Abonelik ödemeniz gecikti."},
		Text{"Payment overdue", "Your subscription payment is overdue."},
	},
	"subscription.expired": {
		Text{"Abonelik sona erdi", "Aboneliğinizin süresi doldu."},
		Text{"Subscription expired", "Your subscription has expired."},
	},
	"subscription.canceled": {
		Text{"Abonelik iptal edildi", "Aboneliğiniz iptal edildi."},
		Text{"Subscription canceled", "Your subscription was canceled."},
	},
	// Background jobs started by the user.
	"exports.ready": {
		Text{"Dışa aktarım hazır", "Dışa aktarımınız indirilmeye hazır."},
		Text{"Export ready", "Your export is ready to download."},
	},
	"imports.applied": {
		Text{"İçe aktarım tamamlandı", "İçe aktarımınız tamamlandı."},
		Text{"Import finished", "Your import has finished."},
	},
	"imports.failed": {
		Text{"İçe aktarım başarısız", "İçe aktarımınız tamamlanamadı."},
		Text{"Import failed", "Your import could not be completed."},
	},
	"bulk.completed": {
		Text{"Toplu işlem tamamlandı", "Toplu işleminiz tamamlandı."},
		Text{"Bulk action finished", "Your bulk action has finished."},
	},
	"bulk.failed": {
		Text{"Toplu işlem başarısız", "Toplu işleminiz tamamlanamadı."},
		Text{"Bulk action failed", "Your bulk action could not be completed."},
	},
	// Account.
	"auth.welcome": {
		Text{"Hoş geldiniz", "Hesabınız hazır."},
		Text{"Welcome", "Your account is ready."},
	},
	"auth.profile_updated": {
		Text{"Profil güncellendi", "Profil bilgileriniz güncellendi."},
		Text{"Profile updated", "Your profile details were updated."},
	},
	"notifications.test": {
		Text{"Test bildirimi", "Bildirimler bu cihazda çalışıyor."},
		Text{"Test notification", "Notifications work on this device."},
	},
}

func init() {
	for status, p := range orderStatuses {
		catalog["orders.status."+status] = p
	}
}

// For returns the push text of a notification code in locale ("en", else Turkish). ok is false
// for a code without an entry: such a notification is not pushed.
func For(code, locale string) (Text, bool) {
	p, ok := catalog[strings.TrimSpace(code)]
	if !ok {
		return Text{}, false
	}
	if strings.EqualFold(strings.TrimSpace(locale), "en") {
		return p.en, true
	}
	return p.tr, true
}

// Known reports whether code has a push text (and is therefore pushed).
func Known(code string) bool {
	_, ok := catalog[strings.TrimSpace(code)]
	return ok
}

// Codes lists every push-eligible notification code (tests, docs).
func Codes() []string {
	out := make([]string, 0, len(catalog))
	for code := range catalog {
		out = append(out, code)
	}
	return out
}
