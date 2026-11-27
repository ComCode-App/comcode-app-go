-- ComCode Business pages (TEC-1327). App queries are scoped by account_id; the visitor read
-- resolves a published page by its username route.

-- name: GetBusinessPage :one
SELECT bp.id, bp.uuid, bp.status, bp.theme, bp.primary_locale, bp.translations, bp.published_at,
       bp.created_at, bp.updated_at
FROM business_pages bp
WHERE bp.account_id = sqlc.arg(account_id);

-- name: UpsertBusinessPage :one
INSERT INTO business_pages (account_id, theme, primary_locale, translations, updated_by_user_id)
VALUES (sqlc.arg(account_id), sqlc.arg(theme), sqlc.arg(primary_locale), sqlc.arg(translations),
        sqlc.narg(updated_by_user_id))
ON CONFLICT (account_id) DO UPDATE SET
    theme = EXCLUDED.theme,
    primary_locale = EXCLUDED.primary_locale,
    translations = EXCLUDED.translations,
    updated_by_user_id = EXCLUDED.updated_by_user_id
RETURNING id;

-- name: SetBusinessPageStatus :execrows
UPDATE business_pages
SET status = sqlc.arg(status),
    published_at = CASE WHEN sqlc.arg(status)::text = 'published' THEN now() ELSE published_at END,
    updated_by_user_id = sqlc.narg(updated_by_user_id)
WHERE account_id = sqlc.arg(account_id);

-- name: ListBusinessPageBlocks :many
SELECT b.uuid, b.type, b.position, b.enabled, b.config, b.translations
FROM business_page_blocks b
WHERE b.account_id = sqlc.arg(account_id) AND b.page_id = sqlc.arg(page_id)
ORDER BY b.position, b.id;

-- name: DeleteBusinessPageBlocks :exec
DELETE FROM business_page_blocks WHERE account_id = sqlc.arg(account_id) AND page_id = sqlc.arg(page_id);

-- name: InsertBusinessPageBlock :exec
INSERT INTO business_page_blocks (uuid, account_id, page_id, type, position, enabled, config, translations)
VALUES (sqlc.arg(uuid), sqlc.arg(account_id), sqlc.arg(page_id), sqlc.arg(type), sqlc.arg(position),
        sqlc.arg(enabled), sqlc.arg(config), sqlc.arg(translations));

-- name: GetBusinessPageRoute :one
-- The live (active or locked) username route of the account's business page.
SELECT r.id, r.path, r.status
FROM routes r
WHERE r.account_id = sqlc.arg(account_id)::bigint AND r.route_type = 'business_page'
  AND r.status IN ('active', 'locked') AND r.deleted_at IS NULL;

-- name: CreateBusinessPageRoute :one
INSERT INTO routes (path, route_type, status, target_type, account_id, owner_user_id)
VALUES (sqlc.arg(path), 'business_page', sqlc.arg(status), 'business_page', sqlc.arg(account_id)::bigint,
        sqlc.narg(owner_user_id))
RETURNING id;

-- name: SetBusinessPageRouteStatus :exec
UPDATE routes SET status = sqlc.arg(status)
WHERE id = sqlc.arg(id) AND account_id = sqlc.arg(account_id)::bigint AND route_type = 'business_page'
  AND status IN ('active', 'locked');

-- name: RetireBusinessPageRoute :exec
UPDATE routes SET status = 'retired'
WHERE id = sqlc.arg(id) AND account_id = sqlc.arg(account_id)::bigint AND route_type = 'business_page';

-- name: ReviveBusinessPageRoute :execrows
UPDATE routes SET status = sqlc.arg(status)
WHERE id = sqlc.arg(id) AND account_id = sqlc.arg(account_id)::bigint AND route_type = 'business_page'
  AND status = 'retired';

-- account-lint: cross-account public path lookup: paths are globally unique
-- name: GetBusinessPathOwner :one
SELECT r.id, r.route_type, r.status, r.account_id
FROM routes r
WHERE lower(trim(r.path)) = lower(trim(sqlc.arg(path)::text))
  AND r.deleted_at IS NULL;

-- account-lint: cross-account public visitor read: a business page username resolves to its company account
-- name: GetPublicBusinessRoute :one
-- A business page route by path whose account is a live company. Any status; the use case
-- decides (active = visible, retired = redirect, locked = not found).
SELECT r.id, r.path, r.status, r.account_id::bigint AS account_id, a.uuid AS account_uuid, a.name AS account_name
FROM routes r
JOIN accounts a ON a.id = r.account_id AND a.status = 'active' AND a.type = 'company'
WHERE lower(trim(r.path)) = lower(trim(sqlc.arg(path)::text))
  AND r.route_type = 'business_page'
  AND r.deleted_at IS NULL;

-- name: ListBusinessTeamProfiles :many
-- Published, open employee profiles of the company for the "team" block (TEC-1327).
SELECT p.uuid, r.path::text AS username, p.avatar_url,
       COALESCE(t.first_name, '')::text AS first_name, COALESCE(t.last_name, '')::text AS last_name,
       COALESCE(t.title, '')::text AS title
FROM company_member_profiles cmp
JOIN profiles p ON p.account_id = sqlc.arg(account_id) AND p.id = cmp.profile_id AND p.deleted_at IS NULL AND p.is_open
JOIN routes r ON r.profile_id = p.id AND r.account_id = sqlc.arg(account_id)::bigint AND r.route_type = 'canonical_profile'
 AND r.status = 'active' AND r.deleted_at IS NULL
LEFT JOIN LATERAL (
    SELECT pt.first_name, pt.last_name, pt.title FROM profile_translations pt
    WHERE pt.profile_id = p.id
    ORDER BY (pt.locale = sqlc.arg(locale)::text) DESC, (pt.locale = p.primary_locale) DESC, pt.is_default DESC, pt.id
    LIMIT 1
) t ON true
WHERE cmp.account_id = sqlc.arg(account_id)
ORDER BY t.first_name, t.last_name, p.id
LIMIT 100;

-- name: ListBusinessOffices :many
SELECT o.uuid, o.name, o.city, o.district, o.address, o.phone
FROM company_offices o
WHERE o.account_id = sqlc.arg(account_id)
ORDER BY o.sort_order, o.id;

-- name: ListBusinessBankAccounts :many
-- Active bank accounts of the company among the picked uuids.
SELECT b.uuid, b.account_holder, b.iban, b.currency, b.title,
       COALESCE(bk.name, '') AS bank_name, COALESCE(bk.icon_url, '') AS bank_icon_url,
       COALESCE(bk.brand_color, '') AS bank_brand_color
FROM user_bank_accounts b
LEFT JOIN banks bk ON bk.id = b.bank_id
WHERE b.account_id = sqlc.arg(account_id) AND b.uuid = ANY(sqlc.arg(uuids)::uuid[])
  AND b.deleted_at IS NULL AND b.is_active = true
ORDER BY b.display_order, b.id;

-- name: ListBusinessMedia :many
-- Image files of the company library by uuid (header logo and cover).
SELECT m.uuid, m.object_key
FROM company_media m
WHERE m.account_id = sqlc.arg(account_id) AND m.uuid = ANY(sqlc.arg(uuids)::uuid[]) AND m.kind <> 'document';

-- name: ListSocialPlatformIconsBySlug :many
SELECT sp.slug, sp.icon_url
FROM social_platforms sp
WHERE sp.slug = ANY(sqlc.arg(slugs)::text[]) AND sp.deleted_at IS NULL AND sp.is_active = true;

-- name: SetBusinessPageRouteLeadCapture :exec
-- Mirrors "the page has an enabled lead form button" onto the live route, where the lead module
-- reads presentation_config.lead_capture.enabled (TEC-552).
UPDATE routes
SET presentation_config = jsonb_set(presentation_config, '{lead_capture}', jsonb_build_object('enabled', sqlc.arg(enabled)::boolean), true)
WHERE account_id = sqlc.arg(account_id)::bigint AND route_type = 'business_page'
  AND status IN ('active', 'locked') AND deleted_at IS NULL;
