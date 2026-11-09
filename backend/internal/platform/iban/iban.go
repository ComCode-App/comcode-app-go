package iban

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrEmptyIBAN       = errors.New("iban cannot be empty")
	ErrInvalidLength   = errors.New("iban length is invalid")
	ErrInvalidFormat   = errors.New("iban contains invalid characters")
	ErrInvalidChecksum = errors.New("iban checksum verification failed")
	ErrCountryNotMatch = errors.New("iban country length does not match standard")
)

// Standard IBAN lengths per country code (ISO 13616)
var countryLengths = map[string]int{
	"AL": 28, "AD": 24, "AT": 20, "AZ": 28, "BH": 22, "BY": 28, "BE": 16,
	"BA": 20, "BR": 29, "BG": 22, "CR": 22, "HR": 21, "CY": 28, "CZ": 24,
	"DK": 18, "DO": 28, "EE": 20, "FO": 18, "FI": 18, "FR": 27, "GE": 22,
	"DE": 22, "GI": 23, "GR": 27, "GL": 18, "GT": 28, "HU": 28, "IS": 26,
	"IE": 22, "IL": 23, "IT": 27, "JO": 30, "KZ": 20, "XK": 20, "KW": 30,
	"LV": 21, "LB": 28, "LI": 21, "LT": 20, "LU": 20, "MK": 19, "MT": 31,
	"MR": 27, "MU": 30, "MD": 24, "MC": 27, "ME": 22, "NL": 18, "NO": 15,
	"PK": 24, "PS": 29, "PL": 28, "PT": 25, "QA": 29, "RO": 24, "SM": 27,
	"SA": 24, "RS": 22, "SK": 24, "SI": 19, "ES": 24, "SE": 24, "CH": 21,
	"TN": 24, "TR": 26, "AE": 23, "GB": 22, "VA": 22, "VG": 24,
}

var nonAlphaNumRegex = regexp.MustCompile(`[^a-zA-Z0-9]`)
var ibanBasicPattern = regexp.MustCompile(`^[A-Z]{2}[0-9]{2}[A-Z0-9]{11,30}$`)

// Normalize removes spaces, dashes, special characters and converts to uppercase.
func Normalize(raw string) string {
	cleaned := nonAlphaNumRegex.ReplaceAllString(raw, "")
	return strings.ToUpper(cleaned)
}

// Format returns IBAN formatted in 4-character blocks separated by spaces.
func Format(raw string) string {
	norm := Normalize(raw)
	if len(norm) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, r := range norm {
		if i > 0 && i%4 == 0 {
			sb.WriteRune(' ')
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// ExtractCountryCode returns the 2-letter ISO country code.
func ExtractCountryCode(raw string) string {
	norm := Normalize(raw)
	if len(norm) < 2 {
		return ""
	}
	return norm[:2]
}

// ExtractBankCode extracts the bank identification code from the IBAN.
// For Turkish IBANs (TR), bank code is characters 5-9 (index 4 to 9).
// For other countries, appropriate heuristics or prefixes are returned.
func ExtractBankCode(raw string) string {
	norm := Normalize(raw)
	if len(norm) < 9 {
		return ""
	}

	country := norm[:2]
	switch country {
	case "TR":
		// TR22 0006 4... -> 00064
		return norm[4:9]
	case "DE":
		// DE89 3704 0044... -> 37040044 (BLZ: 8 digits)
		if len(norm) >= 12 {
			return norm[4:12]
		}
	case "GB":
		// GB29 NWBK 6016... -> NWBK
		if len(norm) >= 8 {
			return norm[4:8]
		}
	}

	// Default fallback: first 4-5 chars of BBAN
	if len(norm) >= 8 {
		return norm[4:8]
	}
	return ""
}

// ExtractAccountNumber extracts the account number portion from the IBAN.
func ExtractAccountNumber(raw string) string {
	norm := Normalize(raw)
	country := ExtractCountryCode(norm)

	switch country {
	case "TR":
		// TR IBAN: TR (2) + Check (2) + BankCode (5) + Reserved (1) + AccountNumber (16) = 26
		if len(norm) == 26 {
			// Trim leading zeros in account number if desirable, or return full 16 digits
			return norm[10:]
		}
	case "DE":
		if len(norm) == 22 {
			return norm[12:]
		}
	case "GB":
		if len(norm) == 22 {
			return norm[14:]
		}
	}

	if len(norm) > 8 {
		return norm[8:]
	}
	return norm
}

// Validate verifies whether the given string is a valid ISO 13616 IBAN using Mod-97 checksum algorithm.
func Validate(raw string) error {
	norm := Normalize(raw)
	if len(norm) == 0 {
		return ErrEmptyIBAN
	}

	if len(norm) < 15 || len(norm) > 34 {
		return ErrInvalidLength
	}

	if !ibanBasicPattern.MatchString(norm) {
		return ErrInvalidFormat
	}

	country := norm[:2]
	if expectedLen, ok := countryLengths[country]; ok {
		if len(norm) != expectedLen {
			return fmt.Errorf("%w: expected %d for %s, got %d", ErrCountryNotMatch, expectedLen, country, len(norm))
		}
	}

	// Move the first 4 characters to the end
	rearranged := norm[4:] + norm[:4]

	// Convert letters to numbers (A=10, B=11, ..., Z=35) and compute Modulo 97
	if !checkMod97(rearranged) {
		return ErrInvalidChecksum
	}

	return nil
}

// CalculateCheckDigits calculates the 2-digit checksum for a given country code and BBAN.
func CalculateCheckDigits(country string, bban string) (string, error) {
	country = strings.ToUpper(country)
	bban = nonAlphaNumRegex.ReplaceAllString(bban, "")
	if len(country) != 2 {
		return "", errors.New("country code must be 2 letters")
	}

	prep := bban + country + "00"
	remainder := 0
	for _, ch := range prep {
		var val string
		if ch >= '0' && ch <= '9' {
			val = string(ch)
		} else if ch >= 'A' && ch <= 'Z' {
			val = strconv.Itoa(int(ch-'A') + 10)
		} else {
			return "", ErrInvalidFormat
		}

		for _, digitChar := range val {
			digit := int(digitChar - '0')
			remainder = (remainder*10 + digit) % 97
		}
	}

	checksum := 98 - remainder
	return fmt.Sprintf("%02d", checksum), nil
}

func checkMod97(s string) bool {
	remainder := 0
	for _, ch := range s {
		var val string
		if ch >= '0' && ch <= '9' {
			val = string(ch)
		} else if ch >= 'A' && ch <= 'Z' {
			val = strconv.Itoa(int(ch-'A') + 10)
		} else {
			return false
		}

		for _, digitChar := range val {
			digit := int(digitChar - '0')
			remainder = (remainder*10 + digit) % 97
		}
	}

	return remainder == 1
}
