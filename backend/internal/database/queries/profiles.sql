-- account-lint: cross-account canonical-route subquery is keyed by this account's profile id (display only)
-- name: ListProfilesByAccountID :many
SELECT 
    p.id,
    p.uuid,
    p.account_id,
    p.primary_locale,
    p.birthday,
    p.gender,
    p.avatar_url,
    p.cover_url,
    p.is_primary,
    p.completion_rate,
    p.created_at,
    p.updated_at,
    COALESCE(pt.first_name, '') AS first_name,
    COALESCE(pt.last_name, '') AS last_name,
    COALESCE(pt.company_name, '') AS company_name,
    COALESCE(pt.title, '') AS title,
    COALESCE((
        SELECT r.path FROM routes r
        WHERE r.profile_id = p.id AND r.route_type = 'canonical_profile'
          AND r.status IN ('active', 'locked') AND r.deleted_at IS NULL
        LIMIT 1
    ), '')::text AS username
FROM profiles p
LEFT JOIN profile_translations pt ON pt.profile_id = p.id AND (pt.locale = p.primary_locale OR pt.is_default = true)
WHERE p.account_id = sqlc.arg(account_id)
  AND p.deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR pt.first_name ILIKE '%' || sqlc.narg(q) || '%'
    OR pt.last_name ILIKE '%' || sqlc.narg(q) || '%'
    OR pt.company_name ILIKE '%' || sqlc.narg(q) || '%'
    OR pt.title ILIKE '%' || sqlc.narg(q) || '%'
  )
ORDER BY p.is_primary DESC, p.created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountProfilesByAccountID :one
SELECT COUNT(DISTINCT p.id)::bigint
FROM profiles p
LEFT JOIN profile_translations pt ON pt.profile_id = p.id AND (pt.locale = p.primary_locale OR pt.is_default = true)
WHERE p.account_id = sqlc.arg(account_id)
  AND p.deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR pt.first_name ILIKE '%' || sqlc.narg(q) || '%'
    OR pt.last_name ILIKE '%' || sqlc.narg(q) || '%'
    OR pt.company_name ILIKE '%' || sqlc.narg(q) || '%'
    OR pt.title ILIKE '%' || sqlc.narg(q) || '%'
  );

-- name: GetProfileByUUIDForAccount :one
SELECT *
FROM profiles
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL;

-- name: GetPrimaryProfileByAccountID :one
SELECT *
FROM profiles
WHERE account_id = $1
  AND is_primary = true
  AND deleted_at IS NULL
LIMIT 1;

-- name: CreateProfile :one
INSERT INTO profiles (
    account_id,
    user_id,
    primary_locale,
    birthday,
    gender,
    avatar_url,
    cover_url,
    is_primary,
    completion_rate
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: UpdateProfile :one
UPDATE profiles
SET primary_locale = COALESCE(sqlc.narg(primary_locale), primary_locale),
    birthday = COALESCE(sqlc.narg(birthday), birthday),
    gender = COALESCE(sqlc.narg(gender), gender),
    avatar_url = COALESCE(sqlc.narg(avatar_url), avatar_url),
    cover_url = COALESCE(sqlc.narg(cover_url), cover_url),
    is_primary = COALESCE(sqlc.narg(is_primary), is_primary),
    completion_rate = COALESCE(sqlc.narg(completion_rate), completion_rate)
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteProfile :exec
UPDATE profiles
SET deleted_at = NOW()
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id);

-- name: ClearPrimaryProfilesForAccount :exec
UPDATE profiles
SET is_primary = false
WHERE account_id = $1
  AND deleted_at IS NULL;

-- name: SetPrimaryProfileByUUID :exec
UPDATE profiles
SET is_primary = true
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL;

-- name: ListProfileTranslations :many
SELECT *
FROM profile_translations
WHERE profile_id = $1
ORDER BY is_default DESC, locale ASC;

-- name: DeleteProfileTranslations :exec
DELETE FROM profile_translations
WHERE profile_id = $1;

-- name: CreateProfileTranslation :one
INSERT INTO profile_translations (
    profile_id,
    locale,
    prefix,
    first_name,
    last_name,
    middle_name,
    suffix,
    nickname,
    company_name,
    department,
    title,
    role,
    bio,
    note,
    is_default
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15
)
RETURNING *;

-- name: ListProfileAddresses :many
SELECT *
FROM profile_addresses
WHERE profile_id = $1
ORDER BY display_order ASC, id ASC;

-- name: DeleteProfileAddresses :exec
DELETE FROM profile_addresses
WHERE profile_id = $1;

-- name: CreateProfileAddress :one
INSERT INTO profile_addresses (
    profile_id,
    type,
    label,
    street,
    extended,
    city,
    region,
    postal_code,
    country,
    is_primary,
    display_order
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: ListProfileContactDetails :many
SELECT *
FROM profile_contact_details
WHERE profile_id = $1
ORDER BY display_order ASC, id ASC;

-- name: DeleteProfileContactDetails :exec
DELETE FROM profile_contact_details
WHERE profile_id = $1;

-- name: CreateProfileContactDetail :one
INSERT INTO profile_contact_details (
    profile_id,
    category,
    type,
    label,
    value,
    is_primary,
    display_order
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: ListProfileSocialLinks :many
SELECT *
FROM profile_social_links
WHERE profile_id = $1
ORDER BY display_order ASC, id ASC;

-- name: DeleteProfileSocialLinks :exec
DELETE FROM profile_social_links
WHERE profile_id = $1;

-- name: CreateProfileSocialLink :one
INSERT INTO profile_social_links (
    profile_id,
    platform_id,
    platform,
    label,
    url,
    icon_url,
    display_order,
    is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- account-lint: cross-account media cleanup worker (TEC-1176): only says whether a profile uuid is still live; returns no row data
-- name: ProfileIsLive :one
SELECT EXISTS (
    SELECT 1 FROM profiles WHERE uuid = sqlc.arg(uuid) AND deleted_at IS NULL
)::boolean AS live;

-- account-lint: cross-account media cleanup worker (TEC-1176): a storage object may be referenced from any account, so the reference check counts rows everywhere and returns only the count
-- name: CountStorageKeyReferences :one
-- Live rows that still point at an object key: profile avatar/cover URLs (bare key, stream path or
-- public URL ending in "/{key}"), company media library files and Background Studio thumbnails.
SELECT (
    (SELECT COUNT(*) FROM profiles p
      WHERE p.deleted_at IS NULL
        AND (p.avatar_url = sqlc.arg(object_key)::text
          OR right(p.avatar_url, length(sqlc.arg(object_key)::text) + 1) = '/' || sqlc.arg(object_key)::text
          OR p.cover_url = sqlc.arg(object_key)::text
          OR right(p.cover_url, length(sqlc.arg(object_key)::text) + 1) = '/' || sqlc.arg(object_key)::text))
  + (SELECT COUNT(*) FROM company_media m WHERE m.object_key = sqlc.arg(object_key)::text)
  + (SELECT COUNT(*) FROM backgrounds b WHERE b.thumbnail_key = sqlc.arg(object_key)::text)
)::bigint AS refs;

-- account-lint: cross-account one-off deleted-profile media backfill (TEC-1177): walks soft-deleted profiles of every account by id cursor and returns only ids, uuids, media columns and locales
-- name: ListDeletedProfilesForMediaBackfill :many
SELECT p.id,
       p.uuid,
       p.avatar_url,
       p.cover_url,
       p.primary_locale,
       COALESCE(
           (SELECT array_agg(DISTINCT t.locale::text) FROM profile_translations t WHERE t.profile_id = p.id),
           '{}'
       )::text[] AS locales
FROM profiles p
WHERE p.deleted_at IS NOT NULL
  AND p.id > sqlc.arg(after_id)::bigint
ORDER BY p.id
LIMIT sqlc.arg(batch_limit)::int;
