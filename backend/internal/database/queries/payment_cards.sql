-- name: GetPaymentProviderCustomer :one
SELECT *
FROM payment_provider_customers
WHERE account_id = $1 AND provider = $2;

-- name: UpsertPaymentProviderCustomer :one
INSERT INTO payment_provider_customers (account_id, user_id, provider, provider_customer_ref)
VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id, provider) DO UPDATE
SET provider_customer_ref = EXCLUDED.provider_customer_ref
RETURNING *;

-- name: ListPaymentSavedCardsByAccount :many
SELECT *
FROM payment_saved_cards
WHERE account_id = $1
ORDER BY is_default DESC, created_at DESC;

-- name: GetPaymentSavedCardByUUID :one
SELECT *
FROM payment_saved_cards
WHERE uuid = $1
  AND account_id = $2;

-- name: UpsertPaymentSavedCard :one
INSERT INTO payment_saved_cards (
    account_id, user_id, provider, provider_card_ref, provider_customer_ref,
    last4, brand, exp_month, exp_year, is_default
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (account_id, provider, provider_card_ref) DO UPDATE
SET last4 = EXCLUDED.last4,
    brand = EXCLUDED.brand,
    is_default = EXCLUDED.is_default OR payment_saved_cards.is_default
RETURNING *;

-- name: ClearDefaultPaymentSavedCards :exec
UPDATE payment_saved_cards
SET is_default = FALSE
WHERE account_id = $1;

-- name: SetDefaultPaymentSavedCard :one
UPDATE payment_saved_cards
SET is_default = TRUE
WHERE uuid = $1 AND account_id = $2
RETURNING *;

-- name: DeletePaymentSavedCard :exec
DELETE FROM payment_saved_cards
WHERE uuid = $1 AND account_id = $2;

-- name: CreatePaymentCardSession :one
INSERT INTO payment_card_sessions (account_id, user_id, provider, status, merchant_oid, expires_at)
VALUES ($1, $2, $3, 'pending', $4, $5)
RETURNING *;

-- account-lint: cross-account usecase checks row.AccountID against the active account (404 otherwise)
-- name: GetPaymentCardSessionByUUID :one
SELECT *
FROM payment_card_sessions
WHERE uuid = $1;

-- account-lint: cross-account payment provider callback: merchant oid is the provider's reference
-- name: GetPaymentCardSessionByMerchantOID :one
SELECT *
FROM payment_card_sessions
WHERE merchant_oid = $1;

-- account-lint: cross-account payment provider callback: provider token is the reference
-- name: GetPaymentCardSessionByProviderToken :one
SELECT *
FROM payment_card_sessions
WHERE provider_token = $1;

-- account-lint: cross-account add-card start: session id from the row just inserted for this account
-- name: UpdatePaymentCardSessionInit :one
UPDATE payment_card_sessions
SET provider_token = COALESCE(sqlc.narg(provider_token), provider_token),
    checkout_url = COALESCE(sqlc.narg(checkout_url), checkout_url)
WHERE uuid = sqlc.arg(uuid)
RETURNING *;

-- account-lint: cross-account payment provider callback: session found by merchant oid / provider token
-- name: CompletePaymentCardSession :one
UPDATE payment_card_sessions
SET status = 'completed',
    completed_at = NOW()
WHERE uuid = sqlc.arg(uuid)
  AND status = 'pending'
RETURNING *;

-- account-lint: cross-account payment provider callback / init failure: session id from that flow
-- name: FailPaymentCardSession :one
UPDATE payment_card_sessions
SET status = 'failed'
WHERE uuid = sqlc.arg(uuid)
  AND status = 'pending'
RETURNING *;

-- name: GetAccountActivePaymentSubscription :one
SELECT s.*,
       p.uuid AS plan_uuid,
       p.slug AS plan_slug,
       p.name AS plan_name,
       p.billing_interval AS plan_billing_interval,
       p.price_minor AS plan_price_minor,
       p.currency AS plan_currency,
       sc.uuid AS saved_card_uuid,
       pending.uuid AS pending_plan_uuid
FROM subscriptions s
INNER JOIN plans p ON p.id = s.plan_id
LEFT JOIN payment_saved_cards sc ON sc.id = s.saved_card_id
LEFT JOIN plans pending ON pending.id = s.pending_plan_id
WHERE s.account_id = $1
  AND s.status IN ('active', 'trialing', 'past_due')
  AND s.source = 'payment'
ORDER BY s.created_at DESC
LIMIT 1;

-- name: CancelAccountPaymentSubscription :one
UPDATE subscriptions
SET status = 'canceled',
    canceled_at = NOW()
WHERE uuid = $1
  AND account_id = $2
  AND source = 'payment'
  AND status IN ('active', 'trialing', 'past_due')
RETURNING *;
