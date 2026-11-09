-- TEC-943: online payment of card orders.

-- name: CreateOrderPaymentSession :one
INSERT INTO order_payment_sessions (
    account_id, order_id, user_id, provider, merchant_oid, amount_minor, currency, expires_at
)
VALUES (
    sqlc.arg(account_id), sqlc.arg(order_id), sqlc.arg(user_id), sqlc.arg(provider),
    sqlc.arg(merchant_oid), sqlc.arg(amount_minor), sqlc.arg(currency), sqlc.arg(expires_at)
)
RETURNING *;

-- account-lint: cross-account order checkout start: session id from the row just inserted for this account
-- name: UpdateOrderPaymentSessionInit :one
UPDATE order_payment_sessions
SET provider_token = sqlc.narg(provider_token),
    checkout_url = sqlc.narg(checkout_url)
WHERE uuid = sqlc.arg(uuid)
RETURNING *;

-- account-lint: cross-account usecase checks row.AccountID against the active account (404 otherwise)
-- name: GetOrderPaymentSessionByUUID :one
SELECT ops.*,
       o.uuid AS order_uuid,
       o.order_number AS order_number,
       o.status AS order_status,
       o.payment_status AS order_payment_status
FROM order_payment_sessions ops
INNER JOIN orders o ON o.id = ops.order_id
WHERE ops.uuid = sqlc.arg(uuid);

-- account-lint: cross-account payment provider callback: merchant oid is the provider's reference
-- name: GetOrderPaymentSessionByMerchantOID :one
SELECT * FROM order_payment_sessions WHERE merchant_oid = sqlc.arg(merchant_oid);

-- account-lint: cross-account payment provider callback: provider token is the reference
-- name: GetOrderPaymentSessionByProviderToken :one
SELECT * FROM order_payment_sessions WHERE provider_token = sqlc.arg(provider_token);

-- account-lint: cross-account order checkout flow: session id from that flow (callback or the account's own start)
-- name: LockOrderPaymentSession :one
SELECT * FROM order_payment_sessions WHERE merchant_oid = sqlc.arg(merchant_oid) FOR UPDATE;

-- account-lint: cross-account payment provider callback: session found by merchant oid / provider token
-- name: CompleteOrderPaymentSession :one
UPDATE order_payment_sessions
SET status = 'completed',
    provider_ref = COALESCE(sqlc.narg(provider_ref), provider_ref),
    metadata = COALESCE(sqlc.narg(metadata), metadata),
    completed_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = 'pending'
RETURNING *;

-- account-lint: cross-account payment provider callback / init failure: session id from that flow
-- name: FailOrderPaymentSession :one
UPDATE order_payment_sessions
SET status = 'failed',
    metadata = COALESCE(sqlc.narg(metadata), metadata)
WHERE id = sqlc.arg(id)
  AND status = 'pending'
RETURNING *;

-- account-lint: cross-account provider payment/refund apply: order id from a verified session
-- name: LockOrderForPayment :one
SELECT id, uuid, account_id, status, payment_status
FROM orders
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
FOR UPDATE;

-- account-lint: cross-account usecase checks target.AccountID against the active account
-- name: GetOrderPaymentAmount :one
SELECT id, uuid, account_id, order_number, status, payment_status, currency,
       (grand_total * 100)::bigint AS amount_minor
FROM orders
WHERE uuid = sqlc.arg(uuid)
  AND deleted_at IS NULL;
