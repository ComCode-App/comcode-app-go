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

-- name: UserHasActivePushDevices :one
-- Cheap check before queueing a mobile push (TEC-1187). The send task re-checks the session.
SELECT EXISTS (
    SELECT 1 FROM push_devices WHERE user_id = $1 AND disabled_at IS NULL
)::bool AS has_devices;

-- name: ListPushTargetsForNotification :many
-- Devices that may receive a notification's push (TEC-1187): not disabled, and the session
-- family they registered from still has a live refresh token. A family that simply expired is
-- not marked disabled (auth.md, "Push devices follow the session"), so the check is here.
-- Devices that already hold a ticket for the notification are skipped, so a retried send task
-- does not push twice.
SELECT d.* FROM push_devices d
WHERE d.user_id = sqlc.arg(user_id)
  AND d.disabled_at IS NULL
  AND EXISTS (
      SELECT 1 FROM refresh_tokens r
      WHERE r.family_uuid = d.session_family_uuid
        AND r.user_id = d.user_id
        AND r.revoked_at IS NULL
        AND r.expires_at > NOW()
  )
  AND NOT EXISTS (
      SELECT 1 FROM push_tickets t
      WHERE t.push_device_id = d.id AND t.notification_id = sqlc.arg(notification_id)
  )
ORDER BY d.id;

-- name: DisablePushDeviceNotRegistered :execrows
-- Expo answered DeviceNotRegistered (ticket or receipt): the app was uninstalled or the token
-- is dead. Registering again from a live session re-enables the row.
UPDATE push_devices
SET disabled_at = NOW(), disabled_reason = 'device_not_registered'
WHERE id = $1 AND disabled_at IS NULL;

-- name: InsertPushTicket :exec
INSERT INTO push_tickets (ticket_id, push_device_id, notification_id)
VALUES (sqlc.arg(ticket_id), sqlc.arg(push_device_id), sqlc.narg(notification_id))
ON CONFLICT (ticket_id) DO NOTHING;

-- name: ListDuePushTickets :many
-- Tickets whose receipt can be read (Expo: about 15 minutes after sending), oldest first.
SELECT id, ticket_id, push_device_id, created_at
FROM push_tickets
WHERE created_at <= sqlc.arg(before_at)
ORDER BY id
LIMIT sqlc.arg(limit_count);

-- name: DeletePushTickets :exec
DELETE FROM push_tickets WHERE id = ANY(sqlc.arg(ids)::bigint[]);

-- name: DeleteExpiredPushTickets :execrows
-- Maintenance: Expo keeps receipts about 24 hours; an unprocessed ticket after that is useless.
DELETE FROM push_tickets WHERE created_at < NOW() - interval '24 hours';

-- name: DeleteDisabledPushDevices :execrows
-- Maintenance: devices disabled for more than 30 days (signed out, revoked, not registered).
DELETE FROM push_devices
WHERE disabled_at IS NOT NULL AND disabled_at < NOW() - interval '30 days';

-- name: DeleteStalePushDevices :execrows
-- Maintenance: the app re-registers on every start, so a device not seen for 270 days is gone.
DELETE FROM push_devices WHERE last_seen_at < NOW() - interval '270 days';
