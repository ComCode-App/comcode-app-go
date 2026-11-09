package usecase

import (
	"context"
	"slices"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

func TestIdentityChanges(t *testing.T) {
	before := &model.ProfileDetail{Translations: []model.ProfileTranslationDTO{
		{Locale: "tr", FirstName: "Ayşe", LastName: "YILMAZ", CompanyName: "ComCode", Title: "CTO"},
	}}
	tr := func(first, last, company, title string) model.UpsertTranslationInput {
		return model.UpsertTranslationInput{Locale: "tr", FirstName: first, LastName: last, CompanyName: company, Title: title}
	}
	cases := []struct {
		name string
		in   []model.UpsertTranslationInput
		want []string
	}{
		{"title and casing only", []model.UpsertTranslationInput{tr(" ayşe ", "Yılmaz", "comcode", "CEO")}, nil},
		{"same name in a new locale", []model.UpsertTranslationInput{tr("Ayşe", "Yılmaz", "ComCode", ""), {Locale: "en", FirstName: "Ayşe", LastName: "Yılmaz"}}, nil},
		{"renamed", []model.UpsertTranslationInput{tr("Ayşe", "Kaya", "ComCode", "CTO")}, []string{reasonDisplayNameChanged}},
		{"middle name added", []model.UpsertTranslationInput{{Locale: "tr", FirstName: "Ayşe", MiddleName: "Nur", LastName: "Yılmaz", CompanyName: "ComCode"}}, []string{reasonDisplayNameChanged}},
		{"organization changed", []model.UpsertTranslationInput{tr("Ayşe", "Yılmaz", "Other", "CTO")}, []string{reasonOrganizationChanged}},
		{"organization removed", []model.UpsertTranslationInput{tr("Ayşe", "Yılmaz", "", "CTO")}, []string{reasonOrganizationChanged}},
		{"both", []model.UpsertTranslationInput{tr("Ali", "Yılmaz", "Other", "CTO")}, []string{reasonDisplayNameChanged, reasonOrganizationChanged}},
	}
	for _, c := range cases {
		got := identityChanges(before, model.UpsertProfileInput{Translations: c.in})
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

type fakeVerifications struct {
	status  string
	revoked [][]string
}

func (f *fakeVerifications) Status(context.Context, int64, uuid.UUID) (string, error) {
	return f.status, nil
}

func (f *fakeVerifications) Verified(context.Context, int64) (bool, error) {
	return f.status == "verified", nil
}

func (f *fakeVerifications) RevokeForIdentityChange(_ context.Context, _ int64, _ uuid.UUID, _ int64, reasons []string) error {
	f.revoked = append(f.revoked, reasons)
	return nil
}

func TestDetailCarriesSuggestionsAndVerification(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, nil)
	svc.SetQuotas(&stubQuotas{})
	ver := &fakeVerifications{status: "verified"}
	svc.SetVerifications(ver)
	p := authctx.Principal{UserInternal: 1, Account: authctx.Account{ID: 7, Role: authctx.AccountRoleOwner}}
	in := model.UpsertProfileInput{Translations: []model.UpsertTranslationInput{{Locale: "tr", FirstName: "Ada", LastName: "L"}}}
	created, err := svc.Create(context.Background(), p, in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(context.Background(), p, created.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CompletionRate != 25 || got.VerificationStatus != "verified" {
		t.Fatalf("rate %d status %q, want 25 verified", got.CompletionRate, got.VerificationStatus)
	}
	open := 0
	for _, s := range got.CompletionSuggestions {
		open += s.Points
	}
	if got.CompletionRate+open != 100 || got.CompletionSuggestions[0].Key != "avatar" {
		t.Fatalf("suggestions %+v", got.CompletionSuggestions)
	}

	in.Translations[0].CompanyName = "ComCode"
	if _, err := svc.Update(context.Background(), p, created.UUID, in); err != nil {
		t.Fatal(err)
	}
	if len(ver.revoked) != 1 || !slices.Equal(ver.revoked[0], []string{reasonOrganizationChanged}) {
		t.Fatalf("revocations = %v", ver.revoked)
	}
}
