-- name: ListAccountCards :many
SELECT 
    c.*,
    p.uuid as profile_uuid,
    p.avatar_url as profile_avatar_url,
    p.primary_locale as profile_primary_locale,
    -- Card scans are analytics events of this route (subject {type: route, id: route_uuid}).
    (SELECT r.uuid FROM routes r
     WHERE r.card_id = c.id AND r.route_type = 'card_route' AND r.deleted_at IS NULL
     LIMIT 1) AS route_uuid
FROM nfc_cards c
LEFT JOIN profiles p ON p.id = c.profile_id AND p.deleted_at IS NULL
WHERE c.account_id = sqlc.arg(account_id)
  AND c.deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR c.name ILIKE '%' || sqlc.narg(q) || '%'
    OR c.smart_code ILIKE '%' || sqlc.narg(q) || '%'
    OR c.card_uid ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(is_active)::boolean IS NULL
    OR c.is_active = sqlc.narg(is_active)::boolean
  )
ORDER BY c.created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountAccountCards :one
SELECT COUNT(*)::bigint
FROM nfc_cards c
WHERE c.account_id = sqlc.arg(account_id)
  AND c.deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR c.name ILIKE '%' || sqlc.narg(q) || '%'
    OR c.smart_code ILIKE '%' || sqlc.narg(q) || '%'
    OR c.card_uid ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(is_active)::boolean IS NULL
    OR c.is_active = sqlc.narg(is_active)::boolean
  );

-- name: GetAccountCardByUUID :one
SELECT 
    c.*,
    p.uuid as profile_uuid,
    p.avatar_url as profile_avatar_url,
    p.primary_locale as profile_primary_locale
FROM nfc_cards c
LEFT JOIN profiles p ON p.id = c.profile_id AND p.deleted_at IS NULL
WHERE c.uuid = sqlc.arg(uuid)
  AND c.account_id = sqlc.arg(account_id)
  AND c.deleted_at IS NULL;

-- name: GetCardByUUIDAdmin :one
SELECT 
    c.*,
    p.uuid as profile_uuid,
    p.avatar_url as profile_avatar_url,
    p.primary_locale as profile_primary_locale,
    u.email as owner_email,
    u.name as owner_name,
    u.surname as owner_surname
FROM nfc_cards c
JOIN users u ON u.id = c.user_id
LEFT JOIN profiles p ON p.id = c.profile_id AND p.deleted_at IS NULL
WHERE c.uuid = sqlc.arg(uuid)
  AND c.deleted_at IS NULL;

-- name: GetCardBySmartCode :one
SELECT 
    c.*,
    p.uuid as profile_uuid,
    p.avatar_url as profile_avatar_url,
    p.primary_locale as profile_primary_locale
FROM nfc_cards c
LEFT JOIN profiles p ON p.id = c.profile_id AND p.deleted_at IS NULL
WHERE c.smart_code = sqlc.arg(smart_code)
  AND c.deleted_at IS NULL;

-- name: GetCardByActivationCode :one
SELECT *
FROM nfc_cards
WHERE activation_code = sqlc.arg(activation_code)
  AND deleted_at IS NULL;

-- CreateCard inserts an app-created card together with its active c/{smart_code} route, so the
-- card's QR / NFC link resolves immediately (provisioned cards get a reserved route instead).
-- name: CreateCard :one
WITH card AS (
INSERT INTO nfc_cards (
    account_id,
    user_id,
    name,
    card_uid,
    activation_code,
    smart_code,
    target_type,
    profile_id,
    direct_url,
    custom_data,
    card_type,
    material,
    color_hex,
    is_active,
    is_locked,
    lead_capture_enabled,
    passcode
) VALUES (
    sqlc.arg(account_id),
    sqlc.arg(user_id),
    sqlc.arg(name),
    sqlc.narg(card_uid),
    sqlc.narg(activation_code),
    sqlc.arg(smart_code),
    sqlc.arg(target_type),
    sqlc.narg(profile_id),
    sqlc.arg(direct_url),
    sqlc.arg(custom_data),
    sqlc.arg(card_type),
    sqlc.arg(material),
    sqlc.arg(color_hex),
    sqlc.arg(is_active),
    sqlc.arg(is_locked),
    sqlc.arg(lead_capture_enabled),
    sqlc.narg(passcode)
)
RETURNING *
), card_route AS (
    INSERT INTO routes (path, route_type, status, is_immutable, target_type, profile_id, card_id, account_id, owner_user_id)
    SELECT 'c/' || card.smart_code, 'card_route', 'active', true, 'profile', card.profile_id, card.id, card.account_id, card.user_id
    FROM card
    RETURNING routes.id
)
SELECT card.* FROM card;

-- name: CreateProvisionedCard :one
INSERT INTO nfc_cards (
    account_id,
    user_id,
    name,
    card_uid,
    activation_code,
    smart_code,
    target_type,
    profile_id,
    direct_url,
    custom_data,
    card_type,
    material,
    color_hex,
    is_active,
    is_locked,
    lead_capture_enabled,
    order_id,
    order_item_id,
    status
) VALUES (
    sqlc.arg(account_id),
    sqlc.arg(user_id),
    sqlc.arg(name),
    sqlc.narg(card_uid),
    sqlc.narg(activation_code),
    sqlc.arg(smart_code),
    sqlc.arg(target_type),
    sqlc.narg(profile_id),
    sqlc.arg(direct_url),
    sqlc.arg(custom_data),
    sqlc.arg(card_type),
    sqlc.arg(material),
    sqlc.arg(color_hex),
    sqlc.arg(is_active),
    sqlc.arg(is_locked),
    sqlc.arg(lead_capture_enabled),
    sqlc.narg(order_id),
    sqlc.narg(order_item_id),
    sqlc.arg(status)
)
RETURNING *;

-- name: UpdateCard :one
UPDATE nfc_cards
SET 
    name = COALESCE(sqlc.narg(name), name),
    target_type = COALESCE(sqlc.narg(target_type), target_type),
    profile_id = CASE WHEN sqlc.arg(set_profile_id)::boolean THEN sqlc.narg(profile_id) ELSE profile_id END,
    direct_url = COALESCE(sqlc.narg(direct_url), direct_url),
    custom_data = COALESCE(sqlc.narg(custom_data), custom_data),
    card_type = COALESCE(sqlc.narg(card_type), card_type),
    material = COALESCE(sqlc.narg(material), material),
    color_hex = COALESCE(sqlc.narg(color_hex), color_hex),
    is_active = COALESCE(sqlc.narg(is_active), is_active),
    is_locked = COALESCE(sqlc.narg(is_locked), is_locked),
    lead_capture_enabled = COALESCE(sqlc.narg(lead_capture_enabled), lead_capture_enabled),
    passcode = CASE WHEN sqlc.arg(set_passcode)::boolean THEN sqlc.narg(passcode) ELSE passcode END
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: ToggleCardLock :one
UPDATE nfc_cards
SET is_locked = NOT is_locked
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL
RETURNING is_locked;

-- ActivateCardByCode moves an unactivated card into the activating account and activates its
-- reserved card route in the same statement, so the card resolves as soon as it is activated.
-- name: ActivateCardByCode :one
WITH card AS (
    UPDATE nfc_cards
    SET account_id = sqlc.arg(account_id),
        user_id = sqlc.arg(user_id),
        name = COALESCE(sqlc.narg(name), nfc_cards.name),
        profile_id = sqlc.narg(profile_id),
        activation_code = NULL,
        is_active = true,
        status = 'active'
    WHERE nfc_cards.activation_code = sqlc.arg(activation_code)
      AND nfc_cards.deleted_at IS NULL
    RETURNING nfc_cards.*
), card_route AS (
    UPDATE routes
    SET status = 'active',
        account_id = card.account_id,
        owner_user_id = card.user_id,
        profile_id = card.profile_id,
        target_type = CASE WHEN card.profile_id IS NOT NULL THEN 'profile' ELSE routes.target_type END
    FROM card
    WHERE routes.card_id = card.id
      AND routes.route_type = 'card_route'
      AND routes.status = 'reserved'
      AND routes.deleted_at IS NULL
    RETURNING routes.id
)
SELECT card.* FROM card;

-- name: DeleteCard :exec
UPDATE nfc_cards
SET deleted_at = NOW()
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL;

-- name: CheckSmartCodeExists :one
SELECT EXISTS(
    SELECT 1 FROM nfc_cards
    WHERE smart_code = sqlc.arg(smart_code)
      AND deleted_at IS NULL
      AND (sqlc.narg(exclude_uuid)::uuid IS NULL OR uuid != sqlc.narg(exclude_uuid)::uuid)
);
