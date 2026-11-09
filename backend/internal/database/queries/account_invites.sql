-- name: CreateAccountInvite :one
-- TEC-945. Every app query takes the account; the token lookup is the only unscoped read.
INSERT INTO account_invites (account_id, email, role, token_hash, source, invited_by_user_id,
                             profile_id, office_id, group_id, expires_at, last_sent_at, send_count)
VALUES (
    sqlc.arg(account_id), sqlc.arg(email), sqlc.arg(role), sqlc.arg(token_hash), sqlc.arg(source),
    sqlc.narg(invited_by_user_id),
    (SELECT p.id FROM profiles p WHERE p.account_id = sqlc.arg(account_id) AND p.uuid = sqlc.narg(profile_uuid)::uuid AND p.deleted_at IS NULL),
    (SELECT o.id FROM company_offices o WHERE o.account_id = sqlc.arg(account_id) AND o.uuid = sqlc.narg(office_uuid)::uuid),
    (SELECT g.id FROM company_groups g WHERE g.account_id = sqlc.arg(account_id) AND g.uuid = sqlc.narg(group_uuid)::uuid),
    sqlc.arg(expires_at), sqlc.narg(last_sent_at), sqlc.arg(send_count)
)
RETURNING uuid;

-- name: GetAccountInvite :one
SELECT i.id, i.uuid, i.account_id, i.email, i.role, i.status, i.source, i.expires_at, i.last_sent_at,
       i.send_count, i.accepted_at, i.revoked_at, i.created_at, i.updated_at,
       p.uuid AS profile_uuid, o.uuid AS office_uuid, g.uuid AS group_uuid,
       u.uuid AS invited_by_uuid,
       COALESCE(btrim(u.name || ' ' || u.surname), '')::text AS invited_by_name
FROM account_invites i
LEFT JOIN profiles p ON p.id = i.profile_id AND p.deleted_at IS NULL
LEFT JOIN company_offices o ON o.id = i.office_id
LEFT JOIN company_groups g ON g.id = i.group_id
LEFT JOIN users u ON u.id = i.invited_by_user_id
WHERE i.account_id = sqlc.arg(account_id) AND i.uuid = sqlc.arg(uuid);

-- name: GetPendingAccountInviteByEmail :one
SELECT i.id, i.uuid, i.expires_at, p.uuid AS profile_uuid, o.uuid AS office_uuid, g.uuid AS group_uuid
FROM account_invites i
LEFT JOIN profiles p ON p.id = i.profile_id AND p.deleted_at IS NULL
LEFT JOIN company_offices o ON o.id = i.office_id
LEFT JOIN company_groups g ON g.id = i.group_id
WHERE i.account_id = sqlc.arg(account_id) AND i.email = lower(btrim(sqlc.arg(email)::text)) AND i.status = 'pending';

-- name: ListAccountInvites :many
-- status: pending (open, not expired), expired (open, past expires_at), accepted, revoked;
-- NULL lists open invites (pending and expired).
SELECT i.id, i.uuid, i.account_id, i.email, i.role, i.status, i.source, i.expires_at, i.last_sent_at,
       i.send_count, i.accepted_at, i.revoked_at, i.created_at, i.updated_at,
       p.uuid AS profile_uuid, o.uuid AS office_uuid, g.uuid AS group_uuid,
       u.uuid AS invited_by_uuid,
       COALESCE(btrim(u.name || ' ' || u.surname), '')::text AS invited_by_name
FROM account_invites i
LEFT JOIN profiles p ON p.id = i.profile_id AND p.deleted_at IS NULL
LEFT JOIN company_offices o ON o.id = i.office_id
LEFT JOIN company_groups g ON g.id = i.group_id
LEFT JOIN users u ON u.id = i.invited_by_user_id
WHERE i.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(status)::text IS NULL AND i.status = 'pending'
       OR sqlc.narg(status)::text = 'pending' AND i.status = 'pending' AND i.expires_at > sqlc.arg(now)::timestamptz
       OR sqlc.narg(status)::text = 'expired' AND i.status = 'pending' AND i.expires_at <= sqlc.arg(now)::timestamptz
       OR sqlc.narg(status)::text IN ('accepted', 'revoked') AND i.status = sqlc.narg(status)::text)
  AND (sqlc.arg(q)::text = '' OR i.email ILIKE '%' || sqlc.arg(q)::text || '%')
ORDER BY i.created_at DESC, i.id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountAccountInvites :one
SELECT count(*)
FROM account_invites i
WHERE i.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(status)::text IS NULL AND i.status = 'pending'
       OR sqlc.narg(status)::text = 'pending' AND i.status = 'pending' AND i.expires_at > sqlc.arg(now)::timestamptz
       OR sqlc.narg(status)::text = 'expired' AND i.status = 'pending' AND i.expires_at <= sqlc.arg(now)::timestamptz
       OR sqlc.narg(status)::text IN ('accepted', 'revoked') AND i.status = sqlc.narg(status)::text)
  AND (sqlc.arg(q)::text = '' OR i.email ILIKE '%' || sqlc.arg(q)::text || '%');

-- name: RenewAccountInvite :execrows
-- Resend: a new token and a new 7-day window for an open invite.
UPDATE account_invites
SET token_hash = sqlc.arg(token_hash), expires_at = sqlc.arg(expires_at),
    last_sent_at = sqlc.arg(sent_at), send_count = send_count + 1
WHERE account_id = sqlc.arg(account_id) AND id = sqlc.arg(id) AND status = 'pending';

-- name: SetAccountInviteProfile :execrows
UPDATE account_invites ai
SET profile_id = (SELECT p.id FROM profiles p WHERE p.account_id = sqlc.arg(account_id) AND p.uuid = sqlc.narg(profile_uuid)::uuid AND p.deleted_at IS NULL),
    office_id = (SELECT o.id FROM company_offices o WHERE o.account_id = sqlc.arg(account_id) AND o.uuid = sqlc.narg(office_uuid)::uuid),
    group_id = (SELECT g.id FROM company_groups g WHERE g.account_id = sqlc.arg(account_id) AND g.uuid = sqlc.narg(group_uuid)::uuid)
WHERE ai.account_id = sqlc.arg(account_id) AND ai.id = sqlc.arg(id) AND ai.status = 'pending';

-- name: RevokeAccountInvite :execrows
UPDATE account_invites SET status = 'revoked', revoked_at = now()
WHERE account_id = sqlc.arg(account_id) AND id = sqlc.arg(id) AND status = 'pending';

-- name: DeleteAccountInvite :execrows
-- Import rollback removes the invite it created.
DELETE FROM account_invites WHERE account_id = sqlc.arg(account_id) AND id = sqlc.arg(id);

-- name: GetAccountInviteByTokenHash :one
-- The link holder's view: not account-scoped, the token is the credential.
SELECT i.id, i.uuid, i.account_id, i.email, i.role, i.status, i.expires_at,
       i.profile_id, i.office_id, i.group_id,
       o.uuid AS office_uuid, g.uuid AS group_uuid,
       a.uuid AS account_uuid, a.name AS account_name, a.type AS account_type, a.status AS account_status,
       COALESCE(btrim(u.name || ' ' || u.surname), '')::text AS invited_by_name
FROM account_invites i
JOIN accounts a ON a.id = i.account_id
LEFT JOIN company_offices o ON o.id = i.office_id
LEFT JOIN company_groups g ON g.id = i.group_id
LEFT JOIN users u ON u.id = i.invited_by_user_id
WHERE i.token_hash = sqlc.arg(token_hash);

-- name: LockAccountInvite :one
-- Re-reads the invite under lock; a resend in between changed the token, so the old one misses.
SELECT id, status, expires_at FROM account_invites
WHERE id = sqlc.arg(id) AND token_hash = sqlc.arg(token_hash)
FOR UPDATE;

-- name: MarkAccountInviteAccepted :execrows
UPDATE account_invites SET status = 'accepted', accepted_at = now(), accepted_by_user_id = sqlc.arg(user_id)
WHERE id = sqlc.arg(id) AND status = 'pending';

-- name: GetAccountMemberIDByUser :one
SELECT id, role FROM account_members WHERE account_id = sqlc.arg(account_id) AND user_id = sqlc.arg(user_id);

-- name: LinkInvitedMemberProfile :execrows
-- Links the invite's profile to the member unless either side is already linked.
INSERT INTO company_member_profiles (account_id, member_id, profile_id)
SELECT sqlc.arg(account_id), sqlc.arg(member_id), p.id
FROM profiles p
WHERE p.account_id = sqlc.arg(account_id) AND p.id = sqlc.arg(profile_id) AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM company_member_profiles l WHERE l.member_id = sqlc.arg(member_id) OR l.profile_id = p.id)
ON CONFLICT DO NOTHING;

-- name: AssignInvitedMember :execrows
-- Places the new member in the invite's office/group unless already placed.
INSERT INTO company_member_assignments (account_id, member_id, office_id, group_id)
VALUES (sqlc.arg(account_id), sqlc.arg(member_id), sqlc.narg(office_id), sqlc.narg(group_id))
ON CONFLICT (member_id) DO NOTHING;

-- name: GetUserForInvite :one
SELECT id, uuid, email, name, surname, locale FROM users WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
