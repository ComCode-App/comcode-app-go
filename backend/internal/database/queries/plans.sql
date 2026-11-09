-- name: ListFeatureDefinitions :many
SELECT *
FROM feature_definitions
ORDER BY sort_order ASC, key ASC;

-- name: GetFeatureDefinition :one
SELECT *
FROM feature_definitions
WHERE key = $1;

-- name: GetPlanByUUID :one
SELECT *
FROM plans
WHERE uuid = $1;

-- name: GetPlanBySlug :one
SELECT *
FROM plans
WHERE slug = $1;

-- name: GetPlanByID :one
SELECT *
FROM plans
WHERE id = $1;

-- name: ListPlansFiltered :many
SELECT *
FROM plans
WHERE (
    sqlc.narg(q)::text IS NULL
    OR name ILIKE '%' || sqlc.narg(q) || '%'
    OR slug ILIKE '%' || sqlc.narg(q) || '%'
)
ORDER BY sort_order ASC, name ASC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountPlans :one
SELECT COUNT(*)::bigint
FROM plans
WHERE (
    sqlc.narg(q)::text IS NULL
    OR name ILIKE '%' || sqlc.narg(q) || '%'
    OR slug ILIKE '%' || sqlc.narg(q) || '%'
);

-- name: CreatePlan :one
INSERT INTO plans (slug, name, description, billing_interval, is_active, sort_order, price_minor, currency, trial_days)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdatePlan :one
UPDATE plans
SET name = COALESCE(sqlc.narg(name), name),
    description = COALESCE(sqlc.narg(description), description),
    billing_interval = COALESCE(sqlc.narg(billing_interval), billing_interval),
    is_active = COALESCE(sqlc.narg(is_active), is_active),
    sort_order = COALESCE(sqlc.narg(sort_order), sort_order),
    price_minor = COALESCE(sqlc.narg(price_minor), price_minor),
    currency = COALESCE(sqlc.narg(currency), currency),
    trial_days = COALESCE(sqlc.narg(trial_days), trial_days)
WHERE uuid = sqlc.arg(uuid)
RETURNING *;

-- name: DeletePlan :exec
DELETE FROM plans
WHERE uuid = $1;

-- name: ListPlanLimitsByPlanID :many
SELECT pl.*, fd.value_type, fd.label, fd.description AS feature_description
FROM plan_limits pl
INNER JOIN feature_definitions fd ON fd.key = pl.feature_key
WHERE pl.plan_id = $1
ORDER BY fd.sort_order ASC, fd.key ASC;

-- name: UpsertPlanLimit :exec
INSERT INTO plan_limits (plan_id, feature_key, value_int, value_bool)
VALUES ($1, $2, $3, $4)
ON CONFLICT (plan_id, feature_key) DO UPDATE
SET value_int = EXCLUDED.value_int,
    value_bool = EXCLUDED.value_bool;

-- name: DeletePlanLimit :exec
DELETE FROM plan_limits
WHERE plan_id = $1 AND feature_key = $2;

-- name: DeletePlanLimitsByPlanID :exec
DELETE FROM plan_limits
WHERE plan_id = $1;

-- name: ListPlanLimitsForPlanSlug :many
SELECT pl.feature_key, pl.value_int, pl.value_bool, fd.value_type
FROM plan_limits pl
INNER JOIN plans p ON p.id = pl.plan_id
INNER JOIN feature_definitions fd ON fd.key = pl.feature_key
WHERE p.slug = $1
ORDER BY fd.sort_order ASC;

-- name: ListPlanLimitsForPlanID :many
SELECT pl.feature_key, pl.value_int, pl.value_bool, fd.value_type
FROM plan_limits pl
INNER JOIN feature_definitions fd ON fd.key = pl.feature_key
WHERE pl.plan_id = $1
ORDER BY fd.sort_order ASC;

-- name: GetActiveSubscriptionByAccountID :one
SELECT s.*, p.uuid AS plan_uuid, p.slug AS plan_slug, p.name AS plan_name
FROM subscriptions s
INNER JOIN plans p ON p.id = s.plan_id
WHERE s.account_id = $1
  AND (s.status IN ('active','trialing') OR (s.status='past_due' AND s.grace_until > now()))
  AND (s.ends_at IS NULL OR s.ends_at > now())
  AND (s.grace_until IS NULL OR s.grace_until > now())
  AND (NOT s.cancel_at_period_end OR s.next_billing_at > now())
ORDER BY s.created_at DESC
LIMIT 1;
