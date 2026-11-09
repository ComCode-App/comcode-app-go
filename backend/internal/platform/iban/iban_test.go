package iban_test

import (
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/iban"
)

func TestCalculateCheckDigits(t *testing.T) {
	bban := "0006400000112345678901" // 22 chars
	check, err := iban.CalculateCheckDigits("TR", bban)
	if err != nil {
		t.Fatalf("CalculateCheckDigits error: %v", err)
	}

	fullIBAN := "TR" + check + bban
	if err := iban.Validate(fullIBAN); err != nil {
		t.Fatalf("Generated IBAN %s failed validation: %v", fullIBAN, err)
	}
}

func TestIBANNormalizationAndFormatting(t *testing.T) {
	raw := "tr33-0006 4000 0011-2345 6789 01"
	expectedNorm := "TR330006400000112345678901"
	expectedFormatted := "TR33 0006 4000 0011 2345 6789 01"

	norm := iban.Normalize(raw)
	if norm != expectedNorm {
		t.Fatalf("Normalize() = %s, expected %s", norm, expectedNorm)
	}

	formatted := iban.Format(raw)
	if formatted != expectedFormatted {
		t.Fatalf("Format() = %s, expected %s", formatted, expectedFormatted)
	}
}

func TestExtractBankAndCountryCode(t *testing.T) {
	// TR IBAN (Is Bankasi: 00064)
	trIBAN := "TR330006400000112345678901"
	if iban.ExtractCountryCode(trIBAN) != "TR" {
		t.Errorf("ExtractCountryCode() = %s, expected TR", iban.ExtractCountryCode(trIBAN))
	}
	if iban.ExtractBankCode(trIBAN) != "00064" {
		t.Errorf("ExtractBankCode() = %s, expected 00064", iban.ExtractBankCode(trIBAN))
	}

	// DE IBAN (BLZ: 37040044)
	deIBAN := "DE89370400440532013000"
	if iban.ExtractCountryCode(deIBAN) != "DE" {
		t.Errorf("ExtractCountryCode() = %s, expected DE", iban.ExtractCountryCode(deIBAN))
	}
	if iban.ExtractBankCode(deIBAN) != "37040044" {
		t.Errorf("ExtractBankCode() = %s, expected 37040044", iban.ExtractBankCode(deIBAN))
	}
}

func TestIBANValidation(t *testing.T) {
	// Generate a valid TR IBAN dynamically to test against known calculation
	check, _ := iban.CalculateCheckDigits("TR", "0006400000112345678901")
	validTRIBAN := "TR" + check + "0006400000112345678901"

	tests := []struct {
		name    string
		iban    string
		wantErr bool
	}{
		{
			name:    "Valid TR IBAN",
			iban:    validTRIBAN,
			wantErr: false,
		},
		{
			name:    "Valid DE IBAN",
			iban:    "DE89370400440532013000",
			wantErr: false,
		},
		{
			name:    "Valid GB IBAN",
			iban:    "GB29NWBK60161331926819",
			wantErr: false,
		},
		{
			name:    "Empty IBAN",
			iban:    "",
			wantErr: true,
		},
		{
			name:    "Invalid checksum",
			iban:    validTRIBAN[:len(validTRIBAN)-1] + "X",
			wantErr: true,
		},
		{
			name:    "Invalid length for TR",
			iban:    validTRIBAN[:25],
			wantErr: true,
		},
		{
			name:    "Invalid characters",
			iban:    validTRIBAN[:25] + "!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iban.Validate(tt.iban)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%q) error = %v, wantErr %v", tt.iban, err, tt.wantErr)
			}
		})
	}
}
