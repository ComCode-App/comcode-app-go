-- name: InsertEvent :one
INSERT INTO events (
    event_type, schema_version,
    subject_type, subject_id,
    actor_type, actor_id,
    session_id, request_id,
    ip_hash,
    country_code, country_name, city, timezone, latitude, longitude,
    device_type, device_brand, device_model,
    browser, browser_version,
    os, os_version,
    referer, user_agent,
    properties, context, metadata,
    occurred_at
) VALUES (
    $1, $2,
    $3, $4,
    $5, $6,
    $7, $8,
    $9,
    $10, $11, $12, $13, $14, $15,
    $16, $17, $18,
    $19, $20,
    $21, $22,
    $23, $24,
    $25, $26, $27,
    $28
)
RETURNING id, uuid, event_type, subject_type, subject_id, occurred_at, created_at;

-- name: GetEventByUUID :one
SELECT * FROM events
WHERE uuid = $1
LIMIT 1;

-- name: ListEventsPaginated :many
SELECT * FROM events
WHERE (sqlc.narg(subject_type)::varchar IS NULL OR subject_type = sqlc.narg(subject_type))
  AND (sqlc.narg(subject_id)::varchar IS NULL OR subject_id = sqlc.narg(subject_id))
  AND (sqlc.narg(event_type)::varchar IS NULL OR event_type = sqlc.narg(event_type))
  AND (sqlc.narg(country_code)::varchar IS NULL OR country_code = sqlc.narg(country_code))
  AND (sqlc.narg(occurred_from)::timestamptz IS NULL OR occurred_at >= sqlc.narg(occurred_from))
  AND (sqlc.narg(occurred_to)::timestamptz IS NULL OR occurred_at <= sqlc.narg(occurred_to))
ORDER BY occurred_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- Bounded count for the raw events list (TEC-859): counts at most cap_count matching rows so a
-- request never scans the whole events table. Callers pass their cap + 1 to detect overflow.
-- name: CountEventsFilteredCapped :one
SELECT COUNT(*)::bigint FROM (
    SELECT 1 FROM events
    WHERE (sqlc.narg(subject_type)::varchar IS NULL OR subject_type = sqlc.narg(subject_type))
      AND (sqlc.narg(subject_id)::varchar IS NULL OR subject_id = sqlc.narg(subject_id))
      AND (sqlc.narg(event_type)::varchar IS NULL OR event_type = sqlc.narg(event_type))
      AND (sqlc.narg(country_code)::varchar IS NULL OR country_code = sqlc.narg(country_code))
      AND (sqlc.narg(occurred_from)::timestamptz IS NULL OR occurred_at >= sqlc.narg(occurred_from))
      AND (sqlc.narg(occurred_to)::timestamptz IS NULL OR occurred_at <= sqlc.narg(occurred_to))
    LIMIT sqlc.arg(cap_count)::int
) capped;

-- Platform-wide event volume per local calendar day (TEC-859), read from the hourly rollups so
-- days follow the viewer's time zone. Never touches the raw events partitions.
-- name: PlatformEventCountsByLocalDay :many
SELECT
    (bucket_hour AT TIME ZONE sqlc.arg(time_zone)::text)::date AS day,
    SUM(total_count)::bigint AS count
FROM event_aggregates_hourly
WHERE bucket_hour >= sqlc.arg(from_at)::timestamptz
  AND bucket_hour < sqlc.arg(to_before)::timestamptz
GROUP BY 1
ORDER BY 1;

-- TEC-953: one row per visitor and hour feeds the platform unique visitor KPI.
-- name: MarkVisitorHour :exec
INSERT INTO analytics_visitors_hourly (bucket_hour, visitor_key)
VALUES (sqlc.arg(bucket_hour)::timestamptz, sqlc.arg(visitor_key)::text)
ON CONFLICT DO NOTHING;

-- Platform-wide distinct visitors (TEC-953) for today, the last 7 and the last 30 local days,
-- from the hourly visitor rollup. Windows end at to_before.
-- name: PlatformUniqueVisitors :one
SELECT
    COUNT(DISTINCT visitor_key) FILTER (WHERE bucket_hour >= sqlc.arg(today_from)::timestamptz)::bigint AS today,
    COUNT(DISTINCT visitor_key) FILTER (WHERE bucket_hour >= sqlc.arg(week_from)::timestamptz)::bigint AS last_7_days,
    COUNT(DISTINCT visitor_key)::bigint AS last_30_days
FROM analytics_visitors_hourly
WHERE bucket_hour >= sqlc.arg(from_at)::timestamptz
  AND bucket_hour < sqlc.arg(to_before)::timestamptz;

-- Most frequent event types over a window of the hourly rollups (TEC-953).
-- name: PlatformTopEventTypes :many
SELECT event_type::text AS event_type, SUM(total_count)::bigint AS count
FROM event_aggregates_hourly
WHERE bucket_hour >= sqlc.arg(from_at)::timestamptz
  AND bucket_hour < sqlc.arg(to_before)::timestamptz
GROUP BY event_type
ORDER BY count DESC, event_type
LIMIT sqlc.arg(limit_count)::int;

-- Owner analytics sources (TEC-953): the subject plus the subjects that lead to it, all from the
-- subject's own account. A profile brings its routes (card, canonical, smart link and legacy hash
-- routes) and cards; a route its card; a card its routes. Counts per event type and source come
-- from the daily rollups only.
-- name: SubjectSourceCounts :many
WITH root_profile AS (
    SELECT p.id, p.account_id FROM profiles p
    WHERE sqlc.arg(subject_type)::text = 'profile' AND p.uuid::text = sqlc.arg(subject_id)::text AND p.deleted_at IS NULL
), root_route AS (
    SELECT r.id, r.card_id, r.account_id FROM routes r
    WHERE sqlc.arg(subject_type)::text = 'route' AND r.uuid::text = sqlc.arg(subject_id)::text AND r.deleted_at IS NULL
), root_card AS (
    SELECT c.id, c.account_id FROM nfc_cards c
    WHERE sqlc.arg(subject_type)::text = 'card' AND c.uuid::text = sqlc.arg(subject_id)::text AND c.deleted_at IS NULL
), root_account AS (
    SELECT account_id FROM root_profile UNION SELECT account_id FROM root_route UNION SELECT account_id FROM root_card
), related AS (
    SELECT 'profile'::text AS subject_type, p.uuid::text AS subject_id
    FROM profiles p WHERE p.id IN (SELECT id FROM root_profile)
    UNION
    SELECT 'route'::text, r.uuid::text
    FROM routes r
    WHERE r.deleted_at IS NULL AND r.account_id IN (SELECT account_id FROM root_account)
      AND (r.profile_id IN (SELECT id FROM root_profile)
        OR r.id IN (SELECT id FROM root_route)
        OR r.card_id IN (SELECT id FROM root_card))
    UNION
    SELECT 'card'::text, c.uuid::text
    FROM nfc_cards c
    WHERE c.deleted_at IS NULL AND c.account_id IN (SELECT account_id FROM root_account)
      AND (c.profile_id IN (SELECT id FROM root_profile)
        OR c.id IN (SELECT card_id FROM root_route)
        OR c.id IN (SELECT id FROM root_card))
)
SELECT related.subject_type::text AS subject_type, d.event_type::text AS event_type, d.source::text AS source,
       SUM(d.total_count)::bigint AS total
FROM related
JOIN event_aggregates_daily d ON d.subject_type = related.subject_type AND d.subject_id = related.subject_id
WHERE d.bucket_date BETWEEN sqlc.arg(from_date)::date AND sqlc.arg(to_date)::date
GROUP BY 1, 2, 3;

-- name: ListEventDefinitions :many
SELECT * FROM event_definitions
WHERE (sqlc.narg(category)::varchar IS NULL OR category = sqlc.narg(category))
  AND (sqlc.narg(enabled)::boolean IS NULL OR enabled = sqlc.narg(enabled))
  AND (sqlc.narg(is_public)::boolean IS NULL OR is_public = sqlc.narg(is_public))
ORDER BY category ASC, key ASC;

-- name: GetEventDefinitionByKey :one
SELECT * FROM event_definitions
WHERE key = $1
LIMIT 1;

-- name: GetEventDefinitionByID :one
SELECT * FROM event_definitions
WHERE id = $1
LIMIT 1;

-- name: CreateEventDefinition :one
INSERT INTO event_definitions (
    key, name, description, category, schema_version,
    enabled, is_public, aggregate_enabled, retention_days, required_properties
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10
)
RETURNING *;

-- name: UpdateEventDefinition :one
UPDATE event_definitions
SET name = COALESCE(sqlc.narg(name), name),
    description = COALESCE(sqlc.narg(description), description),
    category = COALESCE(sqlc.narg(category), category),
    schema_version = COALESCE(sqlc.narg(schema_version), schema_version),
    enabled = COALESCE(sqlc.narg(enabled), enabled),
    is_public = COALESCE(sqlc.narg(is_public), is_public),
    aggregate_enabled = COALESCE(sqlc.narg(aggregate_enabled), aggregate_enabled),
    retention_days = COALESCE(sqlc.narg(retention_days), retention_days),
    required_properties = COALESCE(sqlc.narg(required_properties), required_properties)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteEventDefinition :exec
DELETE FROM event_definitions
WHERE id = $1;

-- name: IsEventProcessed :one
SELECT EXISTS (
    SELECT 1 FROM event_processing_log
    WHERE event_id = $1 AND occurred_at = $2
);

-- name: MarkEventProcessed :exec
INSERT INTO event_processing_log (event_id, occurred_at)
VALUES ($1, $2)
ON CONFLICT (event_id, occurred_at) DO NOTHING;

-- name: UpsertHourlyAggregate :one
INSERT INTO event_aggregates_hourly (
    bucket_hour, subject_type, subject_id, event_type,
    total_count, unique_sessions,
    country_breakdown, city_breakdown, device_breakdown, browser_breakdown, os_breakdown, source
) VALUES (
    $1, $2, $3, $4,
    $5, $6,
    $7, $8, $9, $10, $11, $12
)
ON CONFLICT (bucket_hour, subject_type, subject_id, event_type, source)
DO UPDATE SET
    total_count = event_aggregates_hourly.total_count + EXCLUDED.total_count,
    unique_sessions = event_aggregates_hourly.unique_sessions + EXCLUDED.unique_sessions,
    country_breakdown = jsonb_sum_counts(event_aggregates_hourly.country_breakdown, EXCLUDED.country_breakdown),
    city_breakdown = jsonb_sum_counts(event_aggregates_hourly.city_breakdown, EXCLUDED.city_breakdown),
    device_breakdown = jsonb_sum_counts(event_aggregates_hourly.device_breakdown, EXCLUDED.device_breakdown),
    browser_breakdown = jsonb_sum_counts(event_aggregates_hourly.browser_breakdown, EXCLUDED.browser_breakdown),
    os_breakdown = jsonb_sum_counts(event_aggregates_hourly.os_breakdown, EXCLUDED.os_breakdown),
    updated_at = NOW()
RETURNING *;

-- name: UpsertDailyAggregate :one
INSERT INTO event_aggregates_daily (
    bucket_date, subject_type, subject_id, event_type,
    total_count, unique_sessions,
    country_breakdown, city_breakdown, device_breakdown, browser_breakdown, os_breakdown, referer_breakdown, source
) VALUES (
    $1, $2, $3, $4,
    $5, $6,
    $7, $8, $9, $10, $11, $12, $13
)
ON CONFLICT (bucket_date, subject_type, subject_id, event_type, source)
DO UPDATE SET
    total_count = event_aggregates_daily.total_count + EXCLUDED.total_count,
    unique_sessions = event_aggregates_daily.unique_sessions + EXCLUDED.unique_sessions,
    country_breakdown = jsonb_sum_counts(event_aggregates_daily.country_breakdown, EXCLUDED.country_breakdown),
    city_breakdown = jsonb_sum_counts(event_aggregates_daily.city_breakdown, EXCLUDED.city_breakdown),
    device_breakdown = jsonb_sum_counts(event_aggregates_daily.device_breakdown, EXCLUDED.device_breakdown),
    browser_breakdown = jsonb_sum_counts(event_aggregates_daily.browser_breakdown, EXCLUDED.browser_breakdown),
    os_breakdown = jsonb_sum_counts(event_aggregates_daily.os_breakdown, EXCLUDED.os_breakdown),
    referer_breakdown = jsonb_sum_counts(event_aggregates_daily.referer_breakdown, EXCLUDED.referer_breakdown),
    updated_at = NOW()
RETURNING *;

-- name: GetHourlyAggregates :many
SELECT * FROM event_aggregates_hourly
WHERE subject_type = $1
  AND subject_id = $2
  AND (sqlc.narg(event_type)::varchar IS NULL OR event_type = sqlc.narg(event_type))
  AND bucket_hour >= $3
  AND bucket_hour <= $4
ORDER BY bucket_hour ASC;

-- name: GetDailyAggregates :many
SELECT * FROM event_aggregates_daily
WHERE subject_type = $1
  AND subject_id = $2
  AND (sqlc.narg(event_type)::varchar IS NULL OR event_type = sqlc.narg(event_type))
  AND bucket_date >= $3
  AND bucket_date <= $4
ORDER BY bucket_date ASC;

-- name: GetDailyAggregatesSummary :many
SELECT
    event_type,
    SUM(total_count)::bigint AS total_events,
    SUM(unique_sessions)::bigint AS total_unique_sessions
FROM event_aggregates_daily
WHERE subject_type = $1
  AND subject_id = $2
  AND (sqlc.narg(event_type)::varchar IS NULL OR event_type = sqlc.narg(event_type))
  AND bucket_date >= $3
  AND bucket_date <= $4
GROUP BY event_type;

-- name: GetTopEventTypes :many
SELECT
    event_type,
    SUM(total_count)::bigint AS total_count
FROM event_aggregates_daily
WHERE subject_type = $1
  AND subject_id = $2
  AND bucket_date >= $3
  AND bucket_date <= $4
GROUP BY event_type
ORDER BY total_count DESC
LIMIT sqlc.arg(limit_count);

-- name: GetAnalyticsSettings :one
SELECT * FROM analytics_settings
WHERE id = 1;

-- name: UpdateAnalyticsSettings :one
UPDATE analytics_settings
SET enabled = COALESCE(sqlc.narg(enabled), enabled),
    event_delivery_mode = COALESCE(sqlc.narg(event_delivery_mode), event_delivery_mode),
    raw_event_retention_days = COALESCE(sqlc.narg(raw_event_retention_days), raw_event_retention_days),
    aggregate_retention_days = COALESCE(sqlc.narg(aggregate_retention_days), aggregate_retention_days),
    queue_name = COALESCE(sqlc.narg(queue_name), queue_name),
    realtime_enabled = COALESCE(sqlc.narg(realtime_enabled), realtime_enabled),
    batch_size = COALESCE(sqlc.narg(batch_size), batch_size),
    batch_flush_interval_ms = COALESCE(sqlc.narg(batch_flush_interval_ms), batch_flush_interval_ms),
    -- TEC-936: a new rotation period applies from the next UTC day; previous keeps the period in
    -- force today (unchanged by a second change on the same day).
    ip_hash_salt_rotation_previous_days = CASE
        WHEN sqlc.narg(ip_hash_salt_rotation_days)::int IS NULL
          OR sqlc.narg(ip_hash_salt_rotation_days)::int = ip_hash_salt_rotation_days THEN ip_hash_salt_rotation_previous_days
        WHEN ip_hash_salt_rotation_changed_on = sqlc.arg(today)::date THEN ip_hash_salt_rotation_previous_days
        ELSE ip_hash_salt_rotation_days END,
    ip_hash_salt_rotation_changed_on = CASE
        WHEN sqlc.narg(ip_hash_salt_rotation_days)::int IS NULL
          OR sqlc.narg(ip_hash_salt_rotation_days)::int = ip_hash_salt_rotation_days THEN ip_hash_salt_rotation_changed_on
        ELSE sqlc.arg(today)::date END,
    ip_hash_salt_rotation_days = COALESCE(sqlc.narg(ip_hash_salt_rotation_days)::int, ip_hash_salt_rotation_days),
    anonymize_ip = COALESCE(sqlc.narg(anonymize_ip), anonymize_ip)
WHERE id = 1
RETURNING *;

-- name: GetGeoIPSettings :one
SELECT * FROM geoip_settings
WHERE id = 1;

-- name: UpdateGeoIPSettings :one
UPDATE geoip_settings
SET enabled = COALESCE(sqlc.narg(enabled), enabled),
    provider = COALESCE(sqlc.narg(provider), provider),
    database_type = COALESCE(sqlc.narg(database_type), database_type),
    database_path = COALESCE(sqlc.narg(database_path), database_path),
    auto_update_enabled = COALESCE(sqlc.narg(auto_update_enabled), auto_update_enabled),
    update_interval_hours = COALESCE(sqlc.narg(update_interval_hours), update_interval_hours),
    account_id_encrypted = COALESCE(sqlc.narg(account_id_encrypted), account_id_encrypted),
    license_key_encrypted = COALESCE(sqlc.narg(license_key_encrypted), license_key_encrypted),
    download_url_template = COALESCE(sqlc.narg(download_url_template), download_url_template),
    cache_enabled = COALESCE(sqlc.narg(cache_enabled), cache_enabled),
    cache_ttl_hours = COALESCE(sqlc.narg(cache_ttl_hours), cache_ttl_hours),
    cache_max_entries = COALESCE(sqlc.narg(cache_max_entries), cache_max_entries)
WHERE id = 1
RETURNING *;

-- name: UpdateGeoIPStatus :one
UPDATE geoip_settings
SET last_update_at = $1,
    last_update_status = $2,
    last_update_error = $3,
    database_version = $4
WHERE id = 1
RETURNING *;

-- Unique visitors are counted from raw events: the aggregate tables add 1 per event, so their
-- unique_sessions column is an event count. A visitor is the session id, else the daily IP hash.

-- name: CountDistinctVisitors :one
SELECT COUNT(DISTINCT COALESCE(NULLIF(session_id, ''), ip_hash))::bigint
FROM events
WHERE subject_type = sqlc.arg(subject_type)
  AND subject_id = sqlc.arg(subject_id)
  AND occurred_at >= sqlc.arg(from_at)
  AND occurred_at < sqlc.arg(to_before)
  AND (sqlc.narg(event_type)::varchar IS NULL OR event_type = sqlc.narg(event_type));

-- name: DistinctVisitorsPerDay :many
SELECT (date_trunc('day', occurred_at AT TIME ZONE 'UTC'))::date AS day,
       COUNT(DISTINCT COALESCE(NULLIF(session_id, ''), ip_hash))::bigint AS visitors
FROM events
WHERE subject_type = sqlc.arg(subject_type)
  AND subject_id = sqlc.arg(subject_id)
  AND occurred_at >= sqlc.arg(from_at)
  AND occurred_at < sqlc.arg(to_before)
  AND (sqlc.narg(event_type)::varchar IS NULL OR event_type = sqlc.narg(event_type))
GROUP BY 1;

-- name: DistinctVisitorsPerHour :many
SELECT date_trunc('hour', occurred_at)::timestamptz AS hour,
       COUNT(DISTINCT COALESCE(NULLIF(session_id, ''), ip_hash))::bigint AS visitors
FROM events
WHERE subject_type = sqlc.arg(subject_type)
  AND subject_id = sqlc.arg(subject_id)
  AND occurred_at >= sqlc.arg(from_at)
  AND occurred_at <= sqlc.arg(to_at)
  AND (sqlc.narg(event_type)::varchar IS NULL OR event_type = sqlc.narg(event_type))
GROUP BY 1;
