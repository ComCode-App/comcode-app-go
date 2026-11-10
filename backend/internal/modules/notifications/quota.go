package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
)

// Enqueuer is the part of the notifications service the quota alerter uses.
type Enqueuer interface {
	Enqueue(ctx context.Context, in model.EnqueueInput) ([]model.Notification, error)
}

// QuotaAlerter tells an account's owners and admins in the panel and by email that a monthly
// quota reached 80% or 100% (TEC-941). It implements entitlement.UsageAlerter.
type QuotaAlerter struct {
	svc Enqueuer
	q   *db.Queries
	log *slog.Logger
}

// NewQuotaAlerter creates the alerter.
func NewQuotaAlerter(svc Enqueuer, q *db.Queries, log *slog.Logger) *QuotaAlerter {
	if log == nil {
		log = slog.Default()
	}
	return &QuotaAlerter{svc: svc, q: q, log: log}
}

// quotaText is what the templates say about a feature, per language.
type quotaText struct {
	feature, after, afterReached string
}

var quotaTexts = map[string]map[string]quotaText{
	"tr": {
		entitlement.FeatureLeadsMonthlyMax: {"lead",
			"yeni lead'ler kilitli saklanır; ad ve iletişim bilgileri gizlenir.",
			"Yeni lead'ler kilitli saklanacak; ad ve iletişim bilgileri gizlenir."},
		entitlement.FeatureExchangeMonthlyMax: {"kartvizit değişimi",
			"yeni kartvizit değişimleri kilitli saklanır; ad ve iletişim bilgileri gizlenir.",
			"Yeni kartvizit değişimleri kilitli saklanacak; ad ve iletişim bilgileri gizlenir."},
		entitlement.FeatureLeadsExportMonthlyMax: {"lead dışa aktarma",
			"bu ay yeni dışa aktarma yapılamaz.",
			"Bu ay yeni dışa aktarma yapılamaz."},
	},
	"en": {
		entitlement.FeatureLeadsMonthlyMax: {"leads",
			"new leads are kept locked with their name and contact details hidden.",
			"New leads will be kept locked with their name and contact details hidden."},
		entitlement.FeatureExchangeMonthlyMax: {"contact exchanges",
			"new contact exchanges are kept locked with their name and contact details hidden.",
			"New contact exchanges will be kept locked with their name and contact details hidden."},
		entitlement.FeatureLeadsExportMonthlyMax: {"lead exports",
			"no more exports can be run this month.",
			"No more exports can be run this month."},
	},
}

// QuotaAlert enqueues the alert for every owner and admin; failures are logged only.
func (a *QuotaAlerter) QuotaAlert(ctx context.Context, alert entitlement.QuotaAlert) {
	template := "entitlements.quota_warning"
	if alert.Percent >= 100 {
		template = "entitlements.quota_reached"
	}
	account := ""
	if acc, err := a.q.GetAccountByID(ctx, alert.AccountID); err == nil {
		account = acc.Name
	}
	ids, err := a.q.ListLeadNotifyUserIDs(ctx, alert.AccountID)
	if err != nil {
		a.log.Warn("quota_alert_recipients_failed", "error", err)
		return
	}
	action := "/leads"
	for _, id := range ids {
		uid := id
		lang := "tr"
		if u, err := a.q.GetUserByID(ctx, uid); err == nil && u.Locale == "en" {
			lang = "en"
		}
		text, ok := quotaTexts[lang][alert.Feature]
		if !ok {
			text = quotaText{feature: alert.Feature}
		}
		resets := alert.ResetsAt.Format("02.01.2006")
		if lang == "en" {
			resets = alert.ResetsAt.Format("January 2, 2006")
		}
		if _, err := a.svc.Enqueue(ctx, model.EnqueueInput{
			UserID:       &uid,
			AccountID:    &alert.AccountID,
			Language:     lang,
			Channels:     []string{model.ChannelInapp, model.ChannelRealtime, model.ChannelEmail},
			Priority:     model.PriorityHigh,
			TemplateCode: template,
			TemplateVars: map[string]string{
				"feature": text.feature, "after": text.after, "after_reached": text.afterReached,
				"used": strconv.FormatInt(alert.Used, 10), "limit": strconv.Itoa(int(alert.Limit)),
				"percent": strconv.Itoa(alert.Percent), "account": account, "resets": resets,
			},
			ActionURL: &action,
			Payload: map[string]any{
				"account_id": alert.AccountID, "feature": alert.Feature, "used": alert.Used,
				"limit": alert.Limit, "percent": alert.Percent,
			},
			SourceEvent: fmt.Sprintf("entitlements.quota:%d:%s:%s:%d", alert.AccountID, alert.Feature,
				alert.ResetsAt.Format("2006-01"), alert.Percent),
		}); err != nil {
			a.log.Warn("quota_alert_failed", "error", err, "feature", alert.Feature)
		}
	}
}

var _ entitlement.UsageAlerter = (*QuotaAlerter)(nil)
