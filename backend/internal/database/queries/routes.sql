-- account-lint: cross-account public route resolver (/{path}); paths are globally unique
-- name: GetRouteByPath :one
SELECT 
    r.*,
    p.uuid AS profile_uuid,
    p.avatar_url AS profile_avatar_url,
    p.primary_locale AS profile_primary_locale,
    COALESCE((
        SELECT cr.path FROM routes cr
        WHERE cr.profile_id = p.id AND cr.route_type = 'canonical_profile'
          AND cr.status = 'active' AND cr.deleted_at IS NULL
        LIMIT 1
    ), '')::text AS profile_username,
    c.uuid AS card_uuid,
    c.name AS card_name,
    c.is_locked AS card_is_locked,
    c.is_active AS card_is_active,
    c.lead_capture_enabled AS card_lead_capture_enabled,
    c.passcode AS card_passcode,
    c.target_type AS card_target_type,
    c.direct_url AS card_direct_url
FROM routes r
LEFT JOIN nfc_cards c ON c.id = r.card_id AND c.deleted_at IS NULL
-- A card route follows its card's current profile; other routes use their own.
LEFT JOIN profiles p ON p.id = COALESCE(c.profile_id, r.profile_id) AND p.deleted_at IS NULL
WHERE LOWER(TRIM(r.path)) = LOWER(TRIM(sqlc.arg(path)))
  AND r.deleted_at IS NULL;

-- account-lint: cross-account usecase checks route.AccountID against the active account (404 otherwise)
-- name: GetRouteByUUID :one
SELECT 
    r.*,
    p.uuid AS profile_uuid,
    p.avatar_url AS profile_avatar_url,
    p.primary_locale AS profile_primary_locale,
    c.uuid AS card_uuid,
    c.name AS card_name
FROM routes r
LEFT JOIN profiles p ON p.id = r.profile_id AND p.deleted_at IS NULL
LEFT JOIN nfc_cards c ON c.id = r.card_id AND c.deleted_at IS NULL
WHERE r.uuid = sqlc.arg(uuid)
  AND r.deleted_at IS NULL;

-- name: ListRoutes :many
SELECT 
    r.*,
    p.uuid AS profile_uuid,
    c.uuid AS card_uuid
FROM routes r
LEFT JOIN profiles p ON p.id = r.profile_id AND p.deleted_at IS NULL
LEFT JOIN nfc_cards c ON c.id = r.card_id AND c.deleted_at IS NULL
WHERE r.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(route_type)::text IS NULL OR r.route_type = sqlc.narg(route_type)::text)
  AND (sqlc.narg(status)::text IS NULL OR r.status = sqlc.narg(status)::text)
  AND (sqlc.narg(q)::text IS NULL OR r.path ILIKE '%' || sqlc.narg(q) || '%')
  AND r.deleted_at IS NULL
ORDER BY r.created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountRoutes :one
SELECT COUNT(*)::bigint
FROM routes r
WHERE r.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(route_type)::text IS NULL OR r.route_type = sqlc.narg(route_type)::text)
  AND (sqlc.narg(status)::text IS NULL OR r.status = sqlc.narg(status)::text)
  AND (sqlc.narg(q)::text IS NULL OR r.path ILIKE '%' || sqlc.narg(q) || '%')
  AND r.deleted_at IS NULL;

-- name: CreateRoute :one
INSERT INTO routes (
    path,
    route_type,
    status,
    is_immutable,
    target_type,
    target_id,
    profile_id,
    card_id,
    direct_url,
    account_id,
    owner_user_id,
    presentation_config,
    legacy_metadata
) VALUES (
    LOWER(TRIM(sqlc.arg(path))),
    sqlc.arg(route_type),
    sqlc.arg(status),
    sqlc.arg(is_immutable),
    sqlc.arg(target_type),
    sqlc.narg(target_id),
    sqlc.narg(profile_id),
    sqlc.narg(card_id),
    sqlc.arg(direct_url),
    sqlc.narg(account_id),
    sqlc.narg(owner_user_id),
    sqlc.arg(presentation_config),
    sqlc.arg(legacy_metadata)
)
RETURNING *;

-- name: UpdateRoute :one
UPDATE routes
SET
    path = CASE WHEN is_immutable THEN path ELSE COALESCE(LOWER(TRIM(sqlc.narg(path))), path) END,
    target_type = COALESCE(sqlc.narg(target_type), target_type),
    target_id = CASE WHEN sqlc.arg(set_target_id)::boolean THEN sqlc.narg(target_id) ELSE target_id END,
    profile_id = CASE WHEN sqlc.arg(set_profile_id)::boolean THEN sqlc.narg(profile_id) ELSE profile_id END,
    card_id = CASE WHEN sqlc.arg(set_card_id)::boolean THEN sqlc.narg(card_id) ELSE card_id END,
    direct_url = COALESCE(sqlc.narg(direct_url), direct_url),
    status = COALESCE(sqlc.narg(status), status),
    presentation_config = COALESCE(sqlc.narg(presentation_config), presentation_config),
    legacy_metadata = CASE WHEN is_immutable THEN legacy_metadata ELSE COALESCE(sqlc.narg(legacy_metadata), legacy_metadata) END
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteRoute :exec
UPDATE routes
SET deleted_at = NOW(),
    status = 'retired'
WHERE uuid = sqlc.arg(uuid)
  AND is_immutable = false
  AND account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL;

-- account-lint: cross-account route paths are globally unique
-- name: CheckPathExists :one
-- A deleted or renamed smart link keeps its old path for 30 days (TEC-559/558, architecture
-- decision #4) so nobody can take over a link that is still printed or shared. Platform staff
-- can release that hold early (TEC-950, quarantine_released_at).
SELECT (EXISTS(
    SELECT 1 FROM routes
    WHERE LOWER(TRIM(path)) = LOWER(TRIM(sqlc.arg(path)))
      AND (deleted_at IS NULL
           OR (route_type = 'custom_smart_link' AND deleted_at > NOW() - INTERVAL '30 days'
               AND quarantine_released_at IS NULL))
      AND (sqlc.narg(exclude_uuid)::uuid IS NULL OR uuid != sqlc.narg(exclude_uuid)::uuid)
) OR EXISTS(
    -- A renamed smart link's old path is held the same way (TEC-558); the link itself may
    -- take its old path back.
    SELECT 1 FROM route_history h
    JOIN routes hr ON hr.id = h.route_id
    WHERE LOWER(h.old_path) = LOWER(TRIM(sqlc.arg(path)))
      AND h.reason = 'smart_link_rename'
      AND h.created_at > NOW() - INTERVAL '30 days'
      AND h.quarantine_released_at IS NULL
      AND (sqlc.narg(exclude_uuid)::uuid IS NULL OR hr.uuid != sqlc.narg(exclude_uuid)::uuid)
))::boolean AS taken;

-- account-lint: cross-account card id comes from an account-scoped card (no app caller takes it from input)
-- name: GetRouteByCardID :one
SELECT * FROM routes
WHERE card_id = sqlc.arg(card_id)
  AND deleted_at IS NULL
LIMIT 1;

-- name: InsertRouteHistory :exec
INSERT INTO route_history (route_id, route_type, account_id, old_path, new_path, changed_by_user_id, reason)
VALUES (
    sqlc.arg(route_id),
    sqlc.arg(route_type),
    sqlc.narg(account_id),
    sqlc.arg(old_path),
    sqlc.arg(new_path),
    sqlc.narg(changed_by_user_id),
    sqlc.arg(reason)
);
