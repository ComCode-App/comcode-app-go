-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address, impersonator_user_id, active_account_id,
                            client_type, device_name, app_version)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- RotateRefreshTokenByHash atomically revokes a valid refresh token and issues its
-- successor in the same family (no window without an active row). The old row is marked
-- rotated (reuse detection, TEC-1179). The family keeps its client_type and device_name;
-- expires_at follows the family's client type, app_version follows the latest request.
-- name: RotateRefreshTokenByHash :one
WITH old AS (
    UPDATE refresh_tokens
    SET revoked_at = NOW(), rotated_at = NOW()
    WHERE refresh_tokens.token_hash = sqlc.arg(old_token_hash)
      AND refresh_tokens.revoked_at IS NULL
      AND refresh_tokens.expires_at > NOW()
    RETURNING refresh_tokens.user_id, refresh_tokens.impersonator_user_id,
              refresh_tokens.family_uuid, refresh_tokens.active_account_id,
              refresh_tokens.client_type, refresh_tokens.device_name, refresh_tokens.app_version
)
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address, impersonator_user_id, family_uuid, active_account_id,
                            client_type, device_name, app_version)
SELECT old.user_id, sqlc.arg(new_token_hash),
       CASE WHEN old.client_type = 'mobile' THEN sqlc.arg(mobile_expires_at)::timestamptz ELSE sqlc.arg(web_expires_at)::timestamptz END,
       sqlc.narg(user_agent), sqlc.narg(ip_address),
       old.impersonator_user_id, old.family_uuid, old.active_account_id,
       old.client_type, old.device_name, COALESCE(sqlc.narg(app_version)::text, old.app_version)
FROM old
RETURNING *;

-- RotateRefreshFamilyWithAccount re-issues the active token of a family with a new active account.
-- name: RotateRefreshFamilyWithAccount :one
WITH old AS (
    UPDATE refresh_tokens
    SET revoked_at = NOW(), rotated_at = NOW()
    WHERE refresh_tokens.family_uuid = sqlc.arg(family_uuid)
      AND refresh_tokens.user_id = sqlc.arg(user_id)
      AND refresh_tokens.revoked_at IS NULL
      AND refresh_tokens.expires_at > NOW()
    RETURNING refresh_tokens.user_id, refresh_tokens.user_agent, refresh_tokens.ip_address,
              refresh_tokens.impersonator_user_id, refresh_tokens.family_uuid,
              refresh_tokens.client_type, refresh_tokens.device_name, refresh_tokens.app_version
)
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address, impersonator_user_id, family_uuid, active_account_id,
                            client_type, device_name, app_version)
SELECT old.user_id, sqlc.arg(new_token_hash),
       CASE WHEN old.client_type = 'mobile' THEN sqlc.arg(mobile_expires_at)::timestamptz ELSE sqlc.arg(web_expires_at)::timestamptz END,
       old.user_agent, old.ip_address, old.impersonator_user_id, old.family_uuid, sqlc.arg(active_account_id),
       old.client_type, old.device_name, old.app_version
FROM old
RETURNING *;

-- GetRotatedRefreshToken finds a refresh token that was already rotated (reuse detection,
-- TEC-1179). within_race is true while the rotation is younger than race_seconds (two
-- requests refreshing the same token concurrently).
-- name: GetRotatedRefreshToken :one
SELECT user_id, family_uuid, client_type, device_name, user_agent,
       COALESCE(rotated_at > NOW() - make_interval(secs => sqlc.arg(race_seconds)::float8), FALSE)::boolean AS within_race
FROM refresh_tokens
WHERE token_hash = sqlc.arg(token_hash)
  AND rotated_at IS NOT NULL;

-- name: GetActiveRefreshFamily :one
SELECT *
FROM refresh_tokens
WHERE family_uuid = $1
  AND user_id = $2
  AND revoked_at IS NULL
  AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1;

-- name: GetValidRefreshTokenByHash :one
SELECT *
FROM refresh_tokens
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW();

-- RevokeRefreshFamilyByHash revokes every live token in the family of the given token (logout).
-- name: RevokeRefreshFamilyByHash :exec
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE family_uuid = (SELECT r.family_uuid FROM refresh_tokens r WHERE r.token_hash = $1)
  AND revoked_at IS NULL;

-- name: RevokeAllRefreshTokensForUser :exec
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE user_id = $1
  AND revoked_at IS NULL;

-- name: ListActiveRefreshTokensByUserID :many
SELECT DISTINCT ON (family_uuid) *
FROM refresh_tokens
WHERE user_id = $1
  AND revoked_at IS NULL
  AND expires_at > NOW()
ORDER BY family_uuid, created_at DESC;

-- name: RevokeRefreshFamilyForUser :execrows
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE family_uuid = $1
  AND user_id = $2
  AND revoked_at IS NULL;

-- name: RevokeOtherRefreshFamiliesForUser :exec
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE user_id = $1
  AND family_uuid <> $2
  AND revoked_at IS NULL;
