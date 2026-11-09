-- Public profile pages (/{username}): canonical_profile routes + profile bank account picks.

-- name: GetCanonicalRouteByPath :one
-- Any canonical route (active, locked or retired) whose profile and account are still live.
SELECT r.id, r.uuid, r.path, r.status, r.presentation_config,
       p.id AS profile_id, p.uuid AS profile_uuid, r.account_id::bigint AS account_id
FROM routes r
JOIN profiles p ON p.id = r.profile_id AND p.deleted_at IS NULL
JOIN accounts a ON a.id = r.account_id AND a.status = 'active'
WHERE lower(trim(r.path)) = lower(trim(sqlc.arg(path)::text))
  AND r.route_type = 'canonical_profile'
  AND r.deleted_at IS NULL;

-- name: GetLiveCanonicalRouteForProfile :one
SELECT r.id, r.uuid, r.path, r.status, r.presentation_config
FROM routes r
WHERE r.profile_id = sqlc.arg(profile_id)
  AND r.route_type = 'canonical_profile'
  AND r.status IN ('active', 'locked')
  AND r.deleted_at IS NULL;

-- name: GetRouteByPathAnyType :one
SELECT r.id, r.route_type, r.status, r.profile_id
FROM routes r
WHERE lower(trim(r.path)) = lower(trim(sqlc.arg(path)::text))
  AND r.deleted_at IS NULL;

-- name: CreateCanonicalRoute :one
INSERT INTO routes (path, route_type, status, target_type, profile_id, account_id, owner_user_id, presentation_config)
VALUES (sqlc.arg(path), 'canonical_profile', 'active', 'profile', sqlc.arg(profile_id), sqlc.arg(account_id)::bigint,
        sqlc.narg(owner_user_id), sqlc.arg(presentation_config))
RETURNING id, uuid, path, status, presentation_config;

-- name: SetCanonicalRouteState :exec
UPDATE routes
SET status = sqlc.arg(status), presentation_config = sqlc.arg(presentation_config)
WHERE id = sqlc.arg(id) AND route_type = 'canonical_profile';

-- name: RetireCanonicalRoute :exec
UPDATE routes SET status = 'retired' WHERE id = sqlc.arg(id) AND route_type = 'canonical_profile';

-- name: ReviveCanonicalRoute :exec
UPDATE routes
SET status = sqlc.arg(status), presentation_config = sqlc.arg(presentation_config)
WHERE id = sqlc.arg(id) AND route_type = 'canonical_profile' AND status = 'retired';

-- name: GetProfileIDForAccount :one
SELECT p.id, p.user_id FROM profiles p
WHERE p.uuid = sqlc.arg(uuid) AND p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL;

-- name: ListAccountBankAccountIDsByUUID :many
SELECT b.id, b.uuid FROM user_bank_accounts b
WHERE b.account_id = sqlc.arg(account_id)::bigint
  AND b.uuid = ANY (sqlc.arg(uuids)::uuid[])
  AND b.deleted_at IS NULL;

-- name: DeleteProfileBankAccounts :exec
DELETE FROM profile_bank_accounts WHERE profile_id = sqlc.arg(profile_id);

-- name: InsertProfileBankAccount :exec
INSERT INTO profile_bank_accounts (profile_id, bank_account_id, display_order)
VALUES (sqlc.arg(profile_id), sqlc.arg(bank_account_id), sqlc.arg(display_order));

-- name: ListProfileBankAccountsPublic :many
-- Selected, active bank accounts of a profile with their bank's display data.
SELECT b.uuid, b.account_holder, b.iban, b.currency, b.title,
       COALESCE(bk.name, '') AS bank_name, COALESCE(bk.icon_url, '') AS bank_icon_url,
       COALESCE(bk.brand_color, '') AS bank_brand_color
FROM profile_bank_accounts pba
JOIN user_bank_accounts b ON b.id = pba.bank_account_id AND b.deleted_at IS NULL AND b.is_active = true
LEFT JOIN banks bk ON bk.id = b.bank_id
WHERE pba.profile_id = sqlc.arg(profile_id)
ORDER BY pba.display_order, b.id;
