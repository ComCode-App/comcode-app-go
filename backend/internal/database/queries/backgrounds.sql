-- Background Studio designs (account-owned) and platform presets (account_id IS NULL).

-- name: ListBackgroundsForAccount :many
-- Account's own designs first, then platform presets.
SELECT b.uuid, b.account_id, b.name, b.kind, b.document, b.thumbnail_key, b.created_at, b.updated_at
FROM backgrounds b
WHERE b.account_id = sqlc.arg(account_id)::bigint OR b.account_id IS NULL
ORDER BY (b.account_id IS NULL), b.created_at DESC
LIMIT 200;

-- name: ListBackgroundPresets :many
SELECT b.uuid, b.account_id, b.name, b.kind, b.document, b.thumbnail_key, b.created_at, b.updated_at
FROM backgrounds b
WHERE b.account_id IS NULL
ORDER BY b.created_at DESC
LIMIT 200;

-- name: CountAccountBackgrounds :one
SELECT COUNT(*)::bigint FROM backgrounds WHERE account_id = sqlc.arg(account_id)::bigint;

-- name: GetBackgroundVisibleToAccount :one
-- A background an account may use: its own or a preset.
SELECT b.uuid, b.account_id, b.name, b.kind, b.document, b.thumbnail_key, b.created_at, b.updated_at
FROM backgrounds b
WHERE b.uuid = sqlc.arg(uuid)
  AND (b.account_id = sqlc.arg(account_id)::bigint OR b.account_id IS NULL);

-- name: CreateBackground :one
INSERT INTO backgrounds (account_id, name, kind, document, created_by)
VALUES (sqlc.narg(account_id), sqlc.arg(name), sqlc.arg(kind), sqlc.arg(document), sqlc.narg(created_by))
RETURNING uuid, account_id, name, kind, document, thumbnail_key, created_at, updated_at;

-- name: UpdateAccountBackground :one
UPDATE backgrounds
SET name = sqlc.arg(name), kind = sqlc.arg(kind), document = sqlc.arg(document)
WHERE uuid = sqlc.arg(uuid) AND account_id = sqlc.arg(account_id)::bigint
RETURNING uuid, account_id, name, kind, document, thumbnail_key, created_at, updated_at;

-- name: UpdateBackgroundPreset :one
UPDATE backgrounds
SET name = sqlc.arg(name), kind = sqlc.arg(kind), document = sqlc.arg(document)
WHERE uuid = sqlc.arg(uuid) AND account_id IS NULL
RETURNING uuid, account_id, name, kind, document, thumbnail_key, created_at, updated_at;

-- name: DeleteAccountBackground :execrows
DELETE FROM backgrounds WHERE uuid = sqlc.arg(uuid) AND account_id = sqlc.arg(account_id)::bigint;

-- name: DeleteBackgroundPreset :execrows
DELETE FROM backgrounds WHERE uuid = sqlc.arg(uuid) AND account_id IS NULL;

-- name: GetPublicBackgroundForAccount :one
-- Background shown on a public page: the owner account's or a preset.
SELECT b.document, b.thumbnail_key
FROM backgrounds b
WHERE b.uuid = sqlc.arg(uuid)
  AND (b.account_id = sqlc.arg(account_id)::bigint OR b.account_id IS NULL);

-- name: SetBackgroundThumbnail :execrows
-- Stores the key of a freshly rendered thumbnail (TEC-599). The document must still be the one
-- rendered, so a thumbnail never lands on a newer edit.
UPDATE backgrounds
SET thumbnail_key = sqlc.arg(thumbnail_key)
WHERE uuid = sqlc.arg(uuid) AND document = sqlc.arg(document)::jsonb;

-- name: ListBackgroundsWithoutThumbnail :many
-- Designs saved before thumbnails existed (videos have none by design).
SELECT uuid, document
FROM backgrounds
WHERE thumbnail_key IS NULL AND kind <> 'video'
ORDER BY id
LIMIT sqlc.arg(max_rows)::int;

-- name: GetBackgroundThumbnailKey :one
SELECT thumbnail_key FROM backgrounds WHERE uuid = sqlc.arg(uuid);
