package notifications

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/providers"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/mail"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type sentMail struct {
	mu   sync.Mutex
	msgs []mail.Message
}

func (s *sentMail) Send(_ context.Context, m mail.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs = append(s.msgs, m)
	return nil
}

// TEC-941: the account owner gets the 80% alert in the panel and by email, in Turkish by default.
func TestQuotaAlertReachesOwner(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	owner := d.CreateUser(t)
	sent := &sentMail{}
	svc := usecase.New(d.Queries, nil, []providers.Provider{providers.InappProvider{}, providers.EmailProvider{Mail: sent}}, nil)

	NewQuotaAlerter(svc, d.Queries, nil).QuotaAlert(ctx, entitlement.QuotaAlert{
		AccountID: owner.AccountID, Feature: entitlement.FeatureLeadsMonthlyMax, Used: 16, Limit: 20, Percent: 80,
		ResetsAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
	})

	var inapp int
	var body string
	if err := d.Pool.QueryRow(ctx, `SELECT COUNT(*), MAX(body) FROM notifications
		WHERE user_id=$1 AND template_code='entitlements.quota_warning' AND channel='inapp'`, owner.ID).Scan(&inapp, &body); err != nil {
		t.Fatal(err)
	}
	if inapp != 1 || !strings.Contains(body, "16/20 lead") {
		t.Fatalf("panel alert: %d %q", inapp, body)
	}
	if len(sent.msgs) != 1 || !strings.Contains(sent.msgs[0].Subject, "%80") || !strings.Contains(sent.msgs[0].Body, "01.11.2026") {
		t.Fatalf("email alert: %+v", sent.msgs)
	}
}
