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

-- Card design images chosen when an order is placed (TEC-1320 B).

-- account-lint: cross-account platform order creation sets the order's card images
-- name: SetOrderCardImages :exec
UPDATE orders
SET front_image_url = sqlc.narg(front_image_url)::text,
    back_image_url  = sqlc.narg(back_image_url)::text
WHERE id = sqlc.arg(id);

-- name: SetOrderItemCardImages :exec
UPDATE order_items
SET front_image_url = sqlc.narg(front_image_url)::text,
    back_image_url  = sqlc.narg(back_image_url)::text
WHERE order_id = sqlc.arg(order_id)
  AND uuid = sqlc.arg(item_uuid);

-- name: CopyOrderCardImagesToCard :exec
-- The item's image wins, then the order's; NULL keeps the default artwork.
UPDATE nfc_cards c
SET front_image_url = COALESCE(oi.front_image_url, o.front_image_url),
    back_image_url  = COALESCE(oi.back_image_url, o.back_image_url)
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
WHERE c.id = sqlc.arg(card_id)
  AND c.account_id = sqlc.arg(account_id)
  AND o.account_id = sqlc.arg(account_id)
  AND oi.id = sqlc.arg(order_item_id);
