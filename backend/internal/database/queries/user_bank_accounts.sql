-- name: ListUserBankAccounts :many
SELECT uba.*,
       b.name AS bank_name,
       b.slug AS bank_slug,
       b.code AS bank_code,
       b.icon_url AS bank_icon_url,
       b.brand_color AS bank_brand_color,
       b.bic_swift AS bank_bic_swift
FROM user_bank_accounts uba
LEFT JOIN banks b ON b.id = uba.bank_id AND b.deleted_at IS NULL
WHERE uba.account_id = $1
  AND uba.deleted_at IS NULL
ORDER BY uba.is_primary DESC, uba.display_order ASC, uba.created_at DESC;

-- name: GetUserBankAccountByUUID :one
SELECT uba.*,
       b.name AS bank_name,
       b.slug AS bank_slug,
       b.code AS bank_code,
       b.icon_url AS bank_icon_url,
       b.brand_color AS bank_brand_color,
       b.bic_swift AS bank_bic_swift
FROM user_bank_accounts uba
LEFT JOIN banks b ON b.id = uba.bank_id AND b.deleted_at IS NULL
WHERE uba.uuid = $1
  AND uba.account_id = $2
  AND uba.deleted_at IS NULL;

-- name: CreateUserBankAccount :one
INSERT INTO user_bank_accounts (
    account_id,
    user_id,
    bank_id,
    account_holder,
    iban,
    account_number,
    branch_code,
    currency,
    title,
    is_primary,
    is_active,
    display_order
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: UpdateUserBankAccount :one
UPDATE user_bank_accounts
SET bank_id = COALESCE(sqlc.narg(bank_id), bank_id),
    account_holder = COALESCE(sqlc.narg(account_holder), account_holder),
    iban = COALESCE(sqlc.narg(iban), iban),
    account_number = COALESCE(sqlc.narg(account_number), account_number),
    branch_code = COALESCE(sqlc.narg(branch_code), branch_code),
    currency = COALESCE(sqlc.narg(currency), currency),
    title = COALESCE(sqlc.narg(title), title),
    is_primary = COALESCE(sqlc.narg(is_primary), is_primary),
    is_active = COALESCE(sqlc.narg(is_active), is_active),
    display_order = COALESCE(sqlc.narg(display_order), display_order)
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteUserBankAccount :exec
UPDATE user_bank_accounts
SET deleted_at = NOW()
WHERE uuid = $1
  AND account_id = $2;

-- name: ClearPrimaryUserBankAccounts :exec
UPDATE user_bank_accounts
SET is_primary = false
WHERE account_id = $1
  AND uuid != $2
  AND deleted_at IS NULL;

-- name: ToggleActiveUserBankAccount :one
UPDATE user_bank_accounts
SET is_active = NOT is_active
WHERE uuid = $1
  AND account_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserBankAccountOrder :exec
UPDATE user_bank_accounts
SET display_order = $3
WHERE uuid = $1
  AND account_id = $2
  AND deleted_at IS NULL;
