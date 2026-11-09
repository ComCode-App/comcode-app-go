package legacymigration

import (
	"fmt"
	"testing"
)

func TestOwnerClassifierReference(t *testing.T) {
	cases := []struct{ input, want string }{
		{"kww.com.tr", "kww"}, {"remax-up.com", "remax-up"}, {"gmail.com", ""},
		{"foo.sahibinden.com", ""}, {"drive.google.com", ""}, {"instagram.example", ""},
	}
	for _, tc := range cases {
		if got := ownerBrand(tc.input); got != tc.want {
			t.Errorf(
				"brand(%q)=%q want %q",
				tc.input,
				got,
				tc.want,
			)
		}
	}
	if got := normalizeOrg("Åžirket Ä°nÅŸaat Teknoloji Ltd. Sti."); got != "sirket teknoloji" {
		t.Fatalf("normalized org: %q", got)
	}
	// The reference drops sector/legal tokens, preserving meaningful franchise tokens.
	users := []ownerCandidate{}
	for i := 1; i <= 6; i++ {
		org := "Coldwell Banker Kobi Gayrimenkul"
		if i > 3 {
			org = "Coldwell Banker Life Emlak"
		}
		users = append(users, ownerSource(map[string]string{
			"user_id": fmt.Sprint(i),
			"email":   "agent@gmail.com",
			"vcard":   fmt.Sprintf(`{"workemail":"a@cb.com.tr","organization":%q}`, org),
		}))
	}
	users = append(users, ownerSource(map[string]string{"user_id": "7", "email": "org@gmail.com", "companies": "Coldwell Banker Kobi"}))
	groups := classifyOwners(users)
	if len(groups) != 2 || len(groups["b:cb/kobi"]) != 4 || len(groups["b:cb/life"]) != 3 {
		t.Fatalf("split/merge: %+v", groups)
	}
}

func TestOwnerKeyPrecedenceAndOwnerSelection(t *testing.T) {
	tests := []struct {
		row  map[string]string
		want string
	}{
		{
			map[string]string{
				"email": "a@login.com",
				"vcard": `{"workemail":"x@work.com","organization":"Acme","workurl":"https://site.com"}`,
			},
			"b:work",
		},
		{
			map[string]string{"email": "a@login.com", "vcard": `{"workemail":"x@gmail.com","organization":"Acme"}`},
			"b:login",
		},
		{map[string]string{"email": "a@gmail.com", "companies": "Acme Ltd"}, "o:acme"},
		{
			map[string]string{"email": "a@gmail.com", "companies": "AB", "vcard": `{"workurl":"https://www.site.com/path"}`},
			"b:site",
		},
		{map[string]string{"email": "a@gmail.com"}, "solo:"},
	}
	for _, tt := range tests {
		if got := ownerSource(tt.row); got.Group != tt.want {
			t.Errorf(
				"key %+v = %q want %q",
				tt.row,
				got.Group,
				tt.want,
			)
		}
	}
	users := []ownerCandidate{{Title: "Sales"}, {Title: "Genel Müdür"}, {Title: "Founder"}}
	if got := companyOwner([]int{0, 1, 2}, users); got != 1 {
		t.Fatalf("owner=%d", got)
	}
	if !legacyIDLess("2", "10") {
		t.Fatal("numeric legacy order")
	}
}
