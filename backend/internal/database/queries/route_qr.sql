-- name: GetRouteQR :one
SELECT path, qr_style FROM routes
WHERE uuid = $1 AND account_id = $2 AND deleted_at IS NULL;

-- name: PutRouteQR :execrows
UPDATE routes SET qr_style = $3, updated_at = now()
WHERE uuid = $1 AND account_id = $2 AND deleted_at IS NULL;

-- name: GetQRLogo :one
SELECT uuid, avatar_url, cover_url FROM profiles
WHERE uuid = $1 AND account_id = $2 AND deleted_at IS NULL;
