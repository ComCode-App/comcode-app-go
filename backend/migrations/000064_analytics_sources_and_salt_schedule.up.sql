-- TEC-953: aggregates carry the entry source so dashboards can split QR, NFC, link and legacy
-- ("eski kart") scans without reading raw events. '' means the event is not an entry (page views,
-- clicks, ...). The source is part of the rollup key.
ALTER TABLE event_aggregates_hourly ADD COLUMN source VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE event_aggregates_daily ADD COLUMN source VARCHAR(16) NOT NULL DEFAULT '';

ALTER TABLE event_aggregates_hourly DROP CONSTRAINT uq_event_aggregates_hourly;
ALTER TABLE event_aggregates_hourly ADD CONSTRAINT uq_event_aggregates_hourly
    UNIQUE (bucket_hour, subject_type, subject_id, event_type, source);
ALTER TABLE event_aggregates_daily DROP CONSTRAINT uq_event_aggregates_daily;
ALTER TABLE event_aggregates_daily ADD CONSTRAINT uq_event_aggregates_daily
    UNIQUE (bucket_date, subject_type, subject_id, event_type, source);

-- Backfill: QR scans are always 'qr'; route scans take the source of their route's type. Older
-- route scans did not record a QR marker, so QR-opened card routes stay counted as NFC.
UPDATE event_aggregates_hourly SET source = 'qr' WHERE event_type = 'card.qr.scan';
UPDATE event_aggregates_daily SET source = 'qr' WHERE event_type = 'card.qr.scan';
-- Imported legacy profile page hits (legacymigration) are old-card visits of the profile.
UPDATE event_aggregates_daily SET source = 'legacy' WHERE event_type = 'legacy.profile.view';
UPDATE event_aggregates_hourly a
SET source = CASE r.route_type::text
        WHEN 'card_route' THEN 'nfc'
        WHEN 'legacy_route' THEN 'legacy'
        WHEN 'custom_smart_link' THEN 'link'
    END
FROM routes r
WHERE a.event_type = 'route.scan' AND a.subject_type = 'route' AND a.subject_id = r.uuid::text
  AND r.route_type::text IN ('card_route', 'legacy_route', 'custom_smart_link');
UPDATE event_aggregates_daily a
SET source = CASE r.route_type::text
        WHEN 'card_route' THEN 'nfc'
        WHEN 'legacy_route' THEN 'legacy'
        WHEN 'custom_smart_link' THEN 'link'
    END
FROM routes r
WHERE a.event_type = 'route.scan' AND a.subject_type = 'route' AND a.subject_id = r.uuid::text
  AND r.route_type::text IN ('card_route', 'legacy_route', 'custom_smart_link');

-- TEC-953: platform unique visitors. One row per visitor and UTC hour (the visitor key is a
-- SHA-256 of the session id, else of the rotating IP hash), so distinct counts over any window
-- read this rollup instead of the raw events. Pruned after 35 days by the retention sweep.
CREATE TABLE analytics_visitors_hourly (
    bucket_hour  TIMESTAMPTZ NOT NULL,
    visitor_key  VARCHAR(32) NOT NULL,
    PRIMARY KEY (bucket_hour, visitor_key)
);

-- TEC-936: the IP hash salt rotation period comes from this setting (1-30 days). A change takes
-- effect on the next UTC day; until then the hasher keeps the period that was in force, so hashes
-- of the same day stay comparable. previous/changed_on hold that period and the change day.
UPDATE analytics_settings SET ip_hash_salt_rotation_days = LEAST(GREATEST(ip_hash_salt_rotation_days, 1), 30);
ALTER TABLE analytics_settings
    ADD COLUMN ip_hash_salt_rotation_previous_days INT NULL,
    ADD COLUMN ip_hash_salt_rotation_changed_on DATE NULL,
    ADD CONSTRAINT chk_analytics_ip_hash_salt_rotation_days CHECK (ip_hash_salt_rotation_days BETWEEN 1 AND 30),
    ADD CONSTRAINT chk_analytics_ip_hash_salt_rotation_previous_days
        CHECK (ip_hash_salt_rotation_previous_days IS NULL OR ip_hash_salt_rotation_previous_days BETWEEN 1 AND 30);
