-- Ownership checks for account-scoped analytics queries (subject must belong to the account).

-- name: AccountOwnsRoute :one
SELECT EXISTS (
    SELECT 1 FROM routes
    WHERE uuid = sqlc.arg(uuid) AND account_id = sqlc.arg(account_id)::bigint AND deleted_at IS NULL
);

-- name: AccountOwnsCard :one
SELECT EXISTS (
    SELECT 1 FROM nfc_cards
    WHERE uuid = sqlc.arg(uuid) AND account_id = sqlc.arg(account_id)::bigint AND deleted_at IS NULL
);

-- name: AccountOwnsProfile :one
SELECT EXISTS (
    SELECT 1 FROM profiles
    WHERE uuid = sqlc.arg(uuid) AND account_id = sqlc.arg(account_id)::bigint AND deleted_at IS NULL
);
