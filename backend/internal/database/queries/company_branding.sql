-- Company media library and company branding (TEC-946). Every query is scoped by account_id.

-- name: ListCompanyMedia :many
SELECT m.id, m.uuid, m.kind, m.name, m.object_key, m.content_type, m.size_bytes, m.created_at,
       COALESCE(u.uuid::text, '')::text AS uploaded_by_uuid
FROM company_media m
LEFT JOIN users u ON u.id = m.uploaded_by_user_id
WHERE m.account_id = sqlc.arg(account_id)
  AND (sqlc.arg(kind)::text = '' OR m.kind = sqlc.arg(kind)::text)
ORDER BY m.created_at DESC, m.id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountCompanyMedia :one
SELECT COUNT(*)::bigint FROM company_media
WHERE account_id = sqlc.arg(account_id)
  AND (sqlc.arg(kind)::text = '' OR kind = sqlc.arg(kind)::text);

-- name: GetCompanyMedia :one
SELECT m.id, m.uuid, m.kind, m.name, m.object_key, m.content_type, m.size_bytes, m.created_at,
       COALESCE(u.uuid::text, '')::text AS uploaded_by_uuid
FROM company_media m
LEFT JOIN users u ON u.id = m.uploaded_by_user_id
WHERE m.account_id = sqlc.arg(account_id) AND m.uuid = sqlc.arg(uuid);

-- name: CreateCompanyMedia :one
INSERT INTO company_media (uuid, account_id, kind, name, object_key, content_type, size_bytes, uploaded_by_user_id)
VALUES (sqlc.arg(uuid), sqlc.arg(account_id), sqlc.arg(kind), sqlc.arg(name), sqlc.arg(object_key),
        sqlc.arg(content_type), sqlc.arg(size_bytes), sqlc.narg(uploaded_by_user_id))
RETURNING id, uuid, kind, name, object_key, content_type, size_bytes, created_at;

-- name: DeleteCompanyMedia :execrows
DELETE FROM company_media WHERE account_id = sqlc.arg(account_id) AND id = sqlc.arg(id);

-- name: CompanyMediaBrandingUse :many
-- Branding slots ('logo', 'cover') that point at the given library files.
SELECT b.logo_media_id AS media_id, 'logo'::text AS slot FROM account_branding b
WHERE b.account_id = sqlc.arg(account_id) AND b.logo_media_id = ANY(sqlc.arg(media_ids)::bigint[])
UNION ALL
SELECT b.cover_media_id AS media_id, 'cover'::text AS slot FROM account_branding b
WHERE b.account_id = sqlc.arg(account_id) AND b.cover_media_id = ANY(sqlc.arg(media_ids)::bigint[]);

-- name: CompanyMediaProfileUse :many
-- Live profiles of the account whose avatar or cover URL ends with one of the object keys
-- (the URL is the storage public URL or the stream path of the key).
SELECT k.object_key::text AS object_key, p.uuid AS profile_uuid,
       (p.avatar_url <> '' AND right(p.avatar_url, length(k.object_key)) = k.object_key)::boolean AS as_avatar,
       (p.cover_url <> '' AND right(p.cover_url, length(k.object_key)) = k.object_key)::boolean AS as_cover,
       COALESCE((SELECT trim(pt.first_name || ' ' || pt.last_name) FROM profile_translations pt WHERE pt.profile_id = p.id
                 ORDER BY pt.is_default DESC, (pt.locale = p.primary_locale) DESC, pt.id LIMIT 1), '')::text AS display_name
FROM unnest(sqlc.arg(object_keys)::text[]) AS k(object_key)
JOIN profiles p ON p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL
 AND ((p.avatar_url <> '' AND right(p.avatar_url, length(k.object_key)) = k.object_key)
   OR (p.cover_url <> '' AND right(p.cover_url, length(k.object_key)) = k.object_key))
ORDER BY p.id
LIMIT 1000;

-- name: GetAccountBranding :one
SELECT b.theme, b.layout_mode, b.primary_color, b.locked, b.updated_at,
       lm.uuid AS logo_uuid, lm.object_key AS logo_key,
       cm.uuid AS cover_uuid, cm.object_key AS cover_key
FROM account_branding b
LEFT JOIN company_media lm ON lm.account_id = b.account_id AND lm.id = b.logo_media_id
LEFT JOIN company_media cm ON cm.account_id = b.account_id AND cm.id = b.cover_media_id
WHERE b.account_id = sqlc.arg(account_id);

-- name: UpsertAccountBranding :exec
INSERT INTO account_branding (account_id, theme, layout_mode, primary_color, logo_media_id, cover_media_id, locked, updated_by_user_id)
VALUES (sqlc.arg(account_id), sqlc.arg(theme), sqlc.arg(layout_mode), sqlc.arg(primary_color),
        sqlc.narg(logo_media_id), sqlc.narg(cover_media_id), sqlc.arg(locked), sqlc.narg(updated_by_user_id))
ON CONFLICT (account_id) DO UPDATE SET
    theme = EXCLUDED.theme,
    layout_mode = EXCLUDED.layout_mode,
    primary_color = EXCLUDED.primary_color,
    logo_media_id = EXCLUDED.logo_media_id,
    cover_media_id = EXCLUDED.cover_media_id,
    locked = EXCLUDED.locked,
    updated_by_user_id = EXCLUDED.updated_by_user_id;
