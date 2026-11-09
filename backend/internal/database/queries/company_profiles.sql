-- TEC-573: employee profiles of company accounts (member links, bulk targets, import lookups).

-- name: CompanyActorMembership :one
-- The caller's live membership in a company account. Roles are reloaded per operation, never
-- taken from the token, so a demotion takes effect on the next item.
SELECT m.id AS member_id, m.role, u.uuid AS user_uuid, a.uuid AS account_uuid
FROM account_members m
JOIN accounts a ON a.id = m.account_id AND a.type = 'company'
JOIN users u ON u.id = m.user_id
WHERE m.account_id = sqlc.arg(account_id) AND m.user_id = sqlc.arg(user_id);

-- name: CompanyMemberHasOfficeScope :one
SELECT EXISTS(
    SELECT 1 FROM account_member_scopes
    WHERE account_id = sqlc.arg(account_id) AND member_id = sqlc.arg(member_id)
      AND office_id = sqlc.arg(office_id)
);

-- name: ListCompanyMemberProfiles :many
SELECT u.uuid AS user_uuid, u.email, u.name, u.surname, m.role,
       o.uuid AS office_uuid, g.uuid AS group_uuid, p.uuid AS profile_uuid,
       COALESCE(p.avatar_url, '')::text AS avatar_url,
       COALESCE((SELECT pt.first_name FROM profile_translations pt WHERE pt.profile_id = p.id
                 ORDER BY pt.is_default DESC, (pt.locale = p.primary_locale) DESC, pt.id LIMIT 1), '')::text AS first_name,
       COALESCE((SELECT pt.last_name FROM profile_translations pt WHERE pt.profile_id = p.id
                 ORDER BY pt.is_default DESC, (pt.locale = p.primary_locale) DESC, pt.id LIMIT 1), '')::text AS last_name,
       COALESCE((SELECT pt.title FROM profile_translations pt WHERE pt.profile_id = p.id
                 ORDER BY pt.is_default DESC, (pt.locale = p.primary_locale) DESC, pt.id LIMIT 1), '')::text AS title
FROM account_members m
JOIN users u ON u.id = m.user_id
LEFT JOIN company_member_assignments a ON a.account_id = m.account_id AND a.member_id = m.id
LEFT JOIN company_offices o ON o.account_id = m.account_id AND o.id = a.office_id
LEFT JOIN company_groups g ON g.account_id = m.account_id AND g.id = a.group_id
LEFT JOIN company_member_profiles l ON l.account_id = m.account_id AND l.member_id = m.id
LEFT JOIN profiles p ON p.account_id = m.account_id AND p.id = l.profile_id AND p.deleted_at IS NULL
WHERE m.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(office_uuid)::uuid IS NULL OR o.uuid = sqlc.narg(office_uuid)::uuid)
  AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid)::uuid)
  AND (sqlc.narg(has_profile)::boolean IS NULL OR (p.id IS NOT NULL) = sqlc.narg(has_profile)::boolean)
  AND (sqlc.arg(q)::text = ''
       OR u.email ILIKE '%' || sqlc.arg(q)::text || '%'
       OR (u.name || ' ' || u.surname) ILIKE '%' || sqlc.arg(q)::text || '%')
ORDER BY u.name, u.surname, u.id
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountCompanyMemberProfiles :one
SELECT COUNT(*)::bigint
FROM account_members m
JOIN users u ON u.id = m.user_id
LEFT JOIN company_member_assignments a ON a.account_id = m.account_id AND a.member_id = m.id
LEFT JOIN company_offices o ON o.account_id = m.account_id AND o.id = a.office_id
LEFT JOIN company_groups g ON g.account_id = m.account_id AND g.id = a.group_id
LEFT JOIN company_member_profiles l ON l.account_id = m.account_id AND l.member_id = m.id
LEFT JOIN profiles p ON p.account_id = m.account_id AND p.id = l.profile_id AND p.deleted_at IS NULL
WHERE m.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(office_uuid)::uuid IS NULL OR o.uuid = sqlc.narg(office_uuid)::uuid)
  AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid)::uuid)
  AND (sqlc.narg(has_profile)::boolean IS NULL OR (p.id IS NOT NULL) = sqlc.narg(has_profile)::boolean)
  AND (sqlc.arg(q)::text = ''
       OR u.email ILIKE '%' || sqlc.arg(q)::text || '%'
       OR (u.name || ' ' || u.surname) ILIKE '%' || sqlc.arg(q)::text || '%');

-- name: ListCompanyBulkProfileUUIDs :many
-- Live profiles linked to members matching the filters. A non-nil scope_member_id keeps only
-- members assigned to an office that member (an office manager) was granted.
SELECT p.uuid
FROM company_member_profiles l
JOIN account_members m ON m.account_id = l.account_id AND m.id = l.member_id
JOIN users u ON u.id = m.user_id
JOIN profiles p ON p.account_id = l.account_id AND p.id = l.profile_id AND p.deleted_at IS NULL
LEFT JOIN company_member_assignments a ON a.account_id = l.account_id AND a.member_id = l.member_id
LEFT JOIN company_offices o ON o.account_id = l.account_id AND o.id = a.office_id
LEFT JOIN company_groups g ON g.account_id = l.account_id AND g.id = a.group_id
WHERE l.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(office_uuid)::uuid IS NULL OR o.uuid = sqlc.narg(office_uuid)::uuid)
  AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid)::uuid)
  AND (sqlc.arg(q)::text = ''
       OR u.email ILIKE '%' || sqlc.arg(q)::text || '%'
       OR (u.name || ' ' || u.surname) ILIKE '%' || sqlc.arg(q)::text || '%')
  AND (sqlc.narg(scope_member_id)::bigint IS NULL OR a.office_id IN (
      SELECT s.office_id FROM account_member_scopes s
      WHERE s.account_id = l.account_id AND s.member_id = sqlc.narg(scope_member_id)::bigint))
ORDER BY p.id
LIMIT sqlc.arg(limit_count);

-- name: GetCompanyProfileTarget :one
-- A live profile of the account with the office of the member it is linked to (if any).
SELECT p.id AS profile_id, l.member_id AS member_id, a.office_id AS office_id, o.uuid AS office_uuid
FROM profiles p
LEFT JOIN company_member_profiles l ON l.account_id = p.account_id AND l.profile_id = p.id
LEFT JOIN company_member_assignments a ON a.account_id = p.account_id AND a.member_id = l.member_id
LEFT JOIN company_offices o ON o.account_id = p.account_id AND o.id = a.office_id
WHERE p.account_id = sqlc.arg(account_id) AND p.uuid = sqlc.arg(profile_uuid) AND p.deleted_at IS NULL;

-- name: GetCompanyMemberByUserUUID :one
SELECT m.id AS member_id, m.role, m.user_id
FROM account_members m
JOIN users u ON u.id = m.user_id
WHERE m.account_id = sqlc.arg(account_id) AND u.uuid = sqlc.arg(user_uuid);

-- name: GetCompanyMemberByUserID :one
SELECT m.id AS member_id, m.role
FROM account_members m
WHERE m.account_id = sqlc.arg(account_id) AND m.user_id = sqlc.arg(user_id);

-- name: GetCompanyMemberLink :one
-- The member's linked profile, including a soft-deleted one (callers check deleted).
SELECT p.uuid AS profile_uuid, (p.deleted_at IS NOT NULL)::boolean AS deleted
FROM company_member_profiles l
JOIN profiles p ON p.account_id = l.account_id AND p.id = l.profile_id
WHERE l.account_id = sqlc.arg(account_id) AND l.member_id = sqlc.arg(member_id);

-- name: SetCompanyMemberProfile :execrows
INSERT INTO company_member_profiles (account_id, member_id, profile_id)
SELECT sqlc.arg(account_id), sqlc.arg(member_id), p.id
FROM profiles p
WHERE p.account_id = sqlc.arg(account_id) AND p.uuid = sqlc.arg(profile_uuid) AND p.deleted_at IS NULL
ON CONFLICT (member_id) DO UPDATE SET profile_id = excluded.profile_id;

-- name: DeleteCompanyMemberProfile :execrows
DELETE FROM company_member_profiles
WHERE account_id = sqlc.arg(account_id) AND member_id = sqlc.arg(member_id);

-- name: FindCompanyOfficesByName :many
SELECT uuid FROM company_offices
WHERE account_id = sqlc.arg(account_id) AND lower(btrim(name)) = lower(btrim(sqlc.arg(name)::text))
ORDER BY id
LIMIT 2;

-- name: FindCompanyGroupsByName :many
SELECT g.uuid, o.uuid AS office_uuid
FROM company_groups g
LEFT JOIN company_offices o ON o.account_id = g.account_id AND o.id = g.office_id
WHERE g.account_id = sqlc.arg(account_id) AND lower(btrim(g.name)) = lower(btrim(sqlc.arg(name)::text))
ORDER BY g.id
LIMIT 50;

-- name: FindActiveUserByEmail :one
SELECT id, uuid FROM users
WHERE email = LOWER(TRIM(sqlc.arg(email)::text)) AND deleted_at IS NULL;

-- name: ClearCompanyProfileMedia :execrows
-- Empties avatar and/or cover (the profile upsert keeps media on an empty value); used to roll
-- back a bulk logo onto a profile that had none.
UPDATE profiles
SET avatar_url = CASE WHEN sqlc.arg(clear_avatar)::boolean THEN '' ELSE avatar_url END,
    cover_url = CASE WHEN sqlc.arg(clear_cover)::boolean THEN '' ELSE cover_url END
WHERE account_id = sqlc.arg(account_id) AND uuid = sqlc.arg(profile_uuid) AND deleted_at IS NULL;
