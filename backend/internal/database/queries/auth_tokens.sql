-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address, impersonator_user_id, active_account_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- RotateRefreshTokenByHash atomically revokes a valid refresh token and issues its
-- successor in the same family (no window without an active row).
-- name: RotateRefreshTokenByHash :one
WITH old AS (
    UPDATE refresh_tokens
    SET revoked_at = NOW()
    WHERE refresh_tokens.token_hash = sqlc.arg(old_token_hash)
      AND refresh_tokens.revoked_at IS NULL
      AND refresh_tokens.expires_at > NOW()
    RETURNING refresh_tokens.user_id, refresh_tokens.impersonator_user_id,
              refresh_tokens.family_uuid, refresh_tokens.active_account_id
)
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address, impersonator_user_id, family_uuid, active_account_id)
SELECT old.user_id, sqlc.arg(new_token_hash), sqlc.arg(expires_at), sqlc.narg(user_agent), sqlc.narg(ip_address),
       old.impersonator_user_id, old.family_uuid, old.active_account_id
FROM old
RETURNING *;

-- RotateRefreshFamilyWithAccount re-issues the active token of a family with a new active account.
-- name: RotateRefreshFamilyWithAccount :one
WITH old AS (
    UPDATE refresh_tokens
    SET revoked_at = NOW()
    WHERE refresh_tokens.family_uuid = sqlc.arg(family_uuid)
      AND refresh_tokens.user_id = sqlc.arg(user_id)
      AND refresh_tokens.revoked_at IS NULL
      AND refresh_tokens.expires_at > NOW()
    RETURNING refresh_tokens.user_id, refresh_tokens.user_agent, refresh_tokens.ip_address,
              refresh_tokens.impersonator_user_id, refresh_tokens.family_uuid
)
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address, impersonator_user_id, family_uuid, active_account_id)
SELECT old.user_id, sqlc.arg(new_token_hash), sqlc.arg(expires_at),
       old.user_agent, old.ip_address, old.impersonator_user_id, old.family_uuid, sqlc.arg(active_account_id)
FROM old
RETURNING *;

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
