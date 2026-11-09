-- name: ListBanks :many
SELECT *
FROM banks
WHERE deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR name ILIKE '%' || sqlc.narg(q) || '%'
    OR slug ILIKE '%' || sqlc.narg(q) || '%'
    OR code ILIKE '%' || sqlc.narg(q) || '%'
    OR bic_swift ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(country_code)::text IS NULL
    OR country_code = sqlc.narg(country_code)::text
  )
  AND (
    sqlc.narg(is_active)::boolean IS NULL
    OR is_active = sqlc.narg(is_active)::boolean
  )
ORDER BY display_order ASC, name ASC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountBanks :one
SELECT COUNT(*)::bigint
FROM banks
WHERE deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR name ILIKE '%' || sqlc.narg(q) || '%'
    OR slug ILIKE '%' || sqlc.narg(q) || '%'
    OR code ILIKE '%' || sqlc.narg(q) || '%'
    OR bic_swift ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(country_code)::text IS NULL
    OR country_code = sqlc.narg(country_code)::text
  )
  AND (
    sqlc.narg(is_active)::boolean IS NULL
    OR is_active = sqlc.narg(is_active)::boolean
  );

-- name: ListActiveBanks :many
SELECT *
FROM banks
WHERE deleted_at IS NULL
  AND is_active = true
ORDER BY display_order ASC, name ASC;

-- name: GetBankByUUID :one
SELECT *
FROM banks
WHERE uuid = $1
  AND deleted_at IS NULL;

-- name: GetBankBySlug :one
SELECT *
FROM banks
WHERE slug = $1
  AND deleted_at IS NULL;

-- name: GetBankByCode :one
SELECT *
FROM banks
WHERE code = $1
  AND deleted_at IS NULL
LIMIT 1;

-- name: CreateBank :one
INSERT INTO banks (
    name,
    slug,
    code,
    country_code,
    bic_swift,
    icon_url,
    brand_color,
    iban_pattern,
    description,
    display_order,
    is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: UpdateBank :one
UPDATE banks
SET name = COALESCE(sqlc.narg(name), name),
    slug = COALESCE(sqlc.narg(slug), slug),
    code = COALESCE(sqlc.narg(code), code),
    country_code = COALESCE(sqlc.narg(country_code), country_code),
    bic_swift = COALESCE(sqlc.narg(bic_swift), bic_swift),
    icon_url = COALESCE(sqlc.narg(icon_url), icon_url),
    brand_color = COALESCE(sqlc.narg(brand_color), brand_color),
    iban_pattern = COALESCE(sqlc.narg(iban_pattern), iban_pattern),
    description = COALESCE(sqlc.narg(description), description),
    display_order = COALESCE(sqlc.narg(display_order), display_order),
    is_active = COALESCE(sqlc.narg(is_active), is_active)
WHERE uuid = sqlc.arg(uuid)
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateBankIcon :one
UPDATE banks
SET icon_url = $2
WHERE uuid = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteBank :exec
UPDATE banks
SET deleted_at = NOW()
WHERE uuid = $1;

-- name: CheckBankSlugExists :one
SELECT EXISTS(
    SELECT 1 FROM banks
    WHERE slug = sqlc.arg(slug)
      AND deleted_at IS NULL
      AND (sqlc.narg(exclude_uuid)::uuid IS NULL OR uuid != sqlc.narg(exclude_uuid)::uuid)
);
