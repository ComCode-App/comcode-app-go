package model

import "time"

// BankSummary is the item in lists and dropdowns.
type BankSummary struct {
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Code         string `json:"code"`
	CountryCode  string `json:"country_code"`
	BicSwift     string `json:"bic_swift"`
	IconURL      string `json:"icon_url"`
	BrandColor   string `json:"brand_color"`
	IBANPattern  string `json:"iban_pattern"`
	Description  string `json:"description"`
	DisplayOrder int    `json:"display_order"`
	IsActive     bool   `json:"is_active"`
}

// BankDetail is the full bank resource payload.
type BankDetail struct {
	ID           int64     `json:"-"`
	UUID         string    `json:"uuid"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Code         string    `json:"code"`
	CountryCode  string    `json:"country_code"`
	BicSwift     string    `json:"bic_swift"`
	IconURL      string    `json:"icon_url"`
	BrandColor   string    `json:"brand_color"`
	IBANPattern  string    `json:"iban_pattern"`
	Description  string    `json:"description"`
	DisplayOrder int       `json:"display_order"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UpsertBankInput is used to create or update a bank.
type UpsertBankInput struct {
	Name         string `json:"name"`
	Slug         string `json:"slug,omitempty"`
	Code         string `json:"code"`
	CountryCode  string `json:"country_code"`
	BicSwift     string `json:"bic_swift,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	BrandColor   string `json:"brand_color,omitempty"`
	IBANPattern  string `json:"iban_pattern,omitempty"`
	Description  string `json:"description,omitempty"`
	DisplayOrder int    `json:"display_order"`
	IsActive     bool   `json:"is_active"`
}
