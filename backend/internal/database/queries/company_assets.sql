-- Company employee assets: account_id and live actor scope are always required.

-- name: ListCompanyAssetCards :many
SELECT c.uuid, c.name, c.smart_code, COALESCE(c.card_uid, '')::text AS card_uid,
c.status, c.is_active, c.is_locked, p.uuid AS profile_uuid,
r.uuid AS route_uuid, r.path AS route_path, r.status AS route_status, r.is_immutable,
u.uuid AS member_uuid, COALESCE(u.name || ' ' || u.surname, '')::text AS member_name,
off.uuid AS office_uuid, off.name AS office_name, g.uuid AS group_uuid, g.name AS group_name,
c.created_at
FROM nfc_cards c
LEFT JOIN profiles p ON p.account_id = c.account_id AND p.id = c.profile_id AND p.deleted_at IS NULL
LEFT JOIN routes r ON r.account_id = c.account_id AND r.card_id = c.id
    AND r.route_type = 'card_route' AND r.deleted_at IS NULL
LEFT JOIN company_member_profiles l ON l.account_id = c.account_id AND l.profile_id = c.profile_id
LEFT JOIN account_members m ON m.account_id = c.account_id AND m.id = l.member_id
LEFT JOIN users u ON u.id = m.user_id
LEFT JOIN company_member_assignments a ON a.account_id = m.account_id AND a.member_id = m.id
LEFT JOIN company_offices off ON off.account_id = c.account_id AND off.id = a.office_id
LEFT JOIN company_groups g ON g.account_id = c.account_id AND g.id = a.group_id
WHERE c.account_id = sqlc.arg(account_id) AND c.deleted_at IS NULL
AND (sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR (sqlc.arg(actor_role)::text = 'member' AND m.user_id = sqlc.arg(actor_id))
    OR (sqlc.arg(actor_role)::text = 'office_manager' AND EXISTS (
        SELECT 1 FROM account_member_scopes s JOIN account_members actor ON actor.id = s.member_id
        WHERE s.account_id = m.account_id AND actor.user_id = sqlc.arg(actor_id) AND s.office_id = a.office_id)))
AND (sqlc.narg(office_uuid)::uuid IS NULL OR off.uuid = sqlc.narg(office_uuid))
AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid))
AND (sqlc.arg(status)::text = '' OR c.status = sqlc.arg(status))
AND (sqlc.narg(is_active)::boolean IS NULL OR c.is_active = sqlc.narg(is_active))
AND (sqlc.arg(q)::text = '' OR c.name ILIKE '%' || sqlc.arg(q) || '%'
    OR c.smart_code ILIKE '%' || sqlc.arg(q) || '%' OR c.card_uid ILIKE '%' || sqlc.arg(q) || '%'
    OR (u.name || ' ' || u.surname) ILIKE '%' || sqlc.arg(q) || '%' OR u.email ILIKE '%' || sqlc.arg(q) || '%')
ORDER BY CASE WHEN sqlc.arg(sort)::text = 'created_at' THEN c.created_at END ASC,
CASE WHEN sqlc.arg(sort)::text = '-created_at' THEN c.created_at END DESC,
CASE WHEN sqlc.arg(sort)::text = 'name' THEN c.name END ASC,
CASE WHEN sqlc.arg(sort)::text = '-name' THEN c.name END DESC,
CASE WHEN sqlc.arg(sort)::text = 'status' THEN c.status END ASC,
CASE WHEN sqlc.arg(sort)::text = '-status' THEN c.status END DESC, c.id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountCompanyAssetCards :one
SELECT count(*)
FROM nfc_cards c
LEFT JOIN profiles p ON p.account_id = c.account_id AND p.id = c.profile_id AND p.deleted_at IS NULL
LEFT JOIN routes r ON r.account_id = c.account_id AND r.card_id = c.id
    AND r.route_type = 'card_route' AND r.deleted_at IS NULL
LEFT JOIN company_member_profiles l ON l.account_id = c.account_id AND l.profile_id = c.profile_id
LEFT JOIN account_members m ON m.account_id = c.account_id AND m.id = l.member_id
LEFT JOIN users u ON u.id = m.user_id
LEFT JOIN company_member_assignments a ON a.account_id = m.account_id AND a.member_id = m.id
LEFT JOIN company_offices off ON off.account_id = c.account_id AND off.id = a.office_id
LEFT JOIN company_groups g ON g.account_id = c.account_id AND g.id = a.group_id
WHERE c.account_id = sqlc.arg(account_id) AND c.deleted_at IS NULL
AND (sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR (sqlc.arg(actor_role)::text = 'member' AND m.user_id = sqlc.arg(actor_id))
    OR (sqlc.arg(actor_role)::text = 'office_manager' AND EXISTS (
        SELECT 1 FROM account_member_scopes s JOIN account_members actor ON actor.id = s.member_id
        WHERE s.account_id = m.account_id AND actor.user_id = sqlc.arg(actor_id) AND s.office_id = a.office_id)))
AND (sqlc.narg(office_uuid)::uuid IS NULL OR off.uuid = sqlc.narg(office_uuid))
AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid))
AND (sqlc.arg(status)::text = '' OR c.status = sqlc.arg(status))
AND (sqlc.narg(is_active)::boolean IS NULL OR c.is_active = sqlc.narg(is_active))
AND (sqlc.arg(q)::text = '' OR c.name ILIKE '%' || sqlc.arg(q) || '%'
    OR c.smart_code ILIKE '%' || sqlc.arg(q) || '%' OR c.card_uid ILIKE '%' || sqlc.arg(q) || '%'
    OR (u.name || ' ' || u.surname) ILIKE '%' || sqlc.arg(q) || '%' OR u.email ILIKE '%' || sqlc.arg(q) || '%');

-- name: ListCompanyAssetOrders :many
SELECT o.uuid, o.order_number, o.status, o.payment_status, o.fulfillment_status, o.currency,
o.grand_total::text AS grand_total, o.created_at,
(SELECT u.uuid FROM users u WHERE u.id = o.user_id) AS placed_by_uuid
FROM orders o
WHERE o.account_id = sqlc.arg(account_id) AND o.deleted_at IS NULL
AND (sqlc.arg(status)::text = '' OR o.status = sqlc.arg(status))
AND (sqlc.arg(q)::text = '' OR o.order_number ILIKE '%' || sqlc.arg(q) || '%'
    OR EXISTS (SELECT 1 FROM users u WHERE u.id = o.user_id
        AND (u.email ILIKE '%' || sqlc.arg(q) || '%' OR (u.name || ' ' || u.surname) ILIKE '%' || sqlc.arg(q) || '%')))
AND (
    (sqlc.arg(actor_role)::text IN ('owner','admin') AND sqlc.narg(office_uuid)::uuid IS NULL AND sqlc.narg(group_uuid)::uuid IS NULL)
    OR EXISTS (
        SELECT 1 FROM account_members m
        LEFT JOIN company_member_assignments a ON a.account_id = m.account_id AND a.member_id = m.id
        LEFT JOIN company_offices off ON off.account_id = m.account_id AND off.id = a.office_id
        LEFT JOIN company_groups g ON g.account_id = m.account_id AND g.id = a.group_id
        WHERE m.account_id = o.account_id
        AND (m.user_id = o.user_id OR EXISTS (
            SELECT 1 FROM order_items oi JOIN company_member_profiles l
                ON l.account_id = m.account_id AND l.member_id = m.id AND l.profile_id = oi.profile_id
            WHERE oi.order_id = o.id))
        AND (sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR (sqlc.arg(actor_role)::text = 'member' AND m.user_id = sqlc.arg(actor_id))
    OR (sqlc.arg(actor_role)::text = 'office_manager' AND EXISTS (
        SELECT 1 FROM account_member_scopes s JOIN account_members actor ON actor.id = s.member_id
        WHERE s.account_id = m.account_id AND actor.user_id = sqlc.arg(actor_id) AND s.office_id = a.office_id)))
        AND (sqlc.narg(office_uuid)::uuid IS NULL OR off.uuid = sqlc.narg(office_uuid))
        AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid))
    )
)
ORDER BY CASE WHEN sqlc.arg(sort)::text = 'created_at' THEN o.created_at END ASC,
CASE WHEN sqlc.arg(sort)::text = '-created_at' THEN o.created_at END DESC,
CASE WHEN sqlc.arg(sort)::text = 'order_number' THEN o.order_number END ASC,
CASE WHEN sqlc.arg(sort)::text = '-order_number' THEN o.order_number END DESC,
CASE WHEN sqlc.arg(sort)::text = 'status' THEN o.status END ASC,
CASE WHEN sqlc.arg(sort)::text = '-status' THEN o.status END DESC, o.id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountCompanyAssetOrders :one
SELECT count(*)
FROM orders o
WHERE o.account_id = sqlc.arg(account_id) AND o.deleted_at IS NULL
AND (sqlc.arg(status)::text = '' OR o.status = sqlc.arg(status))
AND (sqlc.arg(q)::text = '' OR o.order_number ILIKE '%' || sqlc.arg(q) || '%'
    OR EXISTS (SELECT 1 FROM users u WHERE u.id = o.user_id
        AND (u.email ILIKE '%' || sqlc.arg(q) || '%' OR (u.name || ' ' || u.surname) ILIKE '%' || sqlc.arg(q) || '%')))
AND (
    (sqlc.arg(actor_role)::text IN ('owner','admin') AND sqlc.narg(office_uuid)::uuid IS NULL AND sqlc.narg(group_uuid)::uuid IS NULL)
    OR EXISTS (
        SELECT 1 FROM account_members m
        LEFT JOIN company_member_assignments a ON a.account_id = m.account_id AND a.member_id = m.id
        LEFT JOIN company_offices off ON off.account_id = m.account_id AND off.id = a.office_id
        LEFT JOIN company_groups g ON g.account_id = m.account_id AND g.id = a.group_id
        WHERE m.account_id = o.account_id
        AND (m.user_id = o.user_id OR EXISTS (
            SELECT 1 FROM order_items oi JOIN company_member_profiles l
                ON l.account_id = m.account_id AND l.member_id = m.id AND l.profile_id = oi.profile_id
            WHERE oi.order_id = o.id))
        AND (sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR (sqlc.arg(actor_role)::text = 'member' AND m.user_id = sqlc.arg(actor_id))
    OR (sqlc.arg(actor_role)::text = 'office_manager' AND EXISTS (
        SELECT 1 FROM account_member_scopes s JOIN account_members actor ON actor.id = s.member_id
        WHERE s.account_id = m.account_id AND actor.user_id = sqlc.arg(actor_id) AND s.office_id = a.office_id)))
        AND (sqlc.narg(office_uuid)::uuid IS NULL OR off.uuid = sqlc.narg(office_uuid))
        AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid))
    )
);

-- name: GetCompanyAssetCardForUpdate :one
SELECT c.id, c.profile_id, c.is_active, c.status, c.order_id
FROM nfc_cards c WHERE c.account_id = $1 AND c.uuid = $2 AND c.deleted_at IS NULL
FOR UPDATE;

-- name: LockCompanyAssetOrder :one
-- Activation may move a card to a new account without moving its purchase order.
-- Follow only the active account's owned card; never accept an arbitrary foreign order ID.
SELECT o.status, o.fulfillment_status, o.payment_status FROM orders o
JOIN nfc_cards c ON c.order_id = o.id
WHERE c.account_id = sqlc.arg(account_id) AND c.id = sqlc.arg(card_id)
  AND c.deleted_at IS NULL AND o.deleted_at IS NULL
FOR UPDATE OF o;

-- name: LockCompanyAssetRoute :one
SELECT uuid, status, path, is_immutable FROM routes
WHERE account_id = $1 AND card_id = $2 AND route_type = 'card_route' AND deleted_at IS NULL
FOR UPDATE;

-- name: CompanyAssetProfileMember :one
SELECT u.uuid AS user_uuid, p.uuid AS profile_uuid, p.id AS profile_id,
u.name, u.surname, u.email, off.uuid AS office_uuid, COALESCE(g.name, '')::text AS group_name
FROM company_member_profiles l
JOIN profiles p ON p.account_id = l.account_id AND p.id = l.profile_id AND p.deleted_at IS NULL
JOIN account_members m ON m.account_id = l.account_id AND m.id = l.member_id
JOIN users u ON u.id = m.user_id
LEFT JOIN company_member_assignments a ON a.account_id = m.account_id AND a.member_id = m.id
LEFT JOIN company_offices off ON off.account_id = m.account_id AND off.id = a.office_id
LEFT JOIN company_groups g ON g.account_id = m.account_id AND g.id = a.group_id
WHERE l.account_id = sqlc.arg(account_id)
AND (sqlc.narg(user_uuid)::uuid IS NULL OR u.uuid = sqlc.narg(user_uuid))
AND (sqlc.narg(profile_id)::bigint IS NULL OR p.id = sqlc.narg(profile_id))
FOR SHARE OF l, p, m;

-- name: AssignCompanyAssetCard :exec
UPDATE nfc_cards SET profile_id = $3, target_type = 'profile', direct_url = ''
WHERE account_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: AssignCompanyAssetRoute :exec
UPDATE routes SET profile_id = $3, target_type = 'profile', direct_url = ''
WHERE account_id = $1 AND card_id = $2 AND route_type = 'card_route' AND deleted_at IS NULL;

-- name: SetCompanyAssetCardActive :exec
UPDATE nfc_cards SET is_active = $3 WHERE account_id = $1 AND id = $2 AND deleted_at IS NULL;
