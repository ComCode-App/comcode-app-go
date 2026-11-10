-- name: UpsertPushDeviceForSession :one
-- Registers the token for the caller's live session family. No row comes back when the family
-- is revoked or expired, so an access token outliving its session cannot re-enable a device.
-- A token registered by another user moves to the caller (the device changed hands). Devices
-- are user-level, not account-owned; auth_tokens.sql disables them with their session family.
INSERT INTO push_devices (user_id, token, platform, locale, app_version, device_name, session_family_uuid, last_seen_at)
SELECT sqlc.arg(user_id), sqlc.arg(token), sqlc.arg(platform), sqlc.narg(locale), sqlc.narg(app_version),
       sqlc.narg(device_name), sqlc.arg(session_family_uuid)::uuid, NOW()
WHERE EXISTS (
    SELECT 1 FROM refresh_tokens r
    WHERE r.family_uuid = sqlc.arg(session_family_uuid)::uuid
      AND r.user_id = sqlc.arg(user_id)
      AND r.revoked_at IS NULL
      AND r.expires_at > NOW()
)
ON CONFLICT (token) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    platform = EXCLUDED.platform,
    locale = EXCLUDED.locale,
    app_version = EXCLUDED.app_version,
    device_name = EXCLUDED.device_name,
    session_family_uuid = EXCLUDED.session_family_uuid,
    last_seen_at = NOW(),
    disabled_at = NULL,
    disabled_reason = NULL
RETURNING *;

-- name: UserHasPushDevices :one
-- Any device, disabled ones included: a user who registered before is not "first" again, so an
-- explicit push_enabled = false survives signing in on the phone again.
SELECT EXISTS (SELECT 1 FROM push_devices WHERE user_id = $1)::bool AS has_devices;

-- name: EnablePushPreference :exec
-- First device registration opts the user into push (the OS permission prompt is the consent).
INSERT INTO notification_preferences (user_id, push_enabled)
VALUES ($1, TRUE)
ON CONFLICT (user_id) DO UPDATE SET push_enabled = TRUE;

-- name: DeletePushDeviceForUser :execrows
DELETE FROM push_devices
WHERE user_id = sqlc.arg(user_id) AND token = sqlc.arg(token);

-- name: ListPushDevicesByUser :many
SELECT * FROM push_devices
WHERE user_id = $1
ORDER BY (disabled_at IS NULL) DESC, last_seen_at DESC, id DESC;
