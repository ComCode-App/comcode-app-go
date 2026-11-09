package ioengine

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/jung-kurt/gofpdf/v2"
)

// ReceiptLine is one labelled value on a payment receipt (label already translated).
type ReceiptLine struct {
	Label string
	Value string
}

// ReceiptDocument is a one-page payment receipt (TEC-590): the letterhead, a title, a reference
// block and a single amount line. It is not an invoice; Note says so on the page.
type ReceiptDocument struct {
	Title       string
	Number      string
	IssuedAt    time.Time
	Lines       []ReceiptLine
	AmountLabel string
	Amount      string
	Note        string
}

// EncodeReceiptPDF renders a letterheaded A4 payment receipt.
func EncodeReceiptPDF(doc ReceiptDocument, locale string, lh *Letterhead) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 18)
	if err := registerPDFFonts(pdf); err != nil {
		return nil, err
	}
	pdf.AddPage()
	loc := i18n.Normalize(locale)

	y := 12.0
	if lh != nil {
		y = drawPDFLetterhead(pdf, lh, y)
	}
	r, g, b := parseRGB(lh, true)
	pdf.SetDrawColor(r, g, b)
	pdf.SetLineWidth(0.6)
	pdf.Line(10, y, 10+pdfContentWidth, y)
	y += 6

	setPDFFont(pdf, "B", 16)
	pdf.SetXY(10, y)
	pdf.Cell(pdfContentWidth, 9, doc.Title)
	y += 10
	setPDFFont(pdf, "", 9)
	pdf.SetXY(10, y)
	pdf.Cell(pdfContentWidth/2, 5, i18n.Translate(loc, "receipt.number")+": "+doc.Number)
	pdf.SetXY(10+pdfContentWidth/2, y)
	pdf.CellFormat(pdfContentWidth/2, 5, i18n.Translate(loc, "receipt.issued_at")+": "+formatDatetime(doc.IssuedAt, loc), "", 0, "R", false, 0, "")
	y += 10

	// Reference block: label column + value column, one row per line.
	labelW := 55.0
	valueW := pdfContentWidth - labelW
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetLineWidth(0.2)
	for _, line := range doc.Lines {
		if strings.TrimSpace(line.Value) == "" {
			continue
		}
		setPDFFont(pdf, "B", 9)
		h := pdfMultiCellHeight(pdf, valueW-2, 5, line.Value)
		if h < 7 {
			h = 7
		}
		pdf.SetXY(10, y)
		pdf.CellFormat(labelW, h, line.Label, "B", 0, "L", false, 0, "")
		setPDFFont(pdf, "", 9)
		pdf.SetXY(10+labelW, y)
		pdf.MultiCell(valueW, 5, line.Value, "B", "L", false)
		y += h
	}
	y += 6

	// Amount line.
	pdf.SetFillColor(r, g, b)
	pdf.SetTextColor(255, 255, 255)
	setPDFFont(pdf, "B", 11)
	pdf.SetXY(10, y)
	pdf.CellFormat(labelW, 10, doc.AmountLabel, "", 0, "L", true, 0, "")
	pdf.SetXY(10+labelW, y)
	pdf.CellFormat(valueW, 10, doc.Amount, "", 0, "R", true, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 16

	if doc.Note != "" {
		setPDFFont(pdf, "I", 8)
		pdf.SetXY(10, y)
		pdf.MultiCell(pdfContentWidth, 4.5, doc.Note, "", "L", false)
	}

	if lh != nil && lh.FooterText != "" {
		pdf.SetY(-12)
		setPDFFont(pdf, "I", 8)
		pdf.Cell(pdfContentWidth, 5, lh.FooterText)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("receipt pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// FormatMoney renders a minor-unit amount with the locale's separators ("1.234,56 TRY").
func FormatMoney(amountMinor int64, currency string, locale string) string {
	loc := i18n.Normalize(locale)
	major := amountMinor / 100
	minor := amountMinor % 100
	if minor < 0 {
		minor = -minor
	}
	digits := fmt.Sprintf("%d", major)
	neg := strings.HasPrefix(digits, "-")
	digits = strings.TrimPrefix(digits, "-")
	var grouped strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			if loc == i18n.LocaleEN {
				grouped.WriteByte(',')
			} else {
				grouped.WriteByte('.')
			}
		}
		grouped.WriteRune(c)
	}
	dec := ","
	if loc == i18n.LocaleEN {
		dec = "."
	}
	out := fmt.Sprintf("%s%s%02d %s", grouped.String(), dec, minor, currency)
	if neg {
		out = "-" + out
	}
	return out
}
