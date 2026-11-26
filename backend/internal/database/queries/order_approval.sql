-- Company bulk order approval (TEC-1317).

-- name: FindAccountProfileByEmail :one
SELECT p.id, p.uuid
FROM profiles p
WHERE p.account_id = sqlc.arg(account_id)
  AND p.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM profile_contact_details c
      WHERE c.profile_id = p.id AND c.category = 'email'
        AND lower(btrim(c.value)) = lower(btrim(sqlc.arg(email)::text))
  )
ORDER BY p.created_at, p.id
LIMIT 1;

-- name: MarkOrderItemProfileCreated :exec
UPDATE order_items
SET profile_created_by_order = true
WHERE order_id = sqlc.arg(order_id)
  AND uuid = sqlc.arg(item_uuid);

-- name: ListOrderCreatedProfiles :many
SELECT p.uuid
FROM order_items oi
JOIN profiles p ON p.id = oi.profile_id
WHERE oi.order_id = sqlc.arg(order_id)
  AND oi.profile_created_by_order
  AND p.account_id = sqlc.arg(account_id)
  AND p.deleted_at IS NULL
ORDER BY oi.id;

-- name: OpenOrderDraftProfileRoutes :many
UPDATE routes r
SET status = 'active'
FROM order_items oi, profiles p
WHERE oi.order_id = sqlc.arg(order_id)
  AND oi.profile_created_by_order
  AND p.id = oi.profile_id
  AND p.account_id = sqlc.arg(account_id)
  AND p.deleted_at IS NULL
  AND r.profile_id = p.id
  AND r.account_id = sqlc.arg(account_id)
  AND r.route_type = 'canonical_profile'
  AND r.status = 'locked'
  AND r.deleted_at IS NULL
RETURNING p.uuid;

-- account-lint: cross-account platform order review (platform.orders.approve)
-- name: SetOrderRejectionReason :exec
UPDATE orders
SET rejection_reason = sqlc.arg(reason)::text
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
