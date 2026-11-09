-- name: ListOrders :many
SELECT 
    o.*,
    u.email as customer_email,
    u.name as customer_name,
    u.surname as customer_surname,
    ca.name as company_name,
    a.uuid as account_uuid,
    a.name as account_name,
    a.type as account_type,
    (SELECT COUNT(*)::int FROM order_items oi WHERE oi.order_id = o.id) as items_count
FROM orders o
JOIN users u ON u.id = o.user_id
JOIN accounts a ON a.id = o.account_id
LEFT JOIN accounts ca ON ca.id = o.account_id AND ca.type = 'company'
WHERE o.deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR o.order_number ILIKE '%' || sqlc.narg(q) || '%'
    OR u.email ILIKE '%' || sqlc.narg(q) || '%'
    OR u.name ILIKE '%' || sqlc.narg(q) || '%'
    OR u.surname ILIKE '%' || sqlc.narg(q) || '%'
    OR o.tracking_number ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(account_id)::bigint IS NULL
    OR o.account_id = sqlc.narg(account_id)::bigint
  )
  AND (
    sqlc.narg(status)::text IS NULL
    OR o.status = sqlc.narg(status)::text
  )
  AND (
    sqlc.narg(payment_status)::text IS NULL
    OR o.payment_status = sqlc.narg(payment_status)::text
  )
  AND (
    sqlc.narg(fulfillment_status)::text IS NULL
    OR o.fulfillment_status = sqlc.narg(fulfillment_status)::text
  )
  AND (
    sqlc.narg(order_type)::text IS NULL
    OR o.order_type = sqlc.narg(order_type)::text
  )
ORDER BY o.created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountOrders :one
SELECT COUNT(*)::bigint
FROM orders o
JOIN users u ON u.id = o.user_id
WHERE o.deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR o.order_number ILIKE '%' || sqlc.narg(q) || '%'
    OR u.email ILIKE '%' || sqlc.narg(q) || '%'
    OR u.name ILIKE '%' || sqlc.narg(q) || '%'
    OR u.surname ILIKE '%' || sqlc.narg(q) || '%'
    OR o.tracking_number ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(account_id)::bigint IS NULL
    OR o.account_id = sqlc.narg(account_id)::bigint
  )
  AND (
    sqlc.narg(status)::text IS NULL
    OR o.status = sqlc.narg(status)::text
  )
  AND (
    sqlc.narg(payment_status)::text IS NULL
    OR o.payment_status = sqlc.narg(payment_status)::text
  )
  AND (
    sqlc.narg(fulfillment_status)::text IS NULL
    OR o.fulfillment_status = sqlc.narg(fulfillment_status)::text
  )
  AND (
    sqlc.narg(order_type)::text IS NULL
    OR o.order_type = sqlc.narg(order_type)::text
  );

-- account-lint: cross-account platform orders; app GET /v1/orders/{uuid} goes through GetForAccount, which checks AccountID
-- name: GetOrderByUUID :one
SELECT 
    o.*,
    u.uuid as user_uuid,
    u.email as customer_email,
    u.name as customer_name,
    u.surname as customer_surname,
    ca.name as company_name,
    a.uuid as account_uuid,
    a.name as account_name,
    a.type as account_type
FROM orders o
JOIN users u ON u.id = o.user_id
JOIN accounts a ON a.id = o.account_id
LEFT JOIN accounts ca ON ca.id = o.account_id AND ca.type = 'company'
WHERE o.uuid = $1
  AND o.deleted_at IS NULL;

-- account-lint: cross-account platform order lookup by number (platform.orders.read)
-- name: GetOrderByOrderNumber :one
SELECT 
    o.*,
    u.uuid as user_uuid,
    u.email as customer_email,
    u.name as customer_name,
    u.surname as customer_surname,
    ca.name as company_name,
    a.uuid as account_uuid,
    a.name as account_name,
    a.type as account_type
FROM orders o
JOIN users u ON u.id = o.user_id
JOIN accounts a ON a.id = o.account_id
LEFT JOIN accounts ca ON ca.id = o.account_id AND ca.type = 'company'
WHERE o.order_number = $1
  AND o.deleted_at IS NULL;

-- name: CreateOrder :one
INSERT INTO orders (
    account_id,
    order_number,
    user_id,
    status,
    subtotal,
    tax_total,
    shipping_total,
    discount_total,
    grand_total,
    currency,
    payment_status,
    payment_provider,
    payment_reference,
    shipping_address,
    billing_address,
    customer_note,
    admin_note,
    order_type,
    fulfillment_status,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
)
RETURNING *;

-- Locks the order row so the from_status written to order_status_history (TEC-944) is the
-- status the transition actually replaced.
-- account-lint: cross-account platform order status change (platform.orders.write)
-- name: LockOrderForStatusChange :one
SELECT id, account_id, status
FROM orders
WHERE uuid = sqlc.arg(uuid)::uuid
  AND deleted_at IS NULL
FOR UPDATE;

-- name: InsertOrderStatusHistory :exec
INSERT INTO order_status_history (account_id, order_id, from_status, to_status, changed_by_user_id)
VALUES (sqlc.arg(account_id), sqlc.arg(order_id), sqlc.narg(from_status), sqlc.arg(to_status), sqlc.narg(changed_by_user_id));

-- account-lint: cross-account platform order status / provider callback apply
-- name: UpdateOrderStatus :one
UPDATE orders
SET status = sqlc.arg(status)::text,
    paid_at = CASE WHEN sqlc.arg(status)::text = 'paid' AND paid_at IS NULL THEN NOW() ELSE paid_at END,
    shipped_at = CASE WHEN sqlc.arg(status)::text = 'shipped' AND shipped_at IS NULL THEN NOW() ELSE shipped_at END,
    delivered_at = CASE WHEN sqlc.arg(status)::text = 'delivered' AND delivered_at IS NULL THEN NOW() ELSE delivered_at END,
    cancelled_at = CASE WHEN sqlc.arg(status)::text = 'cancelled' AND cancelled_at IS NULL THEN NOW() ELSE cancelled_at END
WHERE uuid = sqlc.arg(uuid)::uuid
  AND deleted_at IS NULL
RETURNING *;

-- account-lint: cross-account platform order payment / provider callback apply
-- name: UpdateOrderPaymentStatus :one
UPDATE orders
SET payment_status = sqlc.arg(payment_status)::text,
    payment_provider = COALESCE(sqlc.narg(payment_provider), payment_provider),
    payment_reference = COALESCE(sqlc.narg(payment_reference), payment_reference),
    paid_at = CASE WHEN sqlc.arg(payment_status)::text = 'paid' AND paid_at IS NULL THEN NOW() ELSE paid_at END
WHERE uuid = sqlc.arg(uuid)::uuid
  AND deleted_at IS NULL
RETURNING *;

-- account-lint: cross-account platform /v1/platform/orders (platform.orders.write)
-- name: UpdateOrderFulfillmentStatus :one
UPDATE orders
SET fulfillment_status = sqlc.arg(fulfillment_status)::text,
    shipped_at = CASE WHEN sqlc.arg(fulfillment_status)::text = 'shipped' AND shipped_at IS NULL THEN NOW() ELSE shipped_at END,
    delivered_at = CASE WHEN sqlc.arg(fulfillment_status)::text = 'delivered' AND delivered_at IS NULL THEN NOW() ELSE delivered_at END
WHERE uuid = sqlc.arg(uuid)::uuid
  AND deleted_at IS NULL
RETURNING *;

-- account-lint: cross-account platform /v1/platform/orders (platform.orders.write)
-- name: UpdateOrderShipping :one
UPDATE orders
SET shipping_carrier = sqlc.arg(shipping_carrier),
    tracking_number = sqlc.arg(tracking_number),
    tracking_url = sqlc.arg(tracking_url),
    status = CASE WHEN sqlc.arg(new_status)::text != '' THEN sqlc.arg(new_status)::text ELSE status END,
    fulfillment_status = CASE WHEN sqlc.arg(new_status)::text = 'shipped' THEN 'shipped' ELSE fulfillment_status END,
    shipped_at = CASE WHEN sqlc.arg(new_status)::text = 'shipped' AND shipped_at IS NULL THEN NOW() ELSE shipped_at END
WHERE uuid = sqlc.arg(uuid)
  AND deleted_at IS NULL
RETURNING *;

-- account-lint: cross-account platform /v1/platform/orders (platform.orders.write)
-- name: UpdateOrderNotes :one
UPDATE orders
SET admin_note = COALESCE(sqlc.narg(admin_note), admin_note),
    customer_note = COALESCE(sqlc.narg(customer_note), customer_note)
WHERE uuid = sqlc.arg(uuid)
  AND deleted_at IS NULL
RETURNING *;

-- account-lint: cross-account platform /v1/platform/orders (platform.orders.write)
-- name: DeleteOrder :exec
UPDATE orders
SET deleted_at = NOW()
WHERE uuid = $1;

-- account-lint: cross-account items of an order already loaded through a platform route or GetForAccount
-- name: ListOrderItemsByOrderID :many
SELECT 
    oi.*,
    p.slug as product_slug,
    p.images as product_images,
    prof.uuid as profile_uuid,
    prof.avatar_url as profile_avatar_url,
    nc.uuid as card_uuid,
    nc.smart_code as card_smart_code,
    nc.card_uid as card_uid,
    nc.status as card_status,
    nc.is_active as card_is_active,
    nc.is_locked as card_is_locked,
    r.path as route_path,
    r.status as route_status
FROM order_items oi
LEFT JOIN products p ON p.id = oi.product_id
LEFT JOIN profiles prof ON prof.id = oi.profile_id AND prof.deleted_at IS NULL
LEFT JOIN nfc_cards nc ON nc.id = oi.card_id AND nc.deleted_at IS NULL
LEFT JOIN routes r ON r.card_id = nc.id AND r.deleted_at IS NULL
WHERE oi.order_id = $1
ORDER BY oi.id ASC;

-- account-lint: cross-account platform order provisioning (platform.orders.write)
-- name: GetOrderItemByUUID :one
SELECT 
    oi.*,
    p.slug as product_slug,
    p.images as product_images,
    prof.uuid as profile_uuid,
    prof.avatar_url as profile_avatar_url,
    nc.uuid as card_uuid,
    nc.smart_code as card_smart_code,
    nc.card_uid as card_uid,
    nc.status as card_status,
    nc.is_active as card_is_active,
    nc.is_locked as card_is_locked,
    r.path as route_path,
    r.status as route_status
FROM order_items oi
LEFT JOIN products p ON p.id = oi.product_id
LEFT JOIN profiles prof ON prof.id = oi.profile_id AND prof.deleted_at IS NULL
LEFT JOIN nfc_cards nc ON nc.id = oi.card_id AND nc.deleted_at IS NULL
LEFT JOIN routes r ON r.card_id = nc.id AND r.deleted_at IS NULL
WHERE oi.uuid = $1;

-- name: CreateOrderItem :one
INSERT INTO order_items (
    order_id,
    product_id,
    variant_id,
    product_name,
    variant_name,
    sku,
    quantity,
    unit_price,
    total_price,
    custom_print_data,
    recipient_name,
    recipient_email,
    recipient_phone,
    recipient_title,
    recipient_department,
    profile_id,
    card_id,
    fulfillment_status,
    provisioning_status,
    provisioning_error,
    idempotency_key
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
)
RETURNING *;

-- name: UpdateOrderItemProvisioning :one
UPDATE order_items
SET 
    card_id = COALESCE(sqlc.narg(card_id), card_id),
    provisioning_status = sqlc.arg(provisioning_status),
    provisioning_error = sqlc.arg(provisioning_error)
WHERE uuid = sqlc.arg(uuid)
RETURNING *;

-- name: UpdateOrderItemFulfillment :one
UPDATE order_items
SET fulfillment_status = sqlc.arg(fulfillment_status)
WHERE uuid = sqlc.arg(uuid)
RETURNING *;

-- name: ListOrderAssetRequests :many
SELECT 
    ar.*,
    u.name as requested_by_name,
    u.surname as requested_by_surname
FROM order_asset_requests ar
LEFT JOIN users u ON u.id = ar.requested_by
WHERE ar.order_id = $1
ORDER BY ar.created_at DESC;

-- name: GetAssetRequestByUUID :one
SELECT *
FROM order_asset_requests
WHERE uuid = $1;

-- account-lint: cross-account public asset upload link: the token is the credential
-- name: GetAssetRequestByToken :one
SELECT 
    ar.*,
    o.order_number,
    o.user_id as order_user_id
FROM order_asset_requests ar
JOIN orders o ON o.id = ar.order_id
WHERE ar.token = $1
  AND o.deleted_at IS NULL;

-- name: CreateAssetRequest :one
INSERT INTO order_asset_requests (
    order_id,
    request_type,
    title,
    description,
    status,
    token,
    requested_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: UpdateAssetRequestStatus :one
UPDATE order_asset_requests
SET status = sqlc.arg(status),
    uploaded_file_url = CASE WHEN sqlc.arg(uploaded_file_url)::text != '' THEN sqlc.arg(uploaded_file_url)::text ELSE uploaded_file_url END,
    rejection_reason = CASE WHEN sqlc.arg(rejection_reason)::text != '' THEN sqlc.arg(rejection_reason)::text ELSE rejection_reason END
WHERE uuid = sqlc.arg(uuid)
RETURNING *;

-- account-lint: cross-account platform order KPIs
-- name: GetOrderStats :one
SELECT 
    COUNT(*)::bigint as total_orders,
    COUNT(CASE WHEN created_at >= CURRENT_DATE THEN 1 END)::bigint as today_orders,
    COUNT(CASE WHEN status = 'pending' THEN 1 END)::bigint as pending_orders,
    COUNT(CASE WHEN status = 'design_pending' THEN 1 END)::bigint as design_pending_orders,
    COUNT(CASE WHEN status = 'in_production' THEN 1 END)::bigint as in_production_orders,
    COUNT(CASE WHEN status = 'shipped' THEN 1 END)::bigint as shipped_orders,
    COALESCE(SUM(CASE WHEN payment_status = 'paid' THEN grand_total ELSE 0 END), 0)::numeric as total_revenue
FROM orders
WHERE deleted_at IS NULL;

-- account-lint: cross-account platform production manifest (platform.orders.read + step-up)
-- name: ListOrderManifest :many
-- Production manifest (TEC-581): one row per provisioned card of an order, including the
-- activation code. Platform-only; callers gate it behind platform.orders.read + step-up.
SELECT
    o.order_number,
    oi.uuid AS item_uuid,
    oi.product_name,
    oi.variant_name,
    oi.sku,
    oi.recipient_name,
    nc.uuid AS card_uuid,
    nc.name AS card_name,
    nc.card_uid,
    nc.smart_code,
    nc.activation_code,
    nc.status AS card_status
FROM order_items oi
JOIN orders o ON o.id = oi.order_id AND o.deleted_at IS NULL
JOIN nfc_cards nc ON nc.order_item_id = oi.id AND nc.deleted_at IS NULL
WHERE o.uuid = $1
ORDER BY oi.id ASC, nc.id ASC;

-- account-lint: cross-account platform cancel/refund: retires the cards of that order
-- name: RetireOrderCardRoutes :many
-- Cancelled/refunded orders (TEC-585): the order's card routes become retired, which the
-- resolver shows as "card deactivated". Paths are never released for reuse. Unused activation
-- codes are cleared so nobody can still claim those cards.
WITH retired AS (
    UPDATE routes r
    SET status = 'retired'
    FROM nfc_cards c
    WHERE r.card_id = c.id
      AND c.order_id = sqlc.arg(order_id)
      AND r.deleted_at IS NULL
      AND r.status <> 'retired'
    RETURNING r.path, c.uuid AS card_uuid
), cleared AS (
    UPDATE nfc_cards
    SET activation_code = NULL
    WHERE order_id = sqlc.arg(order_id)
      AND activation_code IS NOT NULL
    RETURNING id
)
SELECT path, card_uuid FROM retired;
