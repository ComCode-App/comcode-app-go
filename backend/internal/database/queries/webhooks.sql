-- Outbound webhooks (TEC-603). Endpoint reads and writes are scoped to the account.

-- name: ListWebhookEndpoints :many
SELECT * FROM webhook_endpoints
WHERE account_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: CountWebhookEndpoints :one
SELECT COUNT(*) FROM webhook_endpoints WHERE account_id = $1 AND deleted_at IS NULL;

-- name: GetWebhookEndpointByUUID :one
SELECT * FROM webhook_endpoints
WHERE uuid = $1 AND account_id = $2 AND deleted_at IS NULL;

-- name: ListActiveWebhookEndpointsForEvent :many
SELECT * FROM webhook_endpoints
WHERE account_id = $1 AND deleted_at IS NULL AND is_active
  AND sqlc.arg(event_name)::text = ANY(events)
ORDER BY id;

-- name: CreateWebhookEndpoint :one
INSERT INTO webhook_endpoints (account_id, url, description, secret_enc, secret_hint, events, is_active, created_by_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateWebhookEndpoint :one
UPDATE webhook_endpoints
SET url = $3, description = $4, events = $5, is_active = $6,
    consecutive_failures = CASE WHEN $6 AND NOT is_active THEN 0 ELSE consecutive_failures END
WHERE uuid = $1 AND account_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: RotateWebhookEndpointSecret :one
UPDATE webhook_endpoints SET secret_enc = $3, secret_hint = $4
WHERE uuid = $1 AND account_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: DeleteWebhookEndpoint :execrows
UPDATE webhook_endpoints SET deleted_at = NOW(), is_active = FALSE
WHERE uuid = $1 AND account_id = $2 AND deleted_at IS NULL;

-- account-lint: cross-account webhook delivery worker: endpoint of the delivery being sent
-- name: TouchWebhookEndpointDelivery :exec
UPDATE webhook_endpoints
SET last_delivery_at = NOW(),
    last_success_at = CASE WHEN sqlc.arg(succeeded)::boolean THEN NOW() ELSE last_success_at END,
    consecutive_failures = CASE WHEN sqlc.arg(succeeded)::boolean THEN 0 ELSE consecutive_failures + 1 END
WHERE id = $1;

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (endpoint_id, account_id, event_name, event_id, payload, max_attempts)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- account-lint: cross-account webhook delivery worker: delivery id from the queue
-- name: GetWebhookDeliveryByID :one
SELECT d.*, e.url AS endpoint_url, e.secret_enc AS endpoint_secret_enc, e.is_active AS endpoint_is_active,
       e.deleted_at AS endpoint_deleted_at, e.uuid AS endpoint_uuid
FROM webhook_deliveries d
JOIN webhook_endpoints e ON e.id = d.endpoint_id
WHERE d.id = $1;

-- name: GetWebhookDeliveryByUUID :one
SELECT d.* FROM webhook_deliveries d
JOIN webhook_endpoints e ON e.id = d.endpoint_id
WHERE d.uuid = $1 AND d.account_id = $2 AND e.uuid = sqlc.arg(endpoint_uuid) AND e.deleted_at IS NULL;

-- account-lint: cross-account backlog cap per endpoint: endpoint row already loaded for the event (account-scoped dispatch)
-- name: CountPendingWebhookDeliveries :one
SELECT COUNT(*) FROM webhook_deliveries WHERE endpoint_id = $1 AND status IN ('pending', 'failed');

-- Claims a delivery for one attempt: only a pending/failed row whose retry time has come. A task
-- that fires early or twice gets no row and does nothing.
-- account-lint: cross-account webhook delivery worker
-- name: ClaimWebhookDelivery :execrows
UPDATE webhook_deliveries SET updated_at = NOW()
WHERE id = $1 AND status IN ('pending', 'failed') AND next_attempt_at <= NOW() + INTERVAL '30 seconds';

-- Deliveries whose task was lost (Redis down at enqueue, worker crash): due for a while and
-- never attempted since.
-- account-lint: cross-account webhook delivery worker
-- name: ListStaleWebhookDeliveries :many
SELECT id, attempts FROM webhook_deliveries
WHERE status IN ('pending', 'failed') AND next_attempt_at < NOW() - INTERVAL '10 minutes'
ORDER BY id LIMIT 500;

-- name: ListWebhookDeliveries :many
SELECT d.*, COUNT(*) OVER()::bigint AS total
FROM webhook_deliveries d
JOIN webhook_endpoints e ON e.id = d.endpoint_id
WHERE d.account_id = $1 AND e.uuid = sqlc.arg(endpoint_uuid)
  AND (sqlc.narg(status)::webhook_delivery_status IS NULL OR d.status = sqlc.narg(status)::webhook_delivery_status)
ORDER BY d.created_at DESC, d.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- account-lint: cross-account webhook delivery worker
-- name: RecordWebhookDeliveryAttempt :one
UPDATE webhook_deliveries
SET status = sqlc.arg(status)::webhook_delivery_status, attempts = attempts + 1,
    next_attempt_at = sqlc.arg(next_attempt_at), last_status_code = sqlc.narg(last_status_code),
    last_error = sqlc.arg(last_error), last_attempt_at = NOW(),
    delivered_at = CASE WHEN sqlc.arg(status)::webhook_delivery_status = 'succeeded' THEN NOW() ELSE delivered_at END
WHERE id = sqlc.arg(id)
RETURNING *;

-- account-lint: cross-account resend: delivery id from GetWebhookDeliveryByUUID (account-scoped)
-- name: ResetWebhookDelivery :one
UPDATE webhook_deliveries
SET status = 'pending', attempts = 0, next_attempt_at = NOW(), last_error = '', last_status_code = NULL
WHERE id = $1
RETURNING *;

-- account-lint: cross-account webhook retention worker
-- name: PruneWebhookDeliveries :execrows
DELETE FROM webhook_deliveries WHERE created_at < sqlc.arg(before)::timestamptz;
