package usecase

import "testing"

func TestCardTypeForProduct(t *testing.T) {
	cases := map[string]string{
		"badge":         CardTypeBadge,
		"physical_card": CardTypeNFC,
		"":              CardTypeNFC,
		"digital":       CardTypeNFC,
	}
	for productType, want := range cases {
		if got := cardTypeForProduct(productType); got != want {
			t.Errorf("cardTypeForProduct(%q) = %q, want %q", productType, got, want)
		}
	}
}
