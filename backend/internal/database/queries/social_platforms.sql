-- name: ListSocialPlatforms :many
SELECT *
FROM social_platforms
WHERE deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR name ILIKE '%' || sqlc.narg(q) || '%'
    OR slug ILIKE '%' || sqlc.narg(q) || '%'
    OR category ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(category)::text IS NULL
    OR category = sqlc.narg(category)::text
  )
  AND (
    sqlc.narg(is_active)::boolean IS NULL
    OR is_active = sqlc.narg(is_active)::boolean
  )
ORDER BY display_order ASC, name ASC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountSocialPlatforms :one
SELECT COUNT(*)::bigint
FROM social_platforms
WHERE deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR name ILIKE '%' || sqlc.narg(q) || '%'
    OR slug ILIKE '%' || sqlc.narg(q) || '%'
    OR category ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(category)::text IS NULL
    OR category = sqlc.narg(category)::text
  )
  AND (
    sqlc.narg(is_active)::boolean IS NULL
    OR is_active = sqlc.narg(is_active)::boolean
  );

-- name: ListActiveSocialPlatforms :many
SELECT *
FROM social_platforms
WHERE deleted_at IS NULL
  AND is_active = true
ORDER BY display_order ASC, name ASC;

-- name: GetSocialPlatformByUUID :one
SELECT *
FROM social_platforms
WHERE uuid = $1
  AND deleted_at IS NULL;

-- name: GetSocialPlatformBySlug :one
SELECT *
FROM social_platforms
WHERE slug = $1
  AND deleted_at IS NULL;

-- name: CreateSocialPlatform :one
INSERT INTO social_platforms (
    name,
    slug,
    category,
    url_pattern,
    icon_url,
    placeholder,
    description,
    display_order,
    is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: UpdateSocialPlatform :one
UPDATE social_platforms
SET name = COALESCE(sqlc.narg(name), name),
    slug = COALESCE(sqlc.narg(slug), slug),
    category = COALESCE(sqlc.narg(category), category),
    url_pattern = COALESCE(sqlc.narg(url_pattern), url_pattern),
    icon_url = COALESCE(sqlc.narg(icon_url), icon_url),
    placeholder = COALESCE(sqlc.narg(placeholder), placeholder),
    description = COALESCE(sqlc.narg(description), description),
    display_order = COALESCE(sqlc.narg(display_order), display_order),
    is_active = COALESCE(sqlc.narg(is_active), is_active)
WHERE uuid = sqlc.arg(uuid)
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateSocialPlatformIcon :one
UPDATE social_platforms
SET icon_url = $2
WHERE uuid = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteSocialPlatform :exec
UPDATE social_platforms
SET deleted_at = NOW()
WHERE uuid = $1;

-- name: CheckSocialPlatformSlugExists :one
SELECT EXISTS(
    SELECT 1 FROM social_platforms
    WHERE slug = sqlc.arg(slug)
      AND deleted_at IS NULL
      AND (sqlc.narg(exclude_uuid)::uuid IS NULL OR uuid != sqlc.narg(exclude_uuid)::uuid)
);
