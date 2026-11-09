package ioengine

import (
	"bytes"
	"testing"
	"time"
)

func TestFormatMoney(t *testing.T) {
	cases := []struct {
		minor  int64
		locale string
		want   string
	}{
		{99900, "tr", "999,00 TRY"},
		{999000, "tr", "9.990,00 TRY"},
		{123456789, "en", "1,234,567.89 TRY"},
		{5, "tr", "0,05 TRY"},
		{-150000, "en", "-1,500.00 TRY"},
	}
	for _, c := range cases {
		if got := FormatMoney(c.minor, "TRY", c.locale); got != c.want {
			t.Errorf("FormatMoney(%d, %s) = %q, want %q", c.minor, c.locale, got, c.want)
		}
	}
}

func TestEncodeReceiptPDF(t *testing.T) {
	lh := &Letterhead{CompanyName: "ComCode", Address: "İstanbul", FooterText: "comcode.app", PrimaryColor: "#D81B8C"}
	doc := ReceiptDocument{
		Title:    "Ödeme Makbuzu",
		Number:   "RCP-3F2A9C1B7D4E",
		IssuedAt: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC),
		Lines: []ReceiptLine{
			{Label: "Hesap", Value: "Örnek Şirket"},
			{Label: "Açıklama", Value: "Abonelik yenileme · Pro (aylık)"},
			{Label: "Boş", Value: ""},
		},
		AmountLabel: "Ödenen tutar",
		Amount:      FormatMoney(99900, "TRY", "tr"),
		Note:        "Bu belge fatura yerine geçmez.",
	}
	out, err := EncodeReceiptPDF(doc, "tr", lh)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("not a PDF: %q", out[:8])
	}
	if _, err := EncodeReceiptPDF(doc, "en", nil); err != nil {
		t.Fatalf("without letterhead: %v", err)
	}
}
