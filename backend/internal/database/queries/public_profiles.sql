-- Public profile pages (/{username}): canonical_profile routes + profile bank account picks.

-- account-lint: cross-account public profile page resolver
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

-- account-lint: cross-account profile id from a profile already resolved for the account (or the public resolver)
-- name: GetLiveCanonicalRouteForProfile :one
SELECT r.id, r.uuid, r.path, r.status, r.presentation_config
FROM routes r
WHERE r.profile_id = sqlc.arg(profile_id)
  AND r.route_type = 'canonical_profile'
  AND r.status IN ('active', 'locked')
  AND r.deleted_at IS NULL;

-- account-lint: cross-account public path lookup: paths are globally unique
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

-- account-lint: cross-account route id from LiveRoute of the account's own profile
-- name: SetCanonicalRouteState :exec
UPDATE routes
SET status = sqlc.arg(status), presentation_config = sqlc.arg(presentation_config)
WHERE id = sqlc.arg(id) AND route_type = 'canonical_profile';

-- account-lint: cross-account route id from LiveRoute of the account's own profile
-- name: RetireCanonicalRoute :exec
UPDATE routes SET status = 'retired' WHERE id = sqlc.arg(id) AND route_type = 'canonical_profile';

-- account-lint: cross-account route id is a retired route of the same profile (PathOwner check)
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

-- account-lint: cross-account public profile page: bank accounts linked to the profile
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

-- Profile swipe (TEC-1326). A person's profiles are the profiles of their personal account and
-- the company profiles linked to them as a member (company_member_profiles). Only profiles with
-- an active public page in an active account take part.

-- account-lint: cross-account public visitor read: the profiles of the one person behind a resolved public profile (TEC-1326)
-- name: ListPersonProfileGroup :many
WITH person AS (
    SELECT COALESCE(am.user_id, a.personal_user_id) AS user_id
    FROM profiles p
    JOIN accounts a ON a.id = p.account_id
    LEFT JOIN company_member_profiles cmp ON cmp.account_id = p.account_id AND cmp.profile_id = p.id
    LEFT JOIN account_members am ON am.account_id = cmp.account_id AND am.id = cmp.member_id
    WHERE p.id = sqlc.arg(profile_id) AND p.deleted_at IS NULL
),
grp AS (
    SELECT p.id, p.uuid, p.primary_locale, p.is_open, p.is_primary, p.created_at,
           0 AS account_rank, a.created_at AS linked_at, false AS always_open
    FROM person
    JOIN accounts a ON a.personal_user_id = person.user_id AND a.type = 'personal' AND a.status = 'active'
    JOIN profiles p ON p.account_id = a.id AND p.deleted_at IS NULL
    UNION ALL
    SELECT p.id, p.uuid, p.primary_locale, p.is_open, p.is_primary, p.created_at,
           1 AS account_rank, cmp.created_at AS linked_at, COALESCE(ab.profiles_always_open, false) AS always_open
    FROM person
    JOIN account_members am ON am.user_id = person.user_id
    JOIN accounts a ON a.id = am.account_id AND a.type = 'company' AND a.status = 'active'
    JOIN company_member_profiles cmp ON cmp.account_id = am.account_id AND cmp.member_id = am.id
    JOIN profiles p ON p.account_id = cmp.account_id AND p.id = cmp.profile_id AND p.deleted_at IS NULL
    LEFT JOIN account_branding ab ON ab.account_id = a.id
)
SELECT g.id AS profile_id, g.uuid AS profile_uuid, r.path::text AS username,
       (g.is_open OR g.always_open)::boolean AS is_open,
       COALESCE(t.first_name, '')::text AS first_name,
       COALESCE(t.last_name, '')::text AS last_name,
       COALESCE(t.company_name, '')::text AS company_name
FROM grp g
JOIN routes r ON r.profile_id = g.id AND r.route_type = 'canonical_profile'
 AND r.status = 'active' AND r.deleted_at IS NULL
LEFT JOIN LATERAL (
    SELECT pt.first_name, pt.last_name, pt.company_name
    FROM profile_translations pt
    WHERE pt.profile_id = g.id
    ORDER BY (pt.locale = g.primary_locale) DESC, pt.is_default DESC, pt.id
    LIMIT 1
) t ON true
ORDER BY g.account_rank, g.linked_at, g.is_primary DESC, g.created_at DESC, g.id;

-- name: GetProfileOpenState :one
-- The open switch of a profile in the account, whether it is the caller's own employee profile,
-- and whether the company keeps employee profiles always open.
SELECT p.id, p.is_open,
       EXISTS (
           SELECT 1 FROM company_member_profiles cmp
           WHERE cmp.account_id = sqlc.arg(account_id) AND cmp.profile_id = p.id
       )::boolean AS is_member_profile,
       EXISTS (
           SELECT 1 FROM company_member_profiles cmp
           JOIN account_members am ON am.account_id = sqlc.arg(account_id) AND am.id = cmp.member_id
           WHERE cmp.account_id = sqlc.arg(account_id) AND cmp.profile_id = p.id
             AND am.user_id = sqlc.arg(user_id)
       )::boolean AS is_own_member_profile,
       COALESCE((
           SELECT ab.profiles_always_open FROM account_branding ab
           WHERE ab.account_id = sqlc.arg(account_id)
       ), false)::boolean AS always_open
FROM profiles p
WHERE p.account_id = sqlc.arg(account_id) AND p.uuid = sqlc.arg(uuid) AND p.deleted_at IS NULL;

-- name: SetProfileOpen :execrows
UPDATE profiles SET is_open = sqlc.arg(is_open)
WHERE account_id = sqlc.arg(account_id) AND uuid = sqlc.arg(uuid) AND deleted_at IS NULL;

-- name: GetOwnMemberProfileUUID :one
-- The caller's own employee profile in a company account (none: no rows).
SELECT p.uuid
FROM company_member_profiles cmp
JOIN account_members am ON am.account_id = sqlc.arg(account_id) AND am.id = cmp.member_id
JOIN profiles p ON p.account_id = sqlc.arg(account_id) AND p.id = cmp.profile_id AND p.deleted_at IS NULL
WHERE cmp.account_id = sqlc.arg(account_id) AND am.user_id = sqlc.arg(user_id);
