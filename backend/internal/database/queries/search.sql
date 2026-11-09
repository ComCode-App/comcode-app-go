-- Cmd+K search sources (TEC-595). Platform-wide by design: the specs are gated by platform.*
-- permissions and link to platform pages; app users never see them.

-- name: ListOrdersForSearch :many
SELECT o.uuid, o.order_number, o.status, u.email AS customer_email,
       TRIM(u.name || ' ' || u.surname)::text AS customer_name, a.name AS account_name
FROM orders o
JOIN users u ON u.id = o.user_id
JOIN accounts a ON a.id = o.account_id
WHERE o.deleted_at IS NULL
  AND (sqlc.narg(uuid)::uuid IS NULL OR o.uuid = sqlc.narg(uuid)::uuid)
ORDER BY o.id;

-- name: ListProductsForSearch :many
SELECT uuid, name, slug, sku, status
FROM products
WHERE deleted_at IS NULL
  AND (sqlc.narg(uuid)::uuid IS NULL OR uuid = sqlc.narg(uuid)::uuid)
ORDER BY id;

-- name: ListCompaniesForSearch :many
SELECT uuid, name, slug, status
FROM accounts
WHERE type = 'company'
  AND (sqlc.narg(uuid)::uuid IS NULL OR uuid = sqlc.narg(uuid)::uuid)
ORDER BY id;

-- Account-scoped app specs (TEC-951). Each row carries its account_id; the search index stores
-- it as a filterable attribute and app queries always filter on the caller's active account.
-- Activation codes and passcodes are never selected.

-- name: ListProfilesForSearch :many
SELECT p.uuid, p.account_id,
       COALESCE(TRIM(d.first_name || ' ' || d.last_name), '')::text AS display_name,
       COALESCE(d.title, '')::text AS title,
       COALESCE(d.company_name, '')::text AS company_name,
       COALESCE((
           SELECT string_agg(concat_ws(' ', t.first_name, t.last_name, t.nickname, t.company_name,
                                       t.department, t.title, t.role), ' ')
           FROM profile_translations t
           WHERE t.profile_id = p.id
       ), '')::text AS translations,
       COALESCE((
           SELECT r.path FROM routes r
           WHERE r.profile_id = p.id AND r.route_type = 'canonical_profile'
             AND r.status IN ('active', 'locked') AND r.deleted_at IS NULL
           LIMIT 1
       ), '')::text AS username
FROM profiles p
LEFT JOIN LATERAL (
    SELECT t.first_name, t.last_name, t.title, t.company_name
    FROM profile_translations t
    WHERE t.profile_id = p.id
    ORDER BY (t.locale = p.primary_locale) DESC, t.is_default DESC, t.id
    LIMIT 1
) d ON true
WHERE p.deleted_at IS NULL
  AND (sqlc.narg(uuid)::uuid IS NULL OR p.uuid = sqlc.narg(uuid)::uuid)
ORDER BY p.id;

-- name: ListCardsForSearch :many
SELECT c.uuid, c.account_id, c.name, c.smart_code, c.status,
       COALESCE(TRIM(d.first_name || ' ' || d.last_name), '')::text AS profile_name
FROM nfc_cards c
LEFT JOIN profiles p ON p.id = c.profile_id AND p.deleted_at IS NULL
LEFT JOIN LATERAL (
    SELECT t.first_name, t.last_name
    FROM profile_translations t
    WHERE t.profile_id = p.id
    ORDER BY (t.locale = p.primary_locale) DESC, t.is_default DESC, t.id
    LIMIT 1
) d ON true
WHERE c.deleted_at IS NULL
  AND (sqlc.narg(uuid)::uuid IS NULL OR c.uuid = sqlc.narg(uuid)::uuid)
ORDER BY c.id;
