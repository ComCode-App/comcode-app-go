package slug

import (
	"strings"
	"testing"
)

func TestFromName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Turkish letters fold to ASCII instead of being stripped (TEC-1111).
		{"turkish company", "Çağrı Şirketi", "cagri-sirketi"},
		{"dotted capital I upper word", "İSTANBUL Ofis", "istanbul-ofis"},
		{"umlauts and cedilla", "Güneş Ölçü", "gunes-olcu"},
		{"dotted capital I", "İzmir", "izmir"},
		{"dotless lowercase i", "Işık", "isik"},
		{"all special lower", "çğıöşü", "cgiosu"},
		{"all special upper", "ÇĞİÖŞÜ", "cgiosu"},
		{"mixed with ascii", "Gaziantep Böğürtlen", "gaziantep-bogurtlen"},

		// Other Latin diacritics keep their base letter.
		{"acute", "Café Crème", "cafe-creme"},
		{"tilde and ring", "São Ångström", "sao-angstrom"},

		// Existing ASCII behaviour is unchanged.
		{"simple", "Hello World", "hello-world"},
		{"already slug", "pro-plan", "pro-plan"},
		{"underscores", "custom_role", "custom-role"},
		{"collapses separators", "a  --  b__c", "a-b-c"},
		{"trims separators", "  -Hello-  ", "hello"},
		{"keeps digits", "Route 66", "route-66"},
		{"empty", "", "item"},
		{"only separators", "   ---   ", "item"},
		{"symbols only", "₺€", "item"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FromName(tc.in); got != tc.want {
				t.Errorf("FromName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFromNameTruncatesTo80(t *testing.T) {
	got := FromName(strings.Repeat("a", 100))
	if len(got) != 80 {
		t.Errorf("slug length = %d, want 80", len(got))
	}
	got = FromName(strings.Repeat("ş", 100))
	if got != strings.Repeat("s", 80) {
		t.Errorf("turkish truncation = %q", got)
	}
}
