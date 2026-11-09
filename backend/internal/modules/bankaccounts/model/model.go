package model

import "time"

// UserBankAccountSummary represents a bank account record owned by a user.
type UserBankAccountSummary struct {
	UUID           string    `json:"uuid"`
	BankID         *int64    `json:"-"`
	BankUUID       *string   `json:"bank_uuid,omitempty"`
	BankName       string    `json:"bank_name,omitempty"`
	BankSlug       string    `json:"bank_slug,omitempty"`
	BankCode       string    `json:"bank_code,omitempty"`
	BankIconURL    string    `json:"bank_icon_url,omitempty"`
	BankBrandColor string    `json:"bank_brand_color,omitempty"`
	BankBicSwift   string    `json:"bank_bic_swift,omitempty"`
	AccountHolder  string    `json:"account_holder"`
	IBAN           string    `json:"iban"`
	FormattedIBAN  string    `json:"formatted_iban"`
	AccountNumber  string    `json:"account_number"`
	BranchCode     string    `json:"branch_code"`
	Currency       string    `json:"currency"`
	Title          string    `json:"title"`
	IsPrimary      bool      `json:"is_primary"`
	IsActive       bool      `json:"is_active"`
	DisplayOrder   int       `json:"display_order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateUserBankAccountInput is the payload to add a new bank account.
type CreateUserBankAccountInput struct {
	IBAN          string  `json:"iban"`
	AccountHolder string  `json:"account_holder"`
	BankUUID      *string `json:"bank_uuid,omitempty"`
	AccountNumber string  `json:"account_number,omitempty"`
	BranchCode    string  `json:"branch_code,omitempty"`
	Currency      string  `json:"currency,omitempty"`
	Title         string  `json:"title,omitempty"`
	IsPrimary     bool    `json:"is_primary,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
}

// UpdateUserBankAccountInput is the payload to update an existing bank account.
type UpdateUserBankAccountInput struct {
	IBAN          string  `json:"iban,omitempty"`
	AccountHolder string  `json:"account_holder,omitempty"`
	BankUUID      *string `json:"bank_uuid,omitempty"`
	AccountNumber string  `json:"account_number,omitempty"`
	BranchCode    string  `json:"branch_code,omitempty"`
	Currency      string  `json:"currency,omitempty"`
	Title         string  `json:"title,omitempty"`
	IsPrimary     *bool   `json:"is_primary,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
	DisplayOrder  *int    `json:"display_order,omitempty"`
}

// ReorderAccountsInput is used to set order of accounts.
type ReorderAccountsInput struct {
	UUIDs []string `json:"uuids"`
}
