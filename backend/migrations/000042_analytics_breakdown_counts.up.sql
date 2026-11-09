-- Analytics breakdowns were merged with jsonb `||`, which overwrites keys: every country, city,
-- device, browser, OS and referer counted at most 1 per (bucket, subject, event type). Sum them
-- instead, and rebuild both aggregate tables from the raw events (kept in `events`).

CREATE OR REPLACE FUNCTION jsonb_sum_counts(a jsonb, b jsonb) RETURNS jsonb
LANGUAGE sql IMMUTABLE AS $$
    SELECT COALESCE(jsonb_object_agg(k, total), '{}'::jsonb)
    FROM (
        SELECT k, SUM(v::numeric)::bigint AS total
        FROM (
            SELECT key AS k, value AS v FROM jsonb_each_text(COALESCE(a, '{}'::jsonb))
            UNION ALL
            SELECT key, value FROM jsonb_each_text(COALESCE(b, '{}'::jsonb))
        ) pairs
        GROUP BY k
    ) sums
$$;

TRUNCATE event_aggregates_daily;

WITH base AS (
    SELECT (occurred_at AT TIME ZONE 'UTC')::date AS b, subject_type AS st, subject_id AS sid, event_type AS et,
           country_code, city, device_type, browser, os, referer, session_id
    FROM events
), g AS (
    SELECT b, st, sid, et, COUNT(*) AS total,
           COUNT(*) FILTER (WHERE COALESCE(session_id, '') <> '') AS sess
    FROM base GROUP BY 1, 2, 3, 4
), dim AS (
    SELECT b, st, sid, et, 'country' AS d, country_code AS k, COUNT(*) AS c FROM base WHERE COALESCE(country_code, '') <> '' GROUP BY 1, 2, 3, 4, 6
    UNION ALL SELECT b, st, sid, et, 'city', city, COUNT(*) FROM base WHERE COALESCE(city, '') <> '' GROUP BY 1, 2, 3, 4, 6
    UNION ALL SELECT b, st, sid, et, 'device', device_type, COUNT(*) FROM base WHERE COALESCE(device_type, '') <> '' GROUP BY 1, 2, 3, 4, 6
    UNION ALL SELECT b, st, sid, et, 'browser', browser, COUNT(*) FROM base WHERE COALESCE(browser, '') <> '' GROUP BY 1, 2, 3, 4, 6
    UNION ALL SELECT b, st, sid, et, 'os', os, COUNT(*) FROM base WHERE COALESCE(os, '') <> '' GROUP BY 1, 2, 3, 4, 6
    UNION ALL SELECT b, st, sid, et, 'referer', referer, COUNT(*) FROM base WHERE COALESCE(referer, '') <> '' GROUP BY 1, 2, 3, 4, 6
), dj AS (
    SELECT b, st, sid, et, d, jsonb_object_agg(k, c) AS j FROM dim GROUP BY 1, 2, 3, 4, 5
)
INSERT INTO event_aggregates_daily (
    bucket_date, subject_type, subject_id, event_type, total_count, unique_sessions,
    country_breakdown, city_breakdown, device_breakdown, browser_breakdown, os_breakdown, referer_breakdown
)
SELECT g.b, g.st, g.sid, g.et, g.total, g.sess,
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'country')), '{}'),
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'city')), '{}'),
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'device')), '{}'),
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'browser')), '{}'),
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'os')), '{}'),
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'referer')), '{}')
FROM g;

TRUNCATE event_aggregates_hourly;

WITH base AS (
    SELECT date_trunc('hour', occurred_at) AS b, subject_type AS st, subject_id AS sid, event_type AS et,
           country_code, city, device_type, browser, os, session_id
    FROM events
), g AS (
    SELECT b, st, sid, et, COUNT(*) AS total,
           COUNT(*) FILTER (WHERE COALESCE(session_id, '') <> '') AS sess
    FROM base GROUP BY 1, 2, 3, 4
), dim AS (
    SELECT b, st, sid, et, 'country' AS d, country_code AS k, COUNT(*) AS c FROM base WHERE COALESCE(country_code, '') <> '' GROUP BY 1, 2, 3, 4, 6
    UNION ALL SELECT b, st, sid, et, 'city', city, COUNT(*) FROM base WHERE COALESCE(city, '') <> '' GROUP BY 1, 2, 3, 4, 6
    UNION ALL SELECT b, st, sid, et, 'device', device_type, COUNT(*) FROM base WHERE COALESCE(device_type, '') <> '' GROUP BY 1, 2, 3, 4, 6
    UNION ALL SELECT b, st, sid, et, 'browser', browser, COUNT(*) FROM base WHERE COALESCE(browser, '') <> '' GROUP BY 1, 2, 3, 4, 6
    UNION ALL SELECT b, st, sid, et, 'os', os, COUNT(*) FROM base WHERE COALESCE(os, '') <> '' GROUP BY 1, 2, 3, 4, 6
), dj AS (
    SELECT b, st, sid, et, d, jsonb_object_agg(k, c) AS j FROM dim GROUP BY 1, 2, 3, 4, 5
)
INSERT INTO event_aggregates_hourly (
    bucket_hour, subject_type, subject_id, event_type, total_count, unique_sessions,
    country_breakdown, city_breakdown, device_breakdown, browser_breakdown, os_breakdown
)
SELECT g.b, g.st, g.sid, g.et, g.total, g.sess,
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'country')), '{}'),
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'city')), '{}'),
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'device')), '{}'),
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'browser')), '{}'),
    COALESCE((SELECT j FROM dj WHERE (dj.b, dj.st, dj.sid, dj.et, dj.d) = (g.b, g.st, g.sid, g.et, 'os')), '{}')
FROM g;
