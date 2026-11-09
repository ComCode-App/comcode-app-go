-- name: GetPaymentSettings :one
SELECT *
FROM payment_settings
WHERE id = 1;

-- name: UpdatePaymentSettings :one
UPDATE payment_settings
SET active_provider = COALESCE(sqlc.narg(active_provider), active_provider),
    test_mode = COALESCE(sqlc.narg(test_mode), test_mode)
WHERE id = 1
RETURNING *;

-- name: GetPaymentProviderSettings :one
SELECT *
FROM payment_provider_settings
WHERE provider = $1;

-- name: ListPaymentProviderSettings :many
SELECT *
FROM payment_provider_settings
ORDER BY provider ASC;

-- name: UpdatePaymentProviderSettings :one
UPDATE payment_provider_settings
SET enabled = COALESCE(sqlc.narg(enabled), enabled),
    merchant_id = COALESCE(sqlc.narg(merchant_id), merchant_id),
    base_url = COALESCE(sqlc.narg(base_url), base_url),
    merchant_key_enc = COALESCE(sqlc.narg(merchant_key_enc), merchant_key_enc),
    merchant_salt_enc = COALESCE(sqlc.narg(merchant_salt_enc), merchant_salt_enc),
    api_key_enc = COALESCE(sqlc.narg(api_key_enc), api_key_enc),
    secret_key_enc = COALESCE(sqlc.narg(secret_key_enc), secret_key_enc)
WHERE provider = sqlc.arg(provider)
RETURNING *;

-- name: CreateCheckoutSession :one
INSERT INTO payment_checkout_sessions (
    account_id, user_id, plan_id, provider, status, merchant_oid,
    amount_minor, currency, expires_at, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- account-lint: cross-account checkout start: session id from the row just inserted for this account
-- name: UpdateCheckoutSessionInit :one
UPDATE payment_checkout_sessions
SET provider_token = COALESCE(sqlc.narg(provider_token), provider_token),
    checkout_url = COALESCE(sqlc.narg(checkout_url), checkout_url),
    metadata = COALESCE(sqlc.narg(metadata), metadata)
WHERE uuid = sqlc.arg(uuid)
RETURNING *;

-- account-lint: cross-account usecase checks row.AccountID against the active account (404 otherwise)
-- name: GetCheckoutSessionByUUID :one
SELECT cs.*,
       p.uuid AS plan_uuid,
       p.slug AS plan_slug,
       p.name AS plan_name,
       p.billing_interval AS plan_billing_interval,
       u.uuid AS user_uuid,
       u.email AS user_email,
       u.name AS user_name,
       u.surname AS user_surname
FROM payment_checkout_sessions cs
INNER JOIN plans p ON p.id = cs.plan_id
INNER JOIN users u ON u.id = cs.user_id
WHERE cs.uuid = $1;

-- account-lint: cross-account payment provider callback: merchant oid is the provider's reference
-- name: GetCheckoutSessionByMerchantOID :one
SELECT cs.*,
       p.uuid AS plan_uuid,
       p.slug AS plan_slug,
       p.name AS plan_name,
       u.uuid AS user_uuid,
       u.email AS user_email,
       u.name AS user_name,
       u.surname AS user_surname
FROM payment_checkout_sessions cs
INNER JOIN plans p ON p.id = cs.plan_id
INNER JOIN users u ON u.id = cs.user_id
WHERE cs.merchant_oid = $1;

-- account-lint: cross-account payment provider callback: provider token is the reference
-- name: GetCheckoutSessionByProviderToken :one
SELECT cs.*,
       p.uuid AS plan_uuid,
       p.slug AS plan_slug,
       p.name AS plan_name,
       u.uuid AS user_uuid,
       u.email AS user_email,
       u.name AS user_name,
       u.surname AS user_surname
FROM payment_checkout_sessions cs
INNER JOIN plans p ON p.id = cs.plan_id
INNER JOIN users u ON u.id = cs.user_id
WHERE cs.provider_token = $1;

-- account-lint: cross-account payment provider callback: session found by merchant oid / provider token
-- name: CompleteCheckoutSession :one
UPDATE payment_checkout_sessions
SET status = 'completed',
    provider_ref = COALESCE(sqlc.narg(provider_ref), provider_ref),
    completed_at = NOW()
WHERE uuid = sqlc.arg(uuid)
  AND status = 'pending'
RETURNING *;

-- account-lint: cross-account payment provider callback / init failure: session id from that flow
-- name: FailCheckoutSession :one
UPDATE payment_checkout_sessions
SET status = 'failed',
    metadata = COALESCE(sqlc.narg(metadata), metadata)
WHERE uuid = sqlc.arg(uuid)
  AND status = 'pending'
RETURNING *;

-- name: ListActivePaidPlans :many
SELECT *
FROM plans
WHERE is_active = TRUE
  AND billing_interval <> 'free'
  AND price_minor > 0
ORDER BY sort_order ASC, name ASC;

-- name: CreatePaymentSubscription :one
INSERT INTO subscriptions (
    plan_id, status, account_id, starts_at, source, provider,
    provider_subscription_ref, checkout_session_id, next_billing_at, saved_card_id
)
VALUES ($1, $2, $3, $4, 'payment', $5, $6, $7, $8, $9)
RETURNING *;

-- account-lint: cross-account payment provider callback: session id from the verified callback
-- name: LockCheckoutSession :one
SELECT * FROM payment_checkout_sessions WHERE merchant_oid = $1 FOR UPDATE;

-- name: LockBillingAccount :one
SELECT id FROM accounts WHERE id = $1 FOR UPDATE;
