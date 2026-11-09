ALTER TABLE analytics_settings
    DROP CONSTRAINT IF EXISTS chk_analytics_ip_hash_salt_rotation_previous_days,
    DROP CONSTRAINT IF EXISTS chk_analytics_ip_hash_salt_rotation_days,
    DROP COLUMN IF EXISTS ip_hash_salt_rotation_changed_on,
    DROP COLUMN IF EXISTS ip_hash_salt_rotation_previous_days;

DROP TABLE IF EXISTS analytics_visitors_hourly;

-- Merge the per-source rows back into one row per (bucket, subject, event type).
CREATE AGGREGATE jsonb_sum_counts_agg(jsonb) (SFUNC = jsonb_sum_counts, STYPE = jsonb, INITCOND = '{}');

CREATE TEMP TABLE merged_hourly AS
SELECT bucket_hour, subject_type, subject_id, event_type,
       SUM(total_count)::bigint AS total_count, SUM(unique_sessions)::bigint AS unique_sessions,
       jsonb_sum_counts_agg(country_breakdown) AS country_breakdown,
       jsonb_sum_counts_agg(city_breakdown) AS city_breakdown,
       jsonb_sum_counts_agg(device_breakdown) AS device_breakdown,
       jsonb_sum_counts_agg(browser_breakdown) AS browser_breakdown,
       jsonb_sum_counts_agg(os_breakdown) AS os_breakdown
FROM event_aggregates_hourly GROUP BY 1, 2, 3, 4;
DELETE FROM event_aggregates_hourly;
ALTER TABLE event_aggregates_hourly DROP CONSTRAINT uq_event_aggregates_hourly;
ALTER TABLE event_aggregates_hourly DROP COLUMN source;
INSERT INTO event_aggregates_hourly (bucket_hour, subject_type, subject_id, event_type, total_count, unique_sessions,
    country_breakdown, city_breakdown, device_breakdown, browser_breakdown, os_breakdown)
SELECT * FROM merged_hourly;
ALTER TABLE event_aggregates_hourly ADD CONSTRAINT uq_event_aggregates_hourly
    UNIQUE (bucket_hour, subject_type, subject_id, event_type);

CREATE TEMP TABLE merged_daily AS
SELECT bucket_date, subject_type, subject_id, event_type,
       SUM(total_count)::bigint AS total_count, SUM(unique_sessions)::bigint AS unique_sessions,
       jsonb_sum_counts_agg(country_breakdown) AS country_breakdown,
       jsonb_sum_counts_agg(city_breakdown) AS city_breakdown,
       jsonb_sum_counts_agg(device_breakdown) AS device_breakdown,
       jsonb_sum_counts_agg(browser_breakdown) AS browser_breakdown,
       jsonb_sum_counts_agg(os_breakdown) AS os_breakdown,
       jsonb_sum_counts_agg(referer_breakdown) AS referer_breakdown
FROM event_aggregates_daily GROUP BY 1, 2, 3, 4;
DELETE FROM event_aggregates_daily;
ALTER TABLE event_aggregates_daily DROP CONSTRAINT uq_event_aggregates_daily;
ALTER TABLE event_aggregates_daily DROP COLUMN source;
INSERT INTO event_aggregates_daily (bucket_date, subject_type, subject_id, event_type, total_count, unique_sessions,
    country_breakdown, city_breakdown, device_breakdown, browser_breakdown, os_breakdown, referer_breakdown)
SELECT * FROM merged_daily;
ALTER TABLE event_aggregates_daily ADD CONSTRAINT uq_event_aggregates_daily
    UNIQUE (bucket_date, subject_type, subject_id, event_type);

DROP TABLE merged_hourly;
DROP TABLE merged_daily;
DROP AGGREGATE jsonb_sum_counts_agg(jsonb);
