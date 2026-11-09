-- Usage counters behind plan quotas (internal/platform/entitlement). Keep them cheap: they run
-- before every quota-guarded create.

-- name: CountAccountProfilesUsage :one
SELECT COUNT(*)::bigint
FROM profiles
WHERE account_id = sqlc.arg(account_id)::bigint
  AND deleted_at IS NULL;

-- name: CountAccountSmartLinksUsage :one
SELECT COUNT(*)::bigint
FROM routes
WHERE account_id = sqlc.arg(account_id)::bigint
  AND route_type = 'custom_smart_link'
  AND deleted_at IS NULL;

-- name: CountAccountActiveCardsUsage :one
SELECT COUNT(*)::bigint
FROM nfc_cards
WHERE account_id = sqlc.arg(account_id)::bigint
  AND is_active = true
  AND deleted_at IS NULL;

-- Monthly quotas (TEC-941). period is the first day of the calendar month in Europe/Istanbul,
-- computed by the caller; a missing row means nothing was used yet this month.

-- name: GetMonthlyUsage :one
SELECT COALESCE(u.used, 0)::bigint AS used, COALESCE(u.unlocked, 0)::bigint AS unlocked
FROM (SELECT 1) AS one
LEFT JOIN account_monthly_usage u
    ON u.account_id = sqlc.arg(account_id)::bigint
   AND u.feature_key = sqlc.arg(feature_key)::text
   AND u.period = sqlc.arg(period)::date;

-- name: IncrementMonthlyUsage :one
INSERT INTO account_monthly_usage (account_id, feature_key, period, used)
VALUES (sqlc.arg(account_id)::bigint, sqlc.arg(feature_key)::text, sqlc.arg(period)::date, 1)
ON CONFLICT (account_id, feature_key, period)
DO UPDATE SET used = account_monthly_usage.used + 1, updated_at = NOW()
RETURNING used::bigint;

-- name: DecrementMonthlyUsage :exec
UPDATE account_monthly_usage
SET used = GREATEST(used - 1, 0), updated_at = NOW()
WHERE account_id = sqlc.arg(account_id)::bigint
  AND feature_key = sqlc.arg(feature_key)::text
  AND period = sqlc.arg(period)::date;

-- name: GrantMonthlyUnlock :execrows
-- Takes one unlocked slot of the month while the limit allows it (-1 = no cap). The row exists:
-- IncrementMonthlyUsage runs first. One affected row means the lead stays unlocked.
UPDATE account_monthly_usage
SET unlocked = unlocked + 1, updated_at = NOW()
WHERE account_id = sqlc.arg(account_id)::bigint
  AND feature_key = sqlc.arg(feature_key)::text
  AND period = sqlc.arg(period)::date
  AND (sqlc.arg(limit_value)::int < 0 OR unlocked < sqlc.arg(limit_value)::int);

-- name: AddMonthlyUnlocked :exec
-- Records leads of the month unlocked after a plan upgrade.
INSERT INTO account_monthly_usage (account_id, feature_key, period, unlocked)
VALUES (sqlc.arg(account_id)::bigint, sqlc.arg(feature_key)::text, sqlc.arg(period)::date, sqlc.arg(n)::int)
ON CONFLICT (account_id, feature_key, period)
DO UPDATE SET unlocked = account_monthly_usage.unlocked + sqlc.arg(n)::int, updated_at = NOW();
