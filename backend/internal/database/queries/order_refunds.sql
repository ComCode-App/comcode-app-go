-- TEC-935: refunds of online card order payments through the payment provider.

-- account-lint: cross-account platform order refunds (platform.orders.read)
-- name: ListOrderPaymentsForRefund :many
SELECT ops.id, ops.uuid, ops.provider, ops.merchant_oid, ops.provider_ref, ops.amount_minor,
       ops.currency, ops.completed_at,
       COALESCE(ops.metadata->>'order_result', '')::text AS order_result,
       COALESCE((SELECT SUM(r.amount_minor) FROM order_payment_refunds r
                 WHERE r.session_id = ops.id AND r.status = 'succeeded'), 0)::bigint AS refunded_minor,
       COALESCE((SELECT SUM(r.amount_minor) FROM order_payment_refunds r
                 WHERE r.session_id = ops.id AND r.status = 'pending'), 0)::bigint AS pending_minor
FROM order_payment_sessions ops
WHERE ops.order_id = sqlc.arg(order_id)
  AND ops.status = 'completed'
ORDER BY ops.completed_at, ops.id;

-- account-lint: cross-account platform order refunds (platform.orders.write)
-- name: LockOrderPaymentSessionByID :one
SELECT * FROM order_payment_sessions WHERE id = sqlc.arg(id) FOR UPDATE;

-- account-lint: cross-account platform order refunds (platform.orders.*)
-- name: SumOrderPaymentRefunds :one
SELECT COALESCE(SUM(amount_minor) FILTER (WHERE status = 'succeeded'), 0)::bigint AS refunded_minor,
       COALESCE(SUM(amount_minor) FILTER (WHERE status = 'pending'), 0)::bigint AS pending_minor
FROM order_payment_refunds
WHERE session_id = sqlc.arg(session_id);

-- name: CreateOrderPaymentRefund :one
INSERT INTO order_payment_refunds (
    account_id, order_id, session_id, provider, amount_minor, currency, reference_no,
    idempotency_key, reason, requested_by_user_id
)
VALUES (
    sqlc.arg(account_id), sqlc.arg(order_id), sqlc.arg(session_id), sqlc.arg(provider),
    sqlc.arg(amount_minor), sqlc.arg(currency), sqlc.arg(reference_no),
    sqlc.arg(idempotency_key), sqlc.arg(reason), sqlc.narg(requested_by_user_id)
)
RETURNING *;

-- account-lint: cross-account platform order refunds (platform.orders.write + step-up)
-- name: GetOrderPaymentRefundByIdempotencyKey :one
SELECT * FROM order_payment_refunds WHERE idempotency_key = sqlc.arg(idempotency_key);

-- account-lint: cross-account platform order refunds (platform.orders.*)
-- name: GetOrderPaymentRefundByUUID :one
SELECT * FROM order_payment_refunds WHERE uuid = sqlc.arg(uuid);

-- account-lint: cross-account platform order refunds (platform.orders.write)
-- name: LockOrderPaymentRefund :one
SELECT * FROM order_payment_refunds WHERE id = sqlc.arg(id) FOR UPDATE;

-- account-lint: cross-account platform order refunds (platform.orders.write)
-- name: MarkOrderPaymentRefundSucceeded :one
UPDATE order_payment_refunds
SET status = 'succeeded',
    provider_ref = sqlc.narg(provider_ref),
    error_code = NULL,
    error_message = NULL,
    completed_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = 'pending'
RETURNING *;

-- account-lint: cross-account platform order refunds (platform.orders.write)
-- name: MarkOrderPaymentRefundFailed :one
UPDATE order_payment_refunds
SET status = 'failed',
    error_code = sqlc.arg(error_code),
    error_message = sqlc.arg(error_message)
WHERE id = sqlc.arg(id)
  AND status = 'pending'
RETURNING *;

-- account-lint: cross-account platform order refunds (platform.orders.write)
-- name: RetryOrderPaymentRefund :one
UPDATE order_payment_refunds
SET status = 'pending',
    attempts = attempts + 1,
    requested_by_user_id = COALESCE(sqlc.narg(requested_by_user_id), requested_by_user_id)
WHERE id = sqlc.arg(id)
  AND status = 'failed'
RETURNING *;

-- account-lint: cross-account platform order refunds (platform.orders.read)
-- name: ListOrderPaymentRefunds :many
SELECT sqlc.embed(r), ops.uuid AS session_uuid
FROM order_payment_refunds r
INNER JOIN order_payment_sessions ops ON ops.id = r.session_id
WHERE r.order_id = sqlc.arg(order_id)
ORDER BY r.created_at DESC, r.id DESC;

-- account-lint: cross-account platform order refunds (platform.orders.*)
-- name: GetOrderPaymentSessionByID :one
SELECT * FROM order_payment_sessions WHERE id = sqlc.arg(id);
