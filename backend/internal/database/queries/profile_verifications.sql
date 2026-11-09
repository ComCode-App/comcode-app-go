-- TEC-549: verified profile badge requests. Owner-side queries take account_id; platform
-- review queries are unscoped by design (gated by platform.profile_verifications.manage).

-- name: GetLatestProfileVerification :one
SELECT *
FROM profile_verification_requests
WHERE profile_id = sqlc.arg(profile_id)
ORDER BY id DESC
LIMIT 1;

-- name: IsProfileVerified :one
SELECT EXISTS (
    SELECT 1 FROM profile_verification_requests
    WHERE profile_id = sqlc.arg(profile_id) AND status = 'approved'
)::boolean AS verified;

-- name: CreateProfileVerificationRequest :one
INSERT INTO profile_verification_requests (account_id, profile_id, note, requested_by_user_id, origin)
VALUES (
    sqlc.arg(account_id),
    sqlc.arg(profile_id),
    sqlc.arg(note),
    sqlc.narg(requested_by_user_id),
    sqlc.arg(origin)
)
RETURNING *;

-- name: CancelPendingProfileVerification :one
UPDATE profile_verification_requests
SET status = 'cancelled'
WHERE profile_id = sqlc.arg(profile_id)
  AND account_id = sqlc.arg(account_id)
  AND status = 'pending'
RETURNING *;

-- name: ReviewProfileVerification :one
-- Approves or rejects a pending request; no row when it is not pending any more.
UPDATE profile_verification_requests
SET status = sqlc.arg(status),
    reviewed_by_user_id = sqlc.narg(reviewed_by_user_id),
    reviewed_at = NOW(),
    review_reason = sqlc.arg(review_reason)
WHERE uuid = sqlc.arg(uuid)
  AND status = 'pending'
RETURNING *;

-- name: RevokeProfileVerificationByUUID :one
UPDATE profile_verification_requests
SET status = 'revoked',
    revoked_by_user_id = sqlc.narg(revoked_by_user_id),
    revoked_at = NOW(),
    revoke_reason = sqlc.arg(revoke_reason)
WHERE uuid = sqlc.arg(uuid)
  AND status = 'approved'
RETURNING *;

-- name: RevokeProfileVerificationForProfile :one
UPDATE profile_verification_requests
SET status = 'revoked',
    revoked_by_user_id = sqlc.narg(revoked_by_user_id),
    revoked_at = NOW(),
    revoke_reason = sqlc.arg(revoke_reason)
WHERE profile_id = sqlc.arg(profile_id)
  AND status = 'approved'
RETURNING *;

-- name: GetProfileVerificationRow :one
SELECT
    v.id, v.uuid, v.status, v.origin, v.note, v.review_reason, v.revoke_reason,
    v.reviewed_at, v.revoked_at, v.created_at,
    p.id AS profile_id, p.uuid AS profile_uuid, p.avatar_url, p.primary_locale,
    a.id AS account_id, a.uuid AS account_uuid, a.name AS account_name, a.type::text AS account_type,
    COALESCE(tr.first_name, '')::text AS first_name,
    COALESCE(tr.last_name, '')::text AS last_name,
    COALESCE(tr.company_name, '')::text AS company_name,
    COALESCE(tr.title, '')::text AS title,
    COALESCE(rt.path, '')::text AS username,
    COALESCE(ru.email, '')::text AS requested_by_email,
    COALESCE(rv.email, '')::text AS reviewed_by_email
FROM profile_verification_requests v
JOIN profiles p ON p.id = v.profile_id
JOIN accounts a ON a.id = v.account_id
LEFT JOIN LATERAL (
    SELECT t.first_name, t.last_name, t.company_name, t.title
    FROM profile_translations t
    WHERE t.profile_id = p.id
    ORDER BY (t.locale = p.primary_locale) DESC, t.is_default DESC, t.locale
    LIMIT 1
) tr ON TRUE
LEFT JOIN LATERAL (
    SELECT r.path FROM routes r
    WHERE r.profile_id = p.id AND r.route_type = 'canonical_profile'
      AND r.status IN ('active', 'locked') AND r.deleted_at IS NULL
    ORDER BY r.id DESC
    LIMIT 1
) rt ON TRUE
LEFT JOIN users ru ON ru.id = v.requested_by_user_id
LEFT JOIN users rv ON rv.id = COALESCE(v.reviewed_by_user_id, v.revoked_by_user_id)
WHERE v.uuid = sqlc.arg(uuid);

-- name: ListProfileVerifications :many
SELECT
    v.id, v.uuid, v.status, v.origin, v.note, v.review_reason, v.revoke_reason,
    v.reviewed_at, v.revoked_at, v.created_at,
    p.id AS profile_id, p.uuid AS profile_uuid, p.avatar_url, p.primary_locale,
    a.id AS account_id, a.uuid AS account_uuid, a.name AS account_name, a.type::text AS account_type,
    COALESCE(tr.first_name, '')::text AS first_name,
    COALESCE(tr.last_name, '')::text AS last_name,
    COALESCE(tr.company_name, '')::text AS company_name,
    COALESCE(tr.title, '')::text AS title,
    COALESCE(rt.path, '')::text AS username,
    COALESCE(ru.email, '')::text AS requested_by_email,
    COALESCE(rv.email, '')::text AS reviewed_by_email
FROM profile_verification_requests v
JOIN profiles p ON p.id = v.profile_id AND p.deleted_at IS NULL
JOIN accounts a ON a.id = v.account_id
LEFT JOIN LATERAL (
    SELECT t.first_name, t.last_name, t.company_name, t.title
    FROM profile_translations t
    WHERE t.profile_id = p.id
    ORDER BY (t.locale = p.primary_locale) DESC, t.is_default DESC, t.locale
    LIMIT 1
) tr ON TRUE
LEFT JOIN LATERAL (
    SELECT r.path FROM routes r
    WHERE r.profile_id = p.id AND r.route_type = 'canonical_profile'
      AND r.status IN ('active', 'locked') AND r.deleted_at IS NULL
    ORDER BY r.id DESC
    LIMIT 1
) rt ON TRUE
LEFT JOIN users ru ON ru.id = v.requested_by_user_id
LEFT JOIN users rv ON rv.id = COALESCE(v.reviewed_by_user_id, v.revoked_by_user_id)
WHERE (sqlc.arg(status)::text = '' OR v.status = sqlc.arg(status)::text)
  AND (
    sqlc.arg(q)::text = ''
    OR a.name ILIKE '%' || sqlc.arg(q)::text || '%'
    OR rt.path ILIKE '%' || sqlc.arg(q)::text || '%'
    OR EXISTS (
        SELECT 1 FROM profile_translations t2
        WHERE t2.profile_id = p.id
          AND (t2.first_name || ' ' || t2.last_name || ' ' || t2.company_name) ILIKE '%' || sqlc.arg(q)::text || '%'
    )
  )
ORDER BY (v.status = 'pending') DESC, v.created_at ASC, v.id ASC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountProfileVerifications :one
SELECT COUNT(*)
FROM profile_verification_requests v
JOIN profiles p ON p.id = v.profile_id AND p.deleted_at IS NULL
JOIN accounts a ON a.id = v.account_id
LEFT JOIN LATERAL (
    SELECT r.path FROM routes r
    WHERE r.profile_id = p.id AND r.route_type = 'canonical_profile'
      AND r.status IN ('active', 'locked') AND r.deleted_at IS NULL
    ORDER BY r.id DESC
    LIMIT 1
) rt ON TRUE
WHERE (sqlc.arg(status)::text = '' OR v.status = sqlc.arg(status)::text)
  AND (
    sqlc.arg(q)::text = ''
    OR a.name ILIKE '%' || sqlc.arg(q)::text || '%'
    OR rt.path ILIKE '%' || sqlc.arg(q)::text || '%'
    OR EXISTS (
        SELECT 1 FROM profile_translations t2
        WHERE t2.profile_id = p.id
          AND (t2.first_name || ' ' || t2.last_name || ' ' || t2.company_name) ILIKE '%' || sqlc.arg(q)::text || '%'
    )
  );

-- name: ListProfileVerificationNotifyUserIDs :many
-- Owners and admins of the profile's account hear about review outcomes.
SELECT m.user_id
FROM account_members m
JOIN users u ON u.id = m.user_id AND u.deleted_at IS NULL
WHERE m.account_id = sqlc.arg(account_id) AND m.role IN ('owner', 'admin')
ORDER BY m.user_id;
