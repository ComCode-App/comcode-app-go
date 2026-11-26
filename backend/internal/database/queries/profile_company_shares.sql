-- Profile shares with companies (TEC-1315). A member shares a personal profile with a company
-- they belong to; the company's owners and admins see it read-only.

-- name: GetShareableProfileID :one
SELECT p.id
FROM profiles p
WHERE p.account_id = sqlc.arg(account_id)
  AND p.uuid = sqlc.arg(profile_uuid)
  AND p.deleted_at IS NULL;

-- account-lint: cross-account the caller's own company memberships, filtered by their user id
-- name: ListProfileShareTargets :many
SELECT a.uuid AS company_uuid, a.name AS company_name, m.role,
       EXISTS (
           SELECT 1 FROM profile_company_shares s
           WHERE s.account_id = sqlc.arg(account_id)
             AND s.profile_id = sqlc.arg(profile_id)
             AND s.company_account_id = a.id
       )::boolean AS shared
FROM account_members m
JOIN accounts a ON a.id = m.account_id
WHERE m.user_id = sqlc.arg(user_id)
  AND a.type = 'company'
  AND a.status = 'active'
ORDER BY a.name, a.id;

-- account-lint: cross-account the caller's own membership in one company, filtered by their user id
-- name: GetCompanyMembershipForUser :one
SELECT m.id AS member_id, a.id AS company_account_id, a.uuid AS company_uuid, a.name AS company_name
FROM account_members m
JOIN accounts a ON a.id = m.account_id
WHERE a.uuid = sqlc.arg(company_uuid)
  AND m.user_id = sqlc.arg(user_id)
  AND a.type = 'company'
  AND a.status = 'active';

-- name: InsertProfileCompanyShare :execrows
INSERT INTO profile_company_shares (account_id, profile_id, company_account_id, member_id, shared_by_user_id)
VALUES (sqlc.arg(account_id), sqlc.arg(profile_id), sqlc.arg(company_account_id), sqlc.arg(member_id), sqlc.arg(shared_by_user_id))
ON CONFLICT (profile_id, company_account_id) DO NOTHING;

-- name: DeleteProfileCompanyShare :execrows
DELETE FROM profile_company_shares
WHERE account_id = sqlc.arg(account_id)
  AND profile_id = sqlc.arg(profile_id)
  AND company_account_id = sqlc.arg(company_account_id);

-- account-lint: cross-account profiles members shared with the active company (read-only, TEC-1315)
-- name: ListCompanySharedProfiles :many
SELECT p.uuid AS profile_uuid,
       p.avatar_url,
       p.completion_rate,
       s.created_at AS shared_at,
       u.uuid AS member_user_uuid,
       u.email AS member_email,
       u.name AS member_name,
       u.surname AS member_surname,
       COALESCE((SELECT pt.first_name FROM profile_translations pt WHERE pt.profile_id = p.id
                 ORDER BY pt.is_default DESC, (pt.locale = p.primary_locale) DESC, pt.id LIMIT 1), '')::text AS first_name,
       COALESCE((SELECT pt.last_name FROM profile_translations pt WHERE pt.profile_id = p.id
                 ORDER BY pt.is_default DESC, (pt.locale = p.primary_locale) DESC, pt.id LIMIT 1), '')::text AS last_name,
       COALESCE((SELECT pt.title FROM profile_translations pt WHERE pt.profile_id = p.id
                 ORDER BY pt.is_default DESC, (pt.locale = p.primary_locale) DESC, pt.id LIMIT 1), '')::text AS title,
       COALESCE((SELECT pt.company_name FROM profile_translations pt WHERE pt.profile_id = p.id
                 ORDER BY pt.is_default DESC, (pt.locale = p.primary_locale) DESC, pt.id LIMIT 1), '')::text AS company_name,
       COALESCE((
           SELECT r.path FROM routes r
           WHERE r.profile_id = p.id AND r.route_type = 'canonical_profile'
             AND r.status IN ('active', 'locked') AND r.deleted_at IS NULL
           LIMIT 1
       ), '')::text AS username
FROM profile_company_shares s
JOIN account_members m ON m.account_id = s.company_account_id AND m.id = s.member_id AND m.user_id = s.shared_by_user_id
JOIN accounts pa ON pa.id = s.account_id AND pa.type = 'personal' AND pa.personal_user_id = s.shared_by_user_id
JOIN profiles p ON p.account_id = s.account_id AND p.id = s.profile_id AND p.deleted_at IS NULL
JOIN users u ON u.id = s.shared_by_user_id
WHERE s.company_account_id = sqlc.arg(company_account_id)
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- account-lint: cross-account counts the profiles members shared with the active company (TEC-1315)
-- name: CountCompanySharedProfiles :one
SELECT COUNT(*)::bigint
FROM profile_company_shares s
JOIN account_members m ON m.account_id = s.company_account_id AND m.id = s.member_id AND m.user_id = s.shared_by_user_id
JOIN accounts pa ON pa.id = s.account_id AND pa.type = 'personal' AND pa.personal_user_id = s.shared_by_user_id
JOIN profiles p ON p.account_id = s.account_id AND p.id = s.profile_id AND p.deleted_at IS NULL
WHERE s.company_account_id = sqlc.arg(company_account_id);
