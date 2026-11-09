-- Platform card and route support screens (TEC-950). These are platform queries: they are not
-- scoped to an account and are only reachable behind platform.cards.* / platform.routes.*.

-- name: PlatformSearchCards :many
-- Prefix match on smart code, card UID (system serial) or a migrated card's legacy serial.
SELECT
    c.id, c.uuid, c.name, c.smart_code, c.card_uid, c.target_type, c.direct_url,
    c.is_active, c.is_locked, c.status, c.created_at, c.updated_at,
    (c.activation_code IS NOT NULL)::boolean AS pending_activation,
    a.uuid AS account_uuid, a.name AS account_name, a.type::text AS account_type,
    u.uuid AS owner_uuid, u.email AS owner_email, u.name AS owner_name, u.surname AS owner_surname,
    p.uuid AS profile_uuid,
    COALESCE((
        SELECT pr.path FROM routes pr
        WHERE pr.profile_id = p.id AND pr.route_type = 'canonical_profile'
          AND pr.status = 'active' AND pr.deleted_at IS NULL
        LIMIT 1
    ), '')::text AS profile_username,
    o.uuid AS order_uuid, o.order_number AS order_number, o.status AS order_status,
    r.uuid AS route_uuid, r.path AS route_path, r.status AS route_status,
    COALESCE(r.legacy_metadata->>'card_serial_number', '')::text AS legacy_serial
FROM nfc_cards c
JOIN accounts a ON a.id = c.account_id
JOIN users u ON u.id = c.user_id
LEFT JOIN profiles p ON p.id = c.profile_id AND p.deleted_at IS NULL
LEFT JOIN orders o ON o.id = c.order_id
LEFT JOIN routes r ON r.card_id = c.id AND r.route_type = 'card_route' AND r.deleted_at IS NULL
WHERE c.deleted_at IS NULL
  AND (LOWER(c.smart_code) LIKE LOWER(sqlc.arg(prefix)::text) || '%'
       OR LOWER(COALESCE(c.card_uid, '')) LIKE LOWER(sqlc.arg(prefix)::text) || '%'
       OR LOWER(COALESCE(r.legacy_metadata->>'card_serial_number', '')) LIKE LOWER(sqlc.arg(prefix)::text) || '%')
ORDER BY (LOWER(c.smart_code) = LOWER(sqlc.arg(prefix)::text)
          OR LOWER(COALESCE(c.card_uid, '')) = LOWER(sqlc.arg(prefix)::text)) DESC,
         c.created_at DESC
LIMIT sqlc.arg(limit_count);

-- name: PlatformGetCard :one
SELECT
    c.id, c.uuid, c.name, c.smart_code, c.card_uid, c.target_type, c.direct_url,
    c.is_active, c.is_locked, c.status, c.created_at, c.updated_at,
    (c.activation_code IS NOT NULL)::boolean AS pending_activation,
    a.uuid AS account_uuid, a.name AS account_name, a.type::text AS account_type,
    u.uuid AS owner_uuid, u.email AS owner_email, u.name AS owner_name, u.surname AS owner_surname,
    p.uuid AS profile_uuid,
    COALESCE((
        SELECT pr.path FROM routes pr
        WHERE pr.profile_id = p.id AND pr.route_type = 'canonical_profile'
          AND pr.status = 'active' AND pr.deleted_at IS NULL
        LIMIT 1
    ), '')::text AS profile_username,
    o.uuid AS order_uuid, o.order_number AS order_number, o.status AS order_status,
    r.uuid AS route_uuid, r.path AS route_path, r.status AS route_status,
    COALESCE(r.legacy_metadata->>'card_serial_number', '')::text AS legacy_serial
FROM nfc_cards c
JOIN accounts a ON a.id = c.account_id
JOIN users u ON u.id = c.user_id
LEFT JOIN profiles p ON p.id = c.profile_id AND p.deleted_at IS NULL
LEFT JOIN orders o ON o.id = c.order_id
LEFT JOIN routes r ON r.card_id = c.id AND r.route_type = 'card_route' AND r.deleted_at IS NULL
WHERE c.uuid = sqlc.arg(uuid)
  AND c.deleted_at IS NULL;

-- name: PlatformRetireCard :many
-- One card's version of RetireOrderCardRoutes (TEC-585): its route becomes retired ("card
-- deactivated", the path is never handed out again) and an unused activation code is cleared.
WITH retired AS (
    UPDATE routes
    SET status = 'retired'
    WHERE card_id = sqlc.arg(card_id)
      AND deleted_at IS NULL
      AND status <> 'retired'
    RETURNING path
), cleared AS (
    UPDATE nfc_cards
    SET activation_code = NULL
    WHERE id = sqlc.arg(card_id)
      AND activation_code IS NOT NULL
    RETURNING id
)
SELECT path FROM retired;

-- name: PlatformGetProfileForCard :one
-- The profile a card is reassigned to, with the user who will own the card: the profile's user,
-- or the account owner for profiles without one (legacy/company profiles).
SELECT
    p.id, p.uuid, p.account_id,
    COALESCE(p.user_id, (
        SELECT m.user_id FROM account_members m
        WHERE m.account_id = p.account_id AND m.role = 'owner'
        ORDER BY m.id
        LIMIT 1
    ), 0)::bigint AS owner_user_id
FROM profiles p
WHERE p.uuid = sqlc.arg(uuid)
  AND p.deleted_at IS NULL;

-- name: PlatformReassignCard :one
-- The card and its card route move to the profile, its account and owner in one statement. The
-- printed identity (smart_code, card_uid, path) does not change.
WITH card AS (
    UPDATE nfc_cards
    SET profile_id = sqlc.arg(profile_id),
        target_type = 'profile',
        account_id = sqlc.arg(account_id),
        user_id = sqlc.arg(user_id)
    WHERE nfc_cards.id = sqlc.arg(card_id)
      AND nfc_cards.deleted_at IS NULL
    RETURNING nfc_cards.id, nfc_cards.account_id, nfc_cards.user_id, nfc_cards.profile_id
), card_route AS (
    UPDATE routes
    SET profile_id = card.profile_id,
        target_type = 'profile',
        account_id = card.account_id,
        owner_user_id = card.user_id
    FROM card
    WHERE routes.card_id = card.id
      AND routes.deleted_at IS NULL
    RETURNING routes.id
)
SELECT card.id FROM card;

-- name: PlatformSearchRoutes :many
-- Path search over every route, deleted smart links included (they hold their path for 30 days),
-- plus routes whose renamed-away old path matches.
SELECT r.id
FROM routes r
WHERE LOWER(r.path) LIKE '%' || LOWER(sqlc.arg(pattern)::text) || '%'
   OR EXISTS (
        SELECT 1 FROM route_history h
        WHERE h.route_id = r.id
          AND LOWER(h.old_path) LIKE '%' || LOWER(sqlc.arg(pattern)::text) || '%'
   )
ORDER BY (LOWER(r.path) = LOWER(sqlc.arg(pattern)::text)) DESC,
         (r.deleted_at IS NULL) DESC,
         r.created_at DESC
LIMIT sqlc.arg(limit_count);

-- name: PlatformGetRoutes :many
SELECT
    r.id, r.uuid, r.path, r.route_type, r.status, r.is_immutable, r.target_type, r.direct_url,
    r.created_at, r.updated_at, r.deleted_at, r.quarantine_released_at,
    (r.deleted_at IS NOT NULL AND r.route_type = 'custom_smart_link'
        AND r.deleted_at > NOW() - INTERVAL '30 days'
        AND r.quarantine_released_at IS NULL)::boolean AS quarantined,
    a.uuid AS account_uuid, a.name AS account_name,
    u.uuid AS owner_uuid, u.email AS owner_email, u.name AS owner_name, u.surname AS owner_surname,
    p.uuid AS profile_uuid,
    COALESCE((
        SELECT pr.path FROM routes pr
        WHERE pr.profile_id = p.id AND pr.route_type = 'canonical_profile'
          AND pr.status = 'active' AND pr.deleted_at IS NULL
        LIMIT 1
    ), '')::text AS profile_username,
    c.uuid AS card_uuid, c.smart_code AS card_smart_code
FROM routes r
LEFT JOIN accounts a ON a.id = r.account_id
LEFT JOIN users u ON u.id = r.owner_user_id
LEFT JOIN nfc_cards c ON c.id = r.card_id AND c.deleted_at IS NULL
-- A card route follows its card's current profile; other routes use their own.
LEFT JOIN profiles p ON p.id = COALESCE(c.profile_id, r.profile_id) AND p.deleted_at IS NULL
WHERE r.id = ANY(sqlc.arg(ids)::bigint[]);

-- name: PlatformGetRouteIDByUUID :one
SELECT id FROM routes WHERE uuid = sqlc.arg(uuid);

-- name: PlatformListRouteHistory :many
SELECT
    h.id, h.route_id, h.old_path, h.new_path, h.reason, h.created_at, h.quarantine_released_at,
    (h.reason = 'smart_link_rename' AND h.created_at > NOW() - INTERVAL '30 days'
        AND h.quarantine_released_at IS NULL)::boolean AS quarantined,
    u.uuid AS changed_by_uuid, u.email AS changed_by_email
FROM route_history h
LEFT JOIN users u ON u.id = h.changed_by_user_id
WHERE h.route_id = ANY(sqlc.arg(route_ids)::bigint[])
ORDER BY h.created_at DESC, h.id DESC;

-- name: PlatformReleaseDeletedRoutePath :many
-- Ends the 30-day hold of deleted smart links on this path (TEC-559 hold, TEC-950 release).
UPDATE routes
SET quarantine_released_at = NOW()
WHERE LOWER(TRIM(path)) = LOWER(TRIM(sqlc.arg(path)::text))
  AND route_type = 'custom_smart_link'
  AND deleted_at IS NOT NULL
  AND deleted_at > NOW() - INTERVAL '30 days'
  AND quarantine_released_at IS NULL
RETURNING uuid;

-- name: PlatformReleaseRenamedPath :many
-- Ends the 30-day hold a smart link rename left on its old path (TEC-558 hold, TEC-950 release).
UPDATE route_history h
SET quarantine_released_at = NOW()
FROM routes r
WHERE r.id = h.route_id
  AND LOWER(h.old_path) = LOWER(TRIM(sqlc.arg(path)::text))
  AND h.reason = 'smart_link_rename'
  AND h.created_at > NOW() - INTERVAL '30 days'
  AND h.quarantine_released_at IS NULL
RETURNING r.uuid;
