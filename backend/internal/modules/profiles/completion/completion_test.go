package completion

import (
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
)

func keys(s []Suggestion) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[i] = v.Key
	}
	return out
}

func sumPoints(s []Suggestion) int {
	n := 0
	for _, v := range s {
		n += v.Points
	}
	return n
}

func TestEvaluateEmptyProfile(t *testing.T) {
	res := Evaluate(Fields{})
	if res.Rate != 0 {
		t.Fatalf("rate = %d, want 0", res.Rate)
	}
	want := []string{KeyName, KeyAvatar, KeyTitle, KeyCompany, KeyPhone, KeyEmail, KeyAddress, KeySocial}
	got := keys(res.Suggestions)
	if len(got) != len(want) {
		t.Fatalf("suggestions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("suggestions = %v, want %v (highest gain first, rule order on ties)", got, want)
		}
	}
	if sumPoints(res.Suggestions) != 100 {
		t.Fatalf("suggestion points = %d, want 100", sumPoints(res.Suggestions))
	}
}

func TestEvaluateCompleteProfile(t *testing.T) {
	in := model.UpsertProfileInput{
		AvatarURL: "/api/v1/storage/stream/profiles/x/avatar.jpg",
		Translations: []model.UpsertTranslationInput{
			{Locale: "tr", FirstName: "Ada", LastName: "Lovelace", Title: "CTO", CompanyName: "ComCode"},
		},
		ContactDetails: []model.CreateContactDetailInput{
			{Category: "whatsapp", Value: "+905551112233"},
			{Category: "email", Value: "ada@example.com"},
		},
		Addresses:   []model.CreateAddressInput{{City: "İstanbul"}},
		SocialLinks: []model.CreateSocialLinkInput{{Platform: "linkedin", URL: "https://linkedin.com/in/ada"}},
	}
	res := Evaluate(FromInput(in))
	if res.Rate != 100 || len(res.Suggestions) != 0 {
		t.Fatalf("got rate %d suggestions %v, want 100 and none", res.Rate, keys(res.Suggestions))
	}
}

func TestPartialNameSuggestsRemainingPoints(t *testing.T) {
	res := Evaluate(Fields{Names: [][2]string{{"Ada", ""}}})
	if res.Rate != 15 {
		t.Fatalf("rate = %d, want 15", res.Rate)
	}
	var name *Suggestion
	for i := range res.Suggestions {
		if res.Suggestions[i].Key == KeyName {
			name = &res.Suggestions[i]
		}
	}
	if name == nil || name.Points != 10 || name.Section != SectionIdentity || name.I18nKey != "profiles.completion.suggestions.name" {
		t.Fatalf("name suggestion = %+v, want 10 points in identity", name)
	}
	if res.Rate+sumPoints(res.Suggestions) != 100 {
		t.Fatalf("rate + open points = %d, want 100", res.Rate+sumPoints(res.Suggestions))
	}
}

func TestEmptyValuesDoNotCount(t *testing.T) {
	in := model.UpsertProfileInput{
		AvatarURL:      "   ",
		Translations:   []model.UpsertTranslationInput{{FirstName: " ", Title: " "}},
		ContactDetails: []model.CreateContactDetailInput{{Category: "phone", Value: "  "}, {Category: "url", Value: "https://x.test"}},
		Addresses:      []model.CreateAddressInput{{Street: "Main st"}, {City: "Ankara"}},
	}
	res := Evaluate(FromInput(in))
	if res.Rate != 0 {
		t.Fatalf("rate = %d, want 0 (blank values, url contact and city only on a later address do not count)", res.Rate)
	}
}

func TestFromDetailMatchesFromInput(t *testing.T) {
	in := model.UpsertProfileInput{
		Translations:   []model.UpsertTranslationInput{{FirstName: "Ada", LastName: "L", CompanyName: "ComCode"}},
		ContactDetails: []model.CreateContactDetailInput{{Category: "email", Value: "a@b.c"}},
	}
	d := &model.ProfileDetail{
		Translations:   []model.ProfileTranslationDTO{{FirstName: "Ada", LastName: "L", CompanyName: "ComCode"}},
		ContactDetails: []model.ContactDetailDTO{{Category: "email", Value: "a@b.c"}},
	}
	a, b := Evaluate(FromInput(in)), Evaluate(FromDetail(d))
	if a.Rate != b.Rate || len(a.Suggestions) != len(b.Suggestions) {
		t.Fatalf("input %+v != detail %+v", a, b)
	}
	if a.Rate != 45 {
		t.Fatalf("rate = %d, want 45 (name 25 + company 10 + email 10)", a.Rate)
	}
	if got := Evaluate(FromDetail(nil)); got.Rate != 0 {
		t.Fatalf("nil detail rate = %d", got.Rate)
	}
}
