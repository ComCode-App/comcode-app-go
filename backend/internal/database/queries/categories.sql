-- name: ListProductCategories :many
SELECT *
FROM product_categories
WHERE deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR name ILIKE '%' || sqlc.narg(q) || '%'
    OR slug ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(is_active)::boolean IS NULL
    OR is_active = sqlc.narg(is_active)::boolean
  )
ORDER BY display_order ASC, name ASC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountProductCategories :one
SELECT COUNT(*)::bigint
FROM product_categories
WHERE deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR name ILIKE '%' || sqlc.narg(q) || '%'
    OR slug ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(is_active)::boolean IS NULL
    OR is_active = sqlc.narg(is_active)::boolean
  );

-- name: ListActiveProductCategories :many
SELECT *
FROM product_categories
WHERE deleted_at IS NULL
  AND is_active = true
ORDER BY display_order ASC, name ASC;

-- name: GetProductCategoryByUUID :one
SELECT *
FROM product_categories
WHERE uuid = $1
  AND deleted_at IS NULL;

-- name: GetProductCategoryBySlug :one
SELECT *
FROM product_categories
WHERE slug = $1
  AND deleted_at IS NULL;

-- name: CreateProductCategory :one
INSERT INTO product_categories (
    name,
    slug,
    description,
    icon,
    image_url,
    display_order,
    is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: UpdateProductCategory :one
UPDATE product_categories
SET name = COALESCE(sqlc.narg(name), name),
    slug = COALESCE(sqlc.narg(slug), slug),
    description = COALESCE(sqlc.narg(description), description),
    icon = COALESCE(sqlc.narg(icon), icon),
    image_url = COALESCE(sqlc.narg(image_url), image_url),
    display_order = COALESCE(sqlc.narg(display_order), display_order),
    is_active = COALESCE(sqlc.narg(is_active), is_active)
WHERE uuid = sqlc.arg(uuid)
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteProductCategory :exec
UPDATE product_categories
SET deleted_at = NOW()
WHERE uuid = $1;

-- name: CheckProductCategorySlugExists :one
SELECT EXISTS(
    SELECT 1 FROM product_categories
    WHERE slug = sqlc.arg(slug)
      AND deleted_at IS NULL
      AND (sqlc.narg(exclude_uuid)::uuid IS NULL OR uuid != sqlc.narg(exclude_uuid)::uuid)
);
