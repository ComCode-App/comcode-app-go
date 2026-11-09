package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

func validExchange() model.ExchangeInput {
	return model.ExchangeInput{
		Path: "ahmet", Name: "Ayşe Yılmaz", Email: "ayse@example.com",
		Company: "Acme", JobTitle: "CTO",
		Consent: true, ConsentVersion: model.ExchangeConsentVersion,
	}
}

// exchangeRepo: account 7 owns profile 3 at "ahmet" (exchange on), a card and a smart link that
// open it, a profile with exchange off and a card of another profile.
func exchangeRepo() *memRepo {
	on := []byte(`{"contact_exchange":{"enabled":true}}`)
	repo := newMemRepo()
	repo.routes = map[string]*repository.CaptureRoute{
		"ahmet":    {ID: 1, RouteType: "canonical_profile", Status: "active", AccountID: 7, PresentationConfig: on, ProfileID: int64p(3)},
		"closed":   {ID: 2, RouteType: "canonical_profile", Status: "active", AccountID: 7, PresentationConfig: []byte(`{"lead_capture":{"enabled":true}}`), ProfileID: int64p(4)},
		"hidden":   {ID: 3, RouteType: "canonical_profile", Status: "locked", AccountID: 7, PresentationConfig: on, ProfileID: int64p(5)},
		"c/card":   {ID: 4, RouteType: "card_route", Status: "active", AccountID: 7, ProfileID: int64p(3), CardID: int64p(9), CardIsActive: true},
		"c/locked": {ID: 5, RouteType: "card_route", Status: "active", AccountID: 7, ProfileID: int64p(3), CardID: int64p(10), CardIsActive: true, CardIsLocked: true},
		"c/other":  {ID: 6, RouteType: "card_route", Status: "active", AccountID: 7, ProfileID: int64p(4), CardID: int64p(11), CardIsActive: true},
		"s/link":   {ID: 7, RouteType: "custom_smart_link", Status: "active", AccountID: 7, ProfileID: int64p(3)},
		"s/":       nil,
	}
	repo.owners = []int64{100}
	return repo
}

func TestValidateExchange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*model.ExchangeInput)
		want   map[string]string
	}{
		{"valid", func(*model.ExchangeInput) {}, nil},
		{"phone only", func(in *model.ExchangeInput) { in.Email = ""; in.Phone = "+90 532 123 45 67" }, nil},
		{"email or phone", func(in *model.ExchangeInput) { in.Email = "" }, map[string]string{"email": "email_or_phone_required"}},
		{"company too long", func(in *model.ExchangeInput) { in.Company = strings.Repeat("a", model.MaxCompany+1) }, map[string]string{"company": "too_long"}},
		{"title control chars", func(in *model.ExchangeInput) { in.JobTitle = "a\u0007b" }, map[string]string{"job_title": "invalid"}},
		{"lead consent text", func(in *model.ExchangeInput) { in.ConsentVersion = model.ConsentVersion }, map[string]string{"consent_version": "invalid"}},
		{"consent required", func(in *model.ExchangeInput) { in.Consent = false }, map[string]string{"consent": "required"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := validExchange()
			tc.mutate(&in)
			_, err := ValidateExchange(in)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			got := fieldCodes(err)
			for field, code := range tc.want {
				if got[field] != code {
					t.Fatalf("field %s: want %q, got %v", field, code, got)
				}
			}
		})
	}
	if out, _ := ValidateExchange(model.ExchangeInput{Via: "/c/ABC/"}); out.Via != "c/ABC" {
		t.Fatalf("via not normalised: %q", out.Via)
	}
	if out, _ := ValidateExchange(model.ExchangeInput{Via: "evil/path"}); out.Via != "" {
		t.Fatalf("bad via kept: %q", out.Via)
	}
}

func TestExchangeAvailability(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]bool{"ahmet": true, "closed": false, "hidden": false, "c/card": false, "unknown": false} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			repo := exchangeRepo()
			svc := newTestService(repo, nil)
			in := validExchange()
			in.Path = path
			_, err := svc.Exchange(context.Background(), in, model.Visitor{})
			if got := err == nil; got != want {
				t.Fatalf("exchange allowed=%v, want %v (err %v)", got, want, err)
			}
			if !want && !errors.Is(err, ErrFormUnavailable) {
				t.Fatalf("want ErrFormUnavailable, got %v", err)
			}
			cfg, _ := svc.FormConfig(context.Background(), path)
			if cfg.ExchangeEnabled != want || cfg.ExchangeConsentVersion != model.ExchangeConsentVersion {
				t.Fatalf("form config disagrees: %+v", cfg)
			}
		})
	}
}

func TestExchangeStoresAndNotifies(t *testing.T) {
	t.Parallel()
	repo := exchangeRepo()
	notifier := &memNotifier{}
	svc := newTestService(repo, notifier)
	if _, err := svc.Exchange(context.Background(), validExchange(), model.Visitor{IP: "203.0.113.9"}); err != nil {
		t.Fatal(err)
	}
	got := repo.created[0]
	if got.Kind != model.KindExchange || got.AccountID != 7 || got.RouteID != 1 || *got.ProfileID != 3 ||
		got.SourceType != model.SourceProfile || got.Company != "Acme" || got.JobTitle != "CTO" ||
		got.SenderProfileID != nil || got.ConsentVersion != model.ExchangeConsentVersion {
		t.Fatalf("wrong exchange row: %+v", got)
	}
	if len(notifier.sent) != 1 || notifier.sent[0].TemplateCode != "leads.exchange_received" {
		t.Fatalf("owner not notified: %+v", notifier.sent)
	}
}

func TestExchangeViaAttribution(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		routeID int64
		source  string
		path    string
	}{
		"c/card":   {4, model.SourceCard, "c/card"},
		"s/link":   {7, model.SourceSmartLink, "s/link"},
		"c/locked": {1, model.SourceProfile, "ahmet"}, // locked card: the profile's own
		"c/other":  {1, model.SourceProfile, "ahmet"}, // opens another profile
		"c/none":   {1, model.SourceProfile, "ahmet"},
	}
	for via, want := range cases {
		t.Run(via, func(t *testing.T) {
			t.Parallel()
			repo := exchangeRepo()
			in := validExchange()
			in.Via = via
			if _, err := newTestService(repo, nil).Exchange(context.Background(), in, model.Visitor{}); err != nil {
				t.Fatal(err)
			}
			got := repo.created[0]
			if got.RouteID != want.routeID || got.SourceType != want.source || got.RoutePath != want.path {
				t.Fatalf("attribution: got route %d %s %s, want %+v", got.RouteID, got.SourceType, got.RoutePath, want)
			}
		})
	}
}

func TestShareProfile(t *testing.T) {
	t.Parallel()
	mine, colleague, foreign := uuid.New(), uuid.New(), uuid.New()
	newRepo := func() *memRepo {
		repo := exchangeRepo()
		repo.senders[mine] = memSender{accountID: 20, profile: repository.SenderProfile{
			ID: 50, UserID: int64p(200), FirstName: "Can", LastName: "Demir",
			Company: strings.Repeat("Ş", 200), Title: "Founder", Email: "can@example.com",
		}}
		repo.senders[colleague] = memSender{accountID: 20, profile: repository.SenderProfile{
			ID: 51, UserID: int64p(201), FirstName: "Ece", Phone: "+90 532 000 00 00",
		}}
		repo.senders[foreign] = memSender{accountID: 30, profile: repository.SenderProfile{ID: 52, Email: "x@example.com", FirstName: "X"}}
		return repo
	}
	member := authctx.Principal{UserInternal: 200, Account: authctx.Account{ID: 20, Role: authctx.AccountRoleMember}}
	admin := authctx.Principal{UserInternal: 202, Account: authctx.Account{ID: 20, Role: authctx.AccountRoleAdmin}}
	share := func(id uuid.UUID) model.ShareProfileInput {
		return model.ShareProfileInput{Path: "ahmet", ProfileUUID: id, Consent: true, ConsentVersion: model.ExchangeConsentVersion}
	}
	ctx := context.Background()

	repo := newRepo()
	if _, err := newTestService(repo, nil).ShareProfile(ctx, member, share(mine), model.Visitor{}); err != nil {
		t.Fatal(err)
	}
	got := repo.created[0]
	if got.Name != "Can Demir" || got.Email != "can@example.com" || got.JobTitle != "Founder" ||
		len([]rune(got.Company)) != model.MaxCompany || *got.SenderUserID != 200 || *got.SenderProfileID != 50 ||
		got.Kind != model.KindExchange {
		t.Fatalf("wrong snapshot: %+v", got)
	}

	cases := []struct {
		name string
		p    authctx.Principal
		in   model.ShareProfileInput
		want error
	}{
		{"member shares a colleague", member, share(colleague), ErrSenderProfileForbidden},
		{"profile of another account", member, share(foreign), ErrSenderProfileNotFound},
		{"unknown profile", member, share(uuid.New()), ErrSenderProfileNotFound},
		{"impersonating", authctx.Principal{UserInternal: 200, ImpersonatorUserID: &uuid.UUID{}, Account: member.Account}, share(mine), ErrImpersonating},
	}
	for _, tc := range cases {
		repo := newRepo()
		if _, err := newTestService(repo, nil).ShareProfile(ctx, tc.p, tc.in, model.Visitor{}); !errors.Is(err, tc.want) {
			t.Fatalf("%s: want %v, got %v", tc.name, tc.want, err)
		}
		if len(repo.created) != 0 {
			t.Fatalf("%s: stored an exchange", tc.name)
		}
	}

	repo = newRepo()
	if _, err := newTestService(repo, nil).ShareProfile(ctx, admin, share(colleague), model.Visitor{}); err != nil {
		t.Fatalf("admin shares a colleague's profile: %v", err)
	}
	if repo.created[0].Phone != "+90 532 000 00 00" || repo.created[0].Name != "Ece" {
		t.Fatalf("wrong snapshot: %+v", repo.created[0])
	}

	repo = newRepo()
	repo.senders[mine] = memSender{accountID: 7, profile: repository.SenderProfile{ID: 3, UserID: int64p(1), FirstName: "A", Email: "a@example.com"}}
	owner := authctx.Principal{UserInternal: 1, Account: authctx.Account{ID: 7, Role: authctx.AccountRoleOwner}}
	if _, err := newTestService(repo, nil).ShareProfile(ctx, owner, share(mine), model.Visitor{}); !errors.Is(err, ErrSelfExchange) {
		t.Fatalf("want ErrSelfExchange, got %v", err)
	}
}
