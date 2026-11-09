-- name: GetPersonalAccountMembership :one
SELECT a.id, a.uuid, a.type, a.name, a.slug, a.status, m.role
FROM accounts a
JOIN account_members m ON m.account_id = a.id AND m.user_id = a.personal_user_id
WHERE a.personal_user_id = $1;

-- name: GetAccountMembershipByID :one
SELECT a.id, a.uuid, a.type, a.name, a.slug, a.status, m.role
FROM accounts a
JOIN account_members m ON m.account_id = a.id
WHERE a.id = sqlc.arg(account_id)
  AND m.user_id = sqlc.arg(user_id);

-- name: GetAccountMembershipByUUID :one
SELECT a.id, a.uuid, a.type, a.name, a.slug, a.status, m.role
FROM accounts a
JOIN account_members m ON m.account_id = a.id
WHERE a.uuid = sqlc.arg(account_uuid)
  AND m.user_id = sqlc.arg(user_id);

-- name: ListAccountMembershipsForUser :many
SELECT a.id, a.uuid, a.type, a.name, a.slug, a.status, m.role
FROM accounts a
JOIN account_members m ON m.account_id = a.id
WHERE m.user_id = $1
ORDER BY (a.type = 'personal') DESC, a.name, a.id;

-- name: GetAccountIDByUUID :one
SELECT id FROM accounts WHERE uuid = $1;

-- name: GetPersonalAccountIDByUserUUID :one
SELECT a.id
FROM accounts a
JOIN users u ON u.id = a.personal_user_id
WHERE u.uuid = $1;

-- name: GetAccountByUUID :one
SELECT a.*,
       (SELECT COUNT(*) FROM account_members m WHERE m.account_id = a.id)::int AS member_count
FROM accounts a
WHERE a.uuid = $1;

-- name: GetAccountByID :one
SELECT a.*,
       (SELECT COUNT(*) FROM account_members m WHERE m.account_id = a.id)::int AS member_count
FROM accounts a
WHERE a.id = $1;

-- name: ListAccountsFiltered :many
SELECT a.*,
       (SELECT COUNT(*) FROM account_members m WHERE m.account_id = a.id)::int AS member_count
FROM accounts a
WHERE (sqlc.narg(q)::text IS NULL OR a.name ILIKE '%' || sqlc.narg(q) || '%' OR a.slug ILIKE '%' || sqlc.narg(q) || '%')
  AND (sqlc.narg(type)::account_type IS NULL OR a.type = sqlc.narg(type)::account_type)
  AND (sqlc.narg(status)::account_status IS NULL OR a.status = sqlc.narg(status)::account_status)
ORDER BY a.name, a.id
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountAccountsFiltered :one
SELECT COUNT(*)::bigint
FROM accounts a
WHERE (sqlc.narg(q)::text IS NULL OR a.name ILIKE '%' || sqlc.narg(q) || '%' OR a.slug ILIKE '%' || sqlc.narg(q) || '%')
  AND (sqlc.narg(type)::account_type IS NULL OR a.type = sqlc.narg(type)::account_type)
  AND (sqlc.narg(status)::account_status IS NULL OR a.status = sqlc.narg(status)::account_status);

-- name: CreateCompanyAccount :one
INSERT INTO accounts (type, name, slug)
VALUES ('company', sqlc.arg(name), sqlc.arg(slug))
RETURNING *;

-- name: UpdateAccount :one
UPDATE accounts
SET name = COALESCE(sqlc.narg(name), name),
    status = COALESCE(sqlc.narg(status)::account_status, status)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListAccountMembers :many
SELECT m.role, m.created_at, u.uuid AS user_uuid, u.email, u.name, u.surname
FROM account_members m
JOIN users u ON u.id = m.user_id
WHERE m.account_id = $1
ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, u.name, u.surname;

-- name: GetAccountMemberByUserUUID :one
SELECT m.role, m.user_id, m.created_at, u.uuid AS user_uuid, u.email, u.name, u.surname
FROM account_members m
JOIN users u ON u.id = m.user_id
WHERE m.account_id = sqlc.arg(account_id)
  AND u.uuid = sqlc.arg(user_uuid);

-- name: AddAccountMember :exec
INSERT INTO account_members (account_id, user_id, role)
VALUES (sqlc.arg(account_id), sqlc.arg(user_id), sqlc.arg(role));

-- name: UpdateAccountMemberRole :execrows
UPDATE account_members
SET role = sqlc.arg(role)
WHERE account_id = sqlc.arg(account_id)
  AND user_id = sqlc.arg(user_id);

-- name: RemoveAccountMember :execrows
DELETE FROM account_members
WHERE account_id = sqlc.arg(account_id)
  AND user_id = sqlc.arg(user_id);

-- name: GetActiveUserIDByEmail :one
SELECT id FROM users
WHERE email = LOWER(TRIM(sqlc.arg(email)::text))
  AND deleted_at IS NULL;
