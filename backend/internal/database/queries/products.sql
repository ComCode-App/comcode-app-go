-- name: ListProducts :many
SELECT 
    p.*,
    c.name as category_name,
    c.slug as category_slug
FROM products p
LEFT JOIN product_categories c ON c.id = p.category_id AND c.deleted_at IS NULL
WHERE p.deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR p.name ILIKE '%' || sqlc.narg(q) || '%'
    OR p.slug ILIKE '%' || sqlc.narg(q) || '%'
    OR p.sku ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(category_id)::bigint IS NULL
    OR p.category_id = sqlc.narg(category_id)::bigint
  )
  AND (
    sqlc.narg(status)::text IS NULL
    OR p.status = sqlc.narg(status)::text
  )
  AND (
    sqlc.narg(product_type)::text IS NULL
    OR p.product_type = sqlc.narg(product_type)::text
  )
ORDER BY p.display_order ASC, p.created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountProducts :one
SELECT COUNT(*)::bigint
FROM products p
WHERE p.deleted_at IS NULL
  AND (
    sqlc.narg(q)::text IS NULL
    OR p.name ILIKE '%' || sqlc.narg(q) || '%'
    OR p.slug ILIKE '%' || sqlc.narg(q) || '%'
    OR p.sku ILIKE '%' || sqlc.narg(q) || '%'
  )
  AND (
    sqlc.narg(category_id)::bigint IS NULL
    OR p.category_id = sqlc.narg(category_id)::bigint
  )
  AND (
    sqlc.narg(status)::text IS NULL
    OR p.status = sqlc.narg(status)::text
  )
  AND (
    sqlc.narg(product_type)::text IS NULL
    OR p.product_type = sqlc.narg(product_type)::text
  );

-- name: GetProductByUUID :one
SELECT 
    p.*,
    c.name as category_name,
    c.slug as category_slug
FROM products p
LEFT JOIN product_categories c ON c.id = p.category_id AND c.deleted_at IS NULL
WHERE p.uuid = $1
  AND p.deleted_at IS NULL;

-- name: GetProductBySlug :one
SELECT 
    p.*,
    c.name as category_name,
    c.slug as category_slug
FROM products p
LEFT JOIN product_categories c ON c.id = p.category_id AND c.deleted_at IS NULL
WHERE p.slug = $1
  AND p.deleted_at IS NULL;

-- name: CreateProduct :one
INSERT INTO products (
    category_id,
    name,
    slug,
    short_description,
    description,
    status,
    product_type,
    base_price,
    currency,
    tax_rate,
    stock_quantity,
    sku,
    allow_custom_print,
    model_3d_file_url,
    model_3d_usdz_url,
    model_3d_viewbox,
    images,
    meta_title,
    meta_description,
    meta_keywords,
    canonical_url,
    og_image_url,
    display_order
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
)
RETURNING *;

-- name: UpdateProduct :one
UPDATE products
SET category_id = COALESCE(sqlc.narg(category_id), category_id),
    name = COALESCE(sqlc.narg(name), name),
    slug = COALESCE(sqlc.narg(slug), slug),
    short_description = COALESCE(sqlc.narg(short_description), short_description),
    description = COALESCE(sqlc.narg(description), description),
    status = COALESCE(sqlc.narg(status), status),
    product_type = COALESCE(sqlc.narg(product_type), product_type),
    base_price = COALESCE(sqlc.narg(base_price), base_price),
    currency = COALESCE(sqlc.narg(currency), currency),
    tax_rate = COALESCE(sqlc.narg(tax_rate), tax_rate),
    stock_quantity = COALESCE(sqlc.narg(stock_quantity), stock_quantity),
    sku = COALESCE(sqlc.narg(sku), sku),
    allow_custom_print = COALESCE(sqlc.narg(allow_custom_print), allow_custom_print),
    model_3d_file_url = COALESCE(sqlc.narg(model_3d_file_url), model_3d_file_url),
    model_3d_usdz_url = COALESCE(sqlc.narg(model_3d_usdz_url), model_3d_usdz_url),
    model_3d_viewbox = COALESCE(sqlc.narg(model_3d_viewbox), model_3d_viewbox),
    images = COALESCE(sqlc.narg(images), images),
    meta_title = COALESCE(sqlc.narg(meta_title), meta_title),
    meta_description = COALESCE(sqlc.narg(meta_description), meta_description),
    meta_keywords = COALESCE(sqlc.narg(meta_keywords), meta_keywords),
    canonical_url = COALESCE(sqlc.narg(canonical_url), canonical_url),
    og_image_url = COALESCE(sqlc.narg(og_image_url), og_image_url),
    display_order = COALESCE(sqlc.narg(display_order), display_order)
WHERE uuid = sqlc.arg(uuid)
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateProduct3DViewbox :one
UPDATE products
SET model_3d_viewbox = $2,
    model_3d_file_url = COALESCE(sqlc.narg(model_3d_file_url), model_3d_file_url),
    model_3d_usdz_url = COALESCE(sqlc.narg(model_3d_usdz_url), model_3d_usdz_url)
WHERE uuid = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteProduct :exec
UPDATE products
SET deleted_at = NOW()
WHERE uuid = $1;

-- name: CheckProductSlugExists :one
SELECT EXISTS(
    SELECT 1 FROM products
    WHERE slug = sqlc.arg(slug)
      AND deleted_at IS NULL
      AND (sqlc.narg(exclude_uuid)::uuid IS NULL OR uuid != sqlc.narg(exclude_uuid)::uuid)
);

-- name: ListProductVariantsByProductID :many
SELECT *
FROM product_variants
WHERE product_id = $1
ORDER BY display_order ASC, name ASC;

-- name: GetProductVariantByUUID :one
SELECT *
FROM product_variants
WHERE uuid = $1;

-- name: CreateProductVariant :one
INSERT INTO product_variants (
    product_id,
    name,
    sku,
    price_delta,
    stock_quantity,
    material,
    color_code,
    image_url,
    model_3d_file_url,
    model_3d_usdz_url,
    model_3d_viewbox,
    is_active,
    display_order
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: UpdateProductVariant :one
UPDATE product_variants
SET name = COALESCE(sqlc.narg(name), name),
    sku = COALESCE(sqlc.narg(sku), sku),
    price_delta = COALESCE(sqlc.narg(price_delta), price_delta),
    stock_quantity = COALESCE(sqlc.narg(stock_quantity), stock_quantity),
    material = COALESCE(sqlc.narg(material), material),
    color_code = COALESCE(sqlc.narg(color_code), color_code),
    image_url = COALESCE(sqlc.narg(image_url), image_url),
    model_3d_file_url = COALESCE(sqlc.narg(model_3d_file_url), model_3d_file_url),
    model_3d_usdz_url = COALESCE(sqlc.narg(model_3d_usdz_url), model_3d_usdz_url),
    model_3d_viewbox = COALESCE(sqlc.narg(model_3d_viewbox), model_3d_viewbox),
    is_active = COALESCE(sqlc.narg(is_active), is_active),
    display_order = COALESCE(sqlc.narg(display_order), display_order)
WHERE uuid = sqlc.arg(uuid)
RETURNING *;

-- name: DeleteProductVariant :exec
DELETE FROM product_variants
WHERE uuid = $1;
