-- name: GetSubscriptionByUUID :one
SELECT s.*,
       p.uuid AS plan_uuid,
       p.slug AS plan_slug,
       p.name AS plan_name,
       a.uuid AS account_uuid,
       a.type AS account_type,
       a.name AS account_name,
       u.uuid AS user_uuid,
       u.email AS user_email,
       u.name AS user_name,
       u.surname AS user_surname,
       ca.uuid AS company_uuid,
       ca.name AS company_name,
       ca.slug AS company_slug
FROM subscriptions s
INNER JOIN plans p ON p.id = s.plan_id
INNER JOIN accounts a ON a.id = s.account_id
LEFT JOIN accounts ca ON ca.id = s.account_id AND ca.type = 'company'
LEFT JOIN users u ON u.id = a.personal_user_id
WHERE s.uuid = $1;

-- name: ListSubscriptionsFiltered :many
SELECT s.*,
       p.uuid AS plan_uuid,
       p.slug AS plan_slug,
       p.name AS plan_name,
       a.uuid AS account_uuid,
       a.type AS account_type,
       a.name AS account_name,
       u.uuid AS user_uuid,
       u.email AS user_email,
       u.name AS user_name,
       u.surname AS user_surname,
       ca.uuid AS company_uuid,
       ca.name AS company_name,
       ca.slug AS company_slug
FROM subscriptions s
INNER JOIN plans p ON p.id = s.plan_id
INNER JOIN accounts a ON a.id = s.account_id
LEFT JOIN accounts ca ON ca.id = s.account_id AND ca.type = 'company'
LEFT JOIN users u ON u.id = a.personal_user_id
WHERE (
    sqlc.narg(q)::text IS NULL
    OR p.name ILIKE '%' || sqlc.narg(q) || '%'
    OR p.slug ILIKE '%' || sqlc.narg(q) || '%'
    OR u.email ILIKE '%' || sqlc.narg(q) || '%'
    OR u.name ILIKE '%' || sqlc.narg(q) || '%'
    OR a.name ILIKE '%' || sqlc.narg(q) || '%'
    OR a.slug ILIKE '%' || sqlc.narg(q) || '%'
)
AND (
    sqlc.narg(status)::subscription_status IS NULL
    OR s.status = sqlc.narg(status)::subscription_status
)
ORDER BY s.created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountSubscriptions :one
SELECT COUNT(*)::bigint
FROM subscriptions s
INNER JOIN plans p ON p.id = s.plan_id
INNER JOIN accounts a ON a.id = s.account_id
LEFT JOIN accounts ca ON ca.id = s.account_id AND ca.type = 'company'
LEFT JOIN users u ON u.id = a.personal_user_id
WHERE (
    sqlc.narg(q)::text IS NULL
    OR p.name ILIKE '%' || sqlc.narg(q) || '%'
    OR p.slug ILIKE '%' || sqlc.narg(q) || '%'
    OR u.email ILIKE '%' || sqlc.narg(q) || '%'
    OR u.name ILIKE '%' || sqlc.narg(q) || '%'
    OR a.name ILIKE '%' || sqlc.narg(q) || '%'
    OR a.slug ILIKE '%' || sqlc.narg(q) || '%'
)
AND (
    sqlc.narg(status)::subscription_status IS NULL
    OR s.status = sqlc.narg(status)::subscription_status
);

-- name: CreateSubscription :one
INSERT INTO subscriptions (plan_id, status, account_id, starts_at, ends_at, source)
VALUES ($1, $2, $3, $4, $5, COALESCE(sqlc.narg(source)::subscription_source, 'manual'))
RETURNING *;

-- name: UpdateSubscription :one
UPDATE subscriptions
SET plan_id = COALESCE(sqlc.narg(plan_id), plan_id),
    status = COALESCE(sqlc.narg(status), status),
    starts_at = COALESCE(sqlc.narg(starts_at), starts_at),
    ends_at = sqlc.narg(ends_at)
WHERE uuid = sqlc.arg(uuid)
RETURNING *;

-- name: GetUserIDByUUID :one
SELECT id FROM users WHERE uuid = $1;

-- name: GetPlanIDByUUID :one
SELECT id FROM plans WHERE uuid = $1;

-- name: CancelActiveAccountSubscriptions :exec
UPDATE subscriptions
SET status = 'canceled'
WHERE account_id = $1
  AND status = 'active';
