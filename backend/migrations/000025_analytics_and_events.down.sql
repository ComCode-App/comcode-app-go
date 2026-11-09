DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE slug IN (
        'platform.analytics.read',
        'platform.analytics.manage',
        'platform.geoip.read',
        'platform.geoip.manage',
        'platform.events.manage'
    )
);

DELETE FROM permissions WHERE slug IN (
    'platform.analytics.read',
    'platform.analytics.manage',
    'platform.geoip.read',
    'platform.geoip.manage',
    'platform.events.manage'
);

DROP TABLE IF EXISTS geoip_settings CASCADE;
DROP TABLE IF EXISTS analytics_settings CASCADE;
DROP TABLE IF EXISTS event_processing_log CASCADE;
DROP TABLE IF EXISTS event_aggregates_daily CASCADE;
DROP TABLE IF EXISTS event_aggregates_hourly CASCADE;
DROP TABLE IF EXISTS event_definitions CASCADE;
DROP TABLE IF EXISTS events CASCADE;
