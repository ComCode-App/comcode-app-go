package usecase

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/repository"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

// Acceptance (TEC-940): a card whose own lead form is off still attributes what comes through it.
func TestCardWithFormOffAttributes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := exchangeRepo() // c/card: lead_capture off, opens profile 3
	in := validExchange()
	in.Via = "c/card"
	if _, err := newTestService(repo, nil).Exchange(ctx, in, model.Visitor{}); err != nil {
		t.Fatal(err)
	}
	if got := repo.created[0]; got.SourceType != model.SourceCard || *got.CardID != 9 || got.RouteID != 4 {
		t.Fatalf("exchange not attributed to the card: %+v", got)
	}

	repo = exchangeRepo()
	repo.routes["ahmet"].PresentationConfig = []byte(`{"lead_capture":{"enabled":true}}`)
	lead := validInput()
	lead.Via = "c/card"
	if _, err := newTestService(repo, nil).Submit(ctx, lead, model.Visitor{}); err != nil {
		t.Fatal(err)
	}
	if got := repo.created[0]; got.SourceType != model.SourceCard || *got.CardID != 9 || got.RoutePath != "c/card" {
		t.Fatalf("profile-form lead not attributed to the card: %+v", got)
	}

	for _, via := range []string{"c/other", "c/locked", "javascript:alert(1)", ""} {
		repo = exchangeRepo()
		repo.routes["ahmet"].PresentationConfig = []byte(`{"lead_capture":{"enabled":true}}`)
		lead.Via = via
		if _, err := newTestService(repo, nil).Submit(ctx, lead, model.Visitor{}); err != nil {
			t.Fatal(err)
		}
		if got := repo.created[0]; got.SourceType != model.SourceProfile || got.RouteID != 1 {
			t.Fatalf("via %q: want the profile's own route, got %+v", via, got)
		}
	}
}

func copyService(repo *memRepo, notifier *memNotifier) *Service {
	return newTestService(repo, notifier).WithVisitorCopy("test-secret", "https://comcode.test/")
}

func copyMail(t *testing.T, n *memNotifier) notifmodel.EnqueueInput {
	t.Helper()
	for _, in := range n.sent {
		if in.TemplateCode == model.ExchangeCopyTemplate {
			return in
		}
	}
	t.Fatalf("no visitor copy queued: %+v", n.sent)
	return notifmodel.EnqueueInput{}
}

func tokenOf(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil || u.Host != "comcode.test" || u.Path != DeletePagePath {
		t.Fatalf("bad delete link %q", link)
	}
	return u.Query().Get("token")
}

func TestExchangeQueuesVisitorCopy(t *testing.T) {
	t.Parallel()
	repo := exchangeRepo()
	repo.names = map[int64]string{3: "Ahmet Kaya"}
	notifier := &memNotifier{}
	in := validExchange()
	in.Locale = "en"
	res, err := copyService(repo, notifier).Exchange(context.Background(), in, model.Visitor{})
	if err != nil {
		t.Fatal(err)
	}
	got := copyMail(t, notifier)
	if got.UserID != nil || *got.Recipient != "ayse@example.com" || got.Language != "en" ||
		len(got.Channels) != 1 || got.Channels[0] != notifmodel.ChannelEmail {
		t.Fatalf("wrong recipient or channel: %+v", got)
	}
	if got.SourceEvent != model.ExchangeCopyEvent(res.UUID) {
		t.Fatalf("source event %q does not name the exchange", got.SourceEvent)
	}
	vars := got.TemplateVars
	if vars["owner"] != "Ahmet Kaya" || vars["name"] != "Ayşe Yılmaz" ||
		!strings.Contains(vars["summary"], "Email: ayse@example.com") || !strings.Contains(vars["summary"], "Company: Acme") ||
		strings.Contains(vars["summary"], "Phone") {
		t.Fatalf("wrong summary: %+v", vars)
	}
	if got.Payload["vcard_username"] != "ahmet" || got.Payload["vcard_locale"] != "en" {
		t.Fatalf("vCard source missing: %+v", got.Payload)
	}
	if tokenOf(t, vars["delete_url"]) == "" {
		t.Fatal("delete link has no token")
	}

	// No email address, or the copy switched off: nothing queued.
	for name, svc := range map[string]*Service{
		"phone only": copyService(exchangeRepo(), notifier),
		"off":        newTestService(exchangeRepo(), notifier),
	} {
		notifier.sent = nil
		in := validExchange()
		if name == "phone only" {
			in.Email, in.Phone = "", "+90 532 111 22 33"
		}
		if _, err := svc.Exchange(context.Background(), in, model.Visitor{}); err != nil {
			t.Fatal(err)
		}
		for _, n := range notifier.sent {
			if n.TemplateCode == model.ExchangeCopyTemplate {
				t.Fatalf("%s: copy queued", name)
			}
		}
	}
}

func TestShareProfileCopyGoesToTheSignedInVisitor(t *testing.T) {
	t.Parallel()
	repo := exchangeRepo()
	colleague := uuid.New()
	repo.senders[colleague] = memSender{accountID: 20, profile: repository.SenderProfile{
		ID: 51, UserID: int64p(201), FirstName: "Ece", Email: "ece@company.test",
	}}
	notifier := &memNotifier{}
	admin := authctx.Principal{UserInternal: 202, Email: "admin@company.test", Account: authctx.Account{ID: 20, Role: authctx.AccountRoleAdmin}}
	_, err := copyService(repo, notifier).ShareProfile(context.Background(), admin, model.ShareProfileInput{
		Path: "ahmet", ProfileUUID: colleague, Consent: true, ConsentVersion: model.ExchangeConsentVersion,
	}, model.Visitor{})
	if err != nil {
		t.Fatal(err)
	}
	if got := copyMail(t, notifier); *got.Recipient != "admin@company.test" || got.TemplateVars["owner"] != "/ahmet" {
		t.Fatalf("copy to %q (owner %q)", *got.Recipient, got.TemplateVars["owner"])
	}
}

// Acceptance (TEC-940): the delete link deletes the exchange and cannot be used again.
func TestDeleteLinkWorksOnce(t *testing.T) {
	t.Parallel()
	repo := exchangeRepo()
	notifier := &memNotifier{}
	svc := copyService(repo, notifier)
	ctx := context.Background()
	res, err := svc.Exchange(ctx, validExchange(), model.Visitor{})
	if err != nil {
		t.Fatal(err)
	}
	token := tokenOf(t, copyMail(t, notifier).TemplateVars["delete_url"])

	if err := svc.DeleteByToken(ctx, model.DeleteExchangeInput{Token: token}); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.leads[res.UUID]; ok || len(repo.deleted) != 1 || repo.deleted[0] != res.UUID {
		t.Fatalf("exchange not deleted: %v", repo.deleted)
	}
	if err := svc.DeleteByToken(ctx, model.DeleteExchangeInput{Token: token}); !errors.Is(err, ErrExchangeGone) {
		t.Fatalf("second use: want ErrExchangeGone, got %v", err)
	}
}

func TestDeleteTokenRejectsForgeries(t *testing.T) {
	t.Parallel()
	repo := exchangeRepo()
	svc := copyService(repo, nil)
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	id := uuid.New()
	repo.leads[id] = 7
	valid := svc.DeleteToken(id, now.Add(time.Hour))
	parts := strings.Split(valid, ".")

	cases := map[string]string{
		"expired":        svc.DeleteToken(id, now.Add(-time.Second)),
		"other exchange": uuid.NewString() + "." + parts[1] + "." + parts[2],
		"longer expiry":  parts[0] + ".9999999999." + parts[2],
		"other secret":   copyService(exchangeRepo(), nil).WithVisitorCopy("other", "").DeleteToken(id, now.Add(time.Hour)),
		"garbage":        "not-a-token",
		"empty":          "",
	}
	for name, token := range cases {
		if err := svc.DeleteByToken(context.Background(), model.DeleteExchangeInput{Token: token}); !errors.Is(err, ErrInvalidDeleteToken) {
			t.Fatalf("%s: want ErrInvalidDeleteToken, got %v", name, err)
		}
	}
	if len(repo.deleted) != 0 {
		t.Fatal("a forged token deleted an exchange")
	}
	if err := newTestService(repo, nil).DeleteByToken(context.Background(), model.DeleteExchangeInput{Token: valid}); !errors.Is(err, ErrInvalidDeleteToken) {
		t.Fatalf("copy off: want ErrInvalidDeleteToken, got %v", err)
	}
}

type memVCards struct {
	byName map[string]string // username -> vCard; "->x" redirects to x
	err    error
}

func (m memVCards) PublicVCard(_ context.Context, username, _ string) (string, string, string, error) {
	if m.err != nil {
		return "", "", "", m.err
	}
	v := m.byName[username]
	if strings.HasPrefix(v, "->") {
		return "", "", strings.TrimPrefix(v, "->"), nil
	}
	return v, username + ".vcf", "", nil
}

func TestCopyAttachments(t *testing.T) {
	t.Parallel()
	gone := errors.New("gone")
	ctx := context.Background()
	payload := map[string]any{"vcard_username": "old", "vcard_locale": "tr"}
	c := &CopyAttachments{VCards: memVCards{byName: map[string]string{"old": "->new", "new": "BEGIN:VCARD"}}, Gone: gone}

	files, err := c.Attachments(ctx, model.ExchangeCopyTemplate, payload)
	if err != nil || len(files) != 1 || string(files[0].Data) != "BEGIN:VCARD" || files[0].Filename != "new.vcf" ||
		!strings.HasPrefix(files[0].ContentType, "text/vcard") {
		t.Fatalf("renamed profile: %+v %v", files, err)
	}
	if files, err := c.Attachments(ctx, "auth.password_reset", payload); files != nil || err != nil {
		t.Fatalf("other templates get no attachment: %+v %v", files, err)
	}
	c.VCards = memVCards{err: gone}
	if files, err := c.Attachments(ctx, model.ExchangeCopyTemplate, payload); files != nil || err != nil {
		t.Fatalf("gone page sends without the card: %+v %v", files, err)
	}
	c.VCards = memVCards{err: errors.New("db down")}
	if _, err := c.Attachments(ctx, model.ExchangeCopyTemplate, payload); err == nil {
		t.Fatal("transient errors must retry the delivery")
	}
	if files, err := (&CopyAttachments{}).Attachments(ctx, model.ExchangeCopyTemplate, payload); files != nil || err != nil {
		t.Fatalf("unwired resolver: %+v %v", files, err)
	}
}
