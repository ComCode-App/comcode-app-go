-- Platform overview KPIs (TEC-594). Platform-wide by design: no account scope. Callers gate
-- them behind platform.overview.read. Analytics numbers read the aggregate tables only, never
-- the raw events partitions.

-- name: PlatformOverviewUsers :one
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE last_login_at >= sqlc.arg(since)::timestamptz)::bigint AS active,
    COUNT(*) FILTER (WHERE created_at >= sqlc.arg(since)::timestamptz)::bigint AS new_users
FROM users
WHERE deleted_at IS NULL;

-- A profile is published when it has a live (active) username route.
-- name: PlatformOverviewProfiles :one
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE EXISTS (
        SELECT 1 FROM routes r
        WHERE r.profile_id = p.id
          AND r.route_type = 'canonical_profile'
          AND r.status = 'active'
          AND r.deleted_at IS NULL
    ))::bigint AS published
FROM profiles p
WHERE p.deleted_at IS NULL;

-- Provisioned cards start as 'unactivated' and become 'active' when the customer activates them.
-- name: PlatformOverviewCards :one
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE status = 'active')::bigint AS activated
FROM nfc_cards
WHERE deleted_at IS NULL;

-- Orders created since the window start, grouped by their current status.
-- name: PlatformOverviewOrderStatusCounts :many
SELECT status, COUNT(*)::bigint AS count
FROM orders
WHERE deleted_at IS NULL
  AND created_at >= sqlc.arg(since)::timestamptz
GROUP BY status;

-- name: PlatformOverviewSubscriptionCounts :one
SELECT
    COUNT(*) FILTER (WHERE status = 'active')::bigint AS active,
    COUNT(*) FILTER (WHERE status = 'trialing')::bigint AS trialing,
    COUNT(*) FILTER (WHERE status = 'past_due')::bigint AS past_due,
    COUNT(*) FILTER (WHERE status = 'canceled')::bigint AS canceled,
    COUNT(*) FILTER (WHERE status = 'canceled' AND canceled_at >= sqlc.arg(since)::timestamptz)::bigint AS canceled_since
FROM subscriptions;

-- MRR: active subscriptions (online payment and manual alike) on recurring plans, the yearly
-- price spread over 12 months. Free and lifetime plans have no recurring revenue and are left
-- out. past_due subscriptions are not MRR; they are summed separately as at-risk MRR (TEC-944).
-- name: PlatformOverviewMRR :many
SELECT
    p.currency,
    COALESCE(ROUND(SUM(m.monthly_minor) FILTER (WHERE s.status = 'active')), 0)::bigint AS mrr_minor,
    COALESCE(ROUND(SUM(m.monthly_minor) FILTER (WHERE s.status = 'active' AND s.source = 'manual')), 0)::bigint AS manual_mrr_minor,
    COUNT(*) FILTER (WHERE s.status = 'active')::bigint AS subscriptions,
    COALESCE(ROUND(SUM(m.monthly_minor) FILTER (WHERE s.status = 'past_due')), 0)::bigint AS at_risk_mrr_minor,
    COALESCE(ROUND(SUM(m.monthly_minor) FILTER (WHERE s.status = 'past_due' AND s.source = 'manual')), 0)::bigint AS at_risk_manual_mrr_minor,
    COUNT(*) FILTER (WHERE s.status = 'past_due')::bigint AS at_risk_subscriptions
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
CROSS JOIN LATERAL (
    SELECT CASE WHEN p.billing_interval = 'year' THEN p.price_minor::numeric / 12 ELSE p.price_minor::numeric END AS monthly_minor
) m
WHERE s.status IN ('active', 'past_due')
  AND p.billing_interval IN ('month', 'year')
GROUP BY p.currency
ORDER BY p.currency;

-- Conversion funnel (TEC-944) of the orders opened in [since, until), read from
-- order_status_history. A stage counts as reached when the order ever entered that status or a
-- later one of the lifecycle (pending → paid → design → in_production → ready_to_ship → shipped
-- → delivered). Stage times are averaged from the order's creation to the first recorded entry;
-- backfilled rows (from_status NULL, migration 000058) count as reached but carry no timing.
-- Bounded by the period: idx_orders_created_at, then idx_order_status_history_order per order.
-- name: PlatformOverviewConversionFunnel :one
WITH opened AS (
    SELECT id, created_at, status
    FROM orders
    WHERE deleted_at IS NULL
      AND created_at >= sqlc.arg(since)::timestamptz
      AND created_at < sqlc.arg(until)::timestamptz
),
per_order AS (
    SELECT
        o.id,
        o.created_at,
        o.status,
        COALESCE(BOOL_OR(h.to_status IN ('paid', 'design_pending', 'design_approved', 'in_production', 'ready_to_ship', 'shipped', 'delivered')), false) AS paid,
        MIN(h.changed_at) FILTER (WHERE h.from_status IS NOT NULL
            AND h.to_status IN ('paid', 'design_pending', 'design_approved', 'in_production', 'ready_to_ship', 'shipped', 'delivered')) AS paid_at,
        COALESCE(BOOL_OR(h.to_status IN ('in_production', 'ready_to_ship', 'shipped', 'delivered')), false) AS produced,
        MIN(h.changed_at) FILTER (WHERE h.from_status IS NOT NULL
            AND h.to_status IN ('in_production', 'ready_to_ship', 'shipped', 'delivered')) AS produced_at,
        COALESCE(BOOL_OR(h.to_status IN ('shipped', 'delivered')), false) AS shipped,
        MIN(h.changed_at) FILTER (WHERE h.from_status IS NOT NULL AND h.to_status IN ('shipped', 'delivered')) AS shipped_at,
        COALESCE(BOOL_OR(h.to_status = 'delivered'), false) AS delivered,
        MIN(h.changed_at) FILTER (WHERE h.from_status IS NOT NULL AND h.to_status = 'delivered') AS delivered_at
    FROM opened o
    LEFT JOIN order_status_history h ON h.order_id = o.id
    GROUP BY o.id, o.created_at, o.status
)
SELECT
    COUNT(*)::bigint AS created,
    COUNT(*) FILTER (WHERE status IN ('cancelled', 'refunded'))::bigint AS cancelled,
    COUNT(*) FILTER (WHERE paid)::bigint AS paid,
    COUNT(paid_at)::bigint AS paid_timed,
    COALESCE(SUM(EXTRACT(EPOCH FROM paid_at - created_at)), 0)::float8 AS paid_seconds,
    COUNT(*) FILTER (WHERE produced)::bigint AS produced,
    COUNT(produced_at)::bigint AS produced_timed,
    COALESCE(SUM(EXTRACT(EPOCH FROM produced_at - created_at)), 0)::float8 AS produced_seconds,
    COUNT(*) FILTER (WHERE shipped)::bigint AS shipped,
    COUNT(shipped_at)::bigint AS shipped_timed,
    COALESCE(SUM(EXTRACT(EPOCH FROM shipped_at - created_at)), 0)::float8 AS shipped_seconds,
    COUNT(*) FILTER (WHERE delivered)::bigint AS delivered,
    COUNT(delivered_at)::bigint AS delivered_timed,
    COALESCE(SUM(EXTRACT(EPOCH FROM delivered_at - created_at)), 0)::float8 AS delivered_seconds
FROM per_order;

-- Most viewed live profiles from the daily aggregates (public profile page views).
-- name: PlatformOverviewTopProfiles :many
WITH top AS (
    SELECT
        p.id,
        p.uuid,
        SUM(a.total_count)::bigint AS views,
        SUM(a.unique_sessions)::bigint AS unique_sessions
    FROM event_aggregates_daily a
    JOIN profiles p ON p.uuid::text = a.subject_id AND p.deleted_at IS NULL
    WHERE a.subject_type = 'profile'
      AND a.event_type = sqlc.arg(event_type)::text
      AND a.bucket_date >= sqlc.arg(since_date)::date
    GROUP BY p.id, p.uuid
    ORDER BY views DESC, p.id
    LIMIT sqlc.arg(limit_count)::int
)
SELECT
    top.uuid,
    top.views,
    top.unique_sessions,
    COALESCE(pt.first_name, '')::text AS first_name,
    COALESCE(pt.last_name, '')::text AS last_name,
    COALESCE(pt.company_name, '')::text AS company_name,
    COALESCE((
        SELECT r.path FROM routes r
        WHERE r.profile_id = top.id AND r.route_type = 'canonical_profile'
          AND r.status IN ('active', 'locked') AND r.deleted_at IS NULL
        LIMIT 1
    ), '')::text AS username
FROM top
JOIN profiles pr ON pr.id = top.id
LEFT JOIN LATERAL (
    SELECT t.first_name, t.last_name, t.company_name
    FROM profile_translations t
    WHERE t.profile_id = top.id
    ORDER BY (t.locale = pr.primary_locale) DESC, t.is_default DESC, t.id
    LIMIT 1
) pt ON true
ORDER BY top.views DESC, top.id;

-- Event volume per hour bucket since the window start (hourly aggregates).
-- name: PlatformOverviewEventsByHour :many
SELECT bucket_hour, SUM(total_count)::bigint AS count
FROM event_aggregates_hourly
WHERE bucket_hour >= sqlc.arg(since)::timestamptz
GROUP BY bucket_hour
ORDER BY bucket_hour;

-- name: PlatformOverviewEventsByType :many
SELECT event_type::text AS event_type, SUM(total_count)::bigint AS count
FROM event_aggregates_hourly
WHERE bucket_hour >= sqlc.arg(since)::timestamptz
GROUP BY event_type
ORDER BY count DESC, event_type
LIMIT sqlc.arg(limit_count)::int;
