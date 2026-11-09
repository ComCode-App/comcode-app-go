CREATE TABLE events (
    id               BIGSERIAL,
    uuid             UUID         NOT NULL DEFAULT gen_random_uuid(),
    event_type       VARCHAR(100) NOT NULL,
    schema_version   INT          NOT NULL DEFAULT 1,
    subject_type     VARCHAR(64)  NOT NULL,
    subject_id       VARCHAR(128) NOT NULL,
    actor_type       VARCHAR(64)  NULL,
    actor_id         VARCHAR(128) NULL,
    session_id       VARCHAR(128) NULL,
    request_id       VARCHAR(128) NULL,
    ip_hash          VARCHAR(64)  NULL,
    country_code     VARCHAR(8)   NULL,
    country_name     VARCHAR(100) NULL,
    city             VARCHAR(100) NULL,
    timezone         VARCHAR(64)  NULL,
    latitude         DOUBLE PRECISION NULL,
    longitude        DOUBLE PRECISION NULL,
    device_type      VARCHAR(32)  NULL,
    device_brand     VARCHAR(64)  NULL,
    device_model     VARCHAR(64)  NULL,
    browser          VARCHAR(64)  NULL,
    browser_version  VARCHAR(32)  NULL,
    os               VARCHAR(64)  NULL,
    os_version       VARCHAR(32)  NULL,
    referer          TEXT         NULL,
    user_agent       TEXT         NULL,
    properties       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    context          JSONB        NOT NULL DEFAULT '{}'::jsonb,
    metadata         JSONB        NOT NULL DEFAULT '{}'::jsonb,
    occurred_at      TIMESTAMPTZ  NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, occurred_at)
) PARTITION BY RANGE (occurred_at);

-- Default partition handles any timestamps outside specific ranges
CREATE TABLE events_default PARTITION OF events DEFAULT;

-- Initial monthly partitions for 2026 and 2027
CREATE TABLE events_2026_01 PARTITION OF events FOR VALUES FROM ('2026-01-01 00:00:00+00') TO ('2026-02-01 00:00:00+00');
CREATE TABLE events_2026_02 PARTITION OF events FOR VALUES FROM ('2026-02-01 00:00:00+00') TO ('2026-03-01 00:00:00+00');
CREATE TABLE events_2026_03 PARTITION OF events FOR VALUES FROM ('2026-03-01 00:00:00+00') TO ('2026-04-01 00:00:00+00');
CREATE TABLE events_2026_04 PARTITION OF events FOR VALUES FROM ('2026-04-01 00:00:00+00') TO ('2026-05-01 00:00:00+00');
CREATE TABLE events_2026_05 PARTITION OF events FOR VALUES FROM ('2026-05-01 00:00:00+00') TO ('2026-06-01 00:00:00+00');
CREATE TABLE events_2026_06 PARTITION OF events FOR VALUES FROM ('2026-06-01 00:00:00+00') TO ('2026-07-01 00:00:00+00');
CREATE TABLE events_2026_07 PARTITION OF events FOR VALUES FROM ('2026-07-01 00:00:00+00') TO ('2026-08-01 00:00:00+00');
CREATE TABLE events_2026_08 PARTITION OF events FOR VALUES FROM ('2026-08-01 00:00:00+00') TO ('2026-09-01 00:00:00+00');
CREATE TABLE events_2026_09 PARTITION OF events FOR VALUES FROM ('2026-09-01 00:00:00+00') TO ('2026-10-01 00:00:00+00');
CREATE TABLE events_2026_10 PARTITION OF events FOR VALUES FROM ('2026-10-01 00:00:00+00') TO ('2026-11-01 00:00:00+00');
CREATE TABLE events_2026_11 PARTITION OF events FOR VALUES FROM ('2026-11-01 00:00:00+00') TO ('2026-12-01 00:00:00+00');
CREATE TABLE events_2026_12 PARTITION OF events FOR VALUES FROM ('2026-12-01 00:00:00+00') TO ('2027-01-01 00:00:00+00');

CREATE TABLE events_2027_01 PARTITION OF events FOR VALUES FROM ('2027-01-01 00:00:00+00') TO ('2027-02-01 00:00:00+00');
CREATE TABLE events_2027_02 PARTITION OF events FOR VALUES FROM ('2027-02-01 00:00:00+00') TO ('2027-03-01 00:00:00+00');
CREATE TABLE events_2027_03 PARTITION OF events FOR VALUES FROM ('2027-03-01 00:00:00+00') TO ('2027-04-01 00:00:00+00');
CREATE TABLE events_2027_04 PARTITION OF events FOR VALUES FROM ('2027-04-01 00:00:00+00') TO ('2027-05-01 00:00:00+00');
CREATE TABLE events_2027_05 PARTITION OF events FOR VALUES FROM ('2027-05-01 00:00:00+00') TO ('2027-06-01 00:00:00+00');
CREATE TABLE events_2027_06 PARTITION OF events FOR VALUES FROM ('2027-06-01 00:00:00+00') TO ('2027-07-01 00:00:00+00');

CREATE INDEX idx_events_type_occurred ON events (event_type, occurred_at DESC);
CREATE INDEX idx_events_subject_occurred ON events (subject_type, subject_id, occurred_at DESC);
CREATE INDEX idx_events_actor_occurred ON events (actor_type, actor_id, occurred_at DESC) WHERE actor_id IS NOT NULL;
CREATE INDEX idx_events_session_occurred ON events (session_id, occurred_at DESC) WHERE session_id IS NOT NULL;
CREATE INDEX idx_events_uuid ON events (uuid);

CREATE TABLE event_definitions (
    id                 BIGSERIAL PRIMARY KEY,
    key                VARCHAR(100) NOT NULL,
    name               VARCHAR(200) NOT NULL,
    description        TEXT         NULL,
    category           VARCHAR(64)  NOT NULL DEFAULT 'general',
    schema_version     INT          NOT NULL DEFAULT 1,
    enabled            BOOLEAN      NOT NULL DEFAULT true,
    is_public          BOOLEAN      NOT NULL DEFAULT true,
    aggregate_enabled  BOOLEAN      NOT NULL DEFAULT true,
    retention_days     INT          NOT NULL DEFAULT 365,
    required_properties TEXT[]      NOT NULL DEFAULT '{}'::text[],
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_event_definitions_key UNIQUE (key)
);

CREATE TRIGGER trg_event_definitions_set_updated_at
    BEFORE UPDATE ON event_definitions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE event_aggregates_hourly (
    id                 BIGSERIAL PRIMARY KEY,
    bucket_hour        TIMESTAMPTZ  NOT NULL,
    subject_type       VARCHAR(64)  NOT NULL,
    subject_id         VARCHAR(128) NOT NULL,
    event_type         VARCHAR(100) NOT NULL,
    total_count        BIGINT       NOT NULL DEFAULT 0,
    unique_sessions    BIGINT       NOT NULL DEFAULT 0,
    country_breakdown  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    city_breakdown     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    device_breakdown   JSONB        NOT NULL DEFAULT '{}'::jsonb,
    browser_breakdown  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    os_breakdown       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_event_aggregates_hourly UNIQUE (bucket_hour, subject_type, subject_id, event_type)
);

CREATE INDEX idx_event_aggregates_hourly_lookup ON event_aggregates_hourly (subject_type, subject_id, event_type, bucket_hour);

CREATE TABLE event_aggregates_daily (
    id                 BIGSERIAL PRIMARY KEY,
    bucket_date        DATE         NOT NULL,
    subject_type       VARCHAR(64)  NOT NULL,
    subject_id         VARCHAR(128) NOT NULL,
    event_type         VARCHAR(100) NOT NULL,
    total_count        BIGINT       NOT NULL DEFAULT 0,
    unique_sessions    BIGINT       NOT NULL DEFAULT 0,
    country_breakdown  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    city_breakdown     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    device_breakdown   JSONB        NOT NULL DEFAULT '{}'::jsonb,
    browser_breakdown  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    os_breakdown       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    referer_breakdown  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_event_aggregates_daily UNIQUE (bucket_date, subject_type, subject_id, event_type)
);

CREATE INDEX idx_event_aggregates_daily_lookup ON event_aggregates_daily (subject_type, subject_id, event_type, bucket_date);

CREATE TABLE event_processing_log (
    event_id           BIGINT       NOT NULL,
    occurred_at        TIMESTAMPTZ  NOT NULL,
    processed_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, occurred_at)
);

CREATE TABLE analytics_settings (
    id                          INT PRIMARY KEY DEFAULT 1,
    enabled                     BOOLEAN NOT NULL DEFAULT true,
    event_delivery_mode         TEXT NOT NULL DEFAULT 'async',
    raw_event_retention_days    INT NOT NULL DEFAULT 730,
    aggregate_retention_days    INT NOT NULL DEFAULT 1095,
    queue_name                  TEXT NOT NULL DEFAULT 'analytics',
    realtime_enabled            BOOLEAN NOT NULL DEFAULT true,
    batch_size                  INT NOT NULL DEFAULT 200,
    batch_flush_interval_ms     INT NOT NULL DEFAULT 1000,
    ip_hash_salt_rotation_days  INT NOT NULL DEFAULT 7,
    anonymize_ip                BOOLEAN NOT NULL DEFAULT true,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_analytics_settings_single_row CHECK (id = 1),
    CONSTRAINT chk_analytics_delivery_mode CHECK (event_delivery_mode IN ('async', 'sync', 'hybrid'))
);

CREATE TRIGGER trg_analytics_settings_set_updated_at
    BEFORE UPDATE ON analytics_settings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

INSERT INTO analytics_settings (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE geoip_settings (
    id                  INT PRIMARY KEY DEFAULT 1,
    enabled             BOOLEAN NOT NULL DEFAULT true,
    provider            TEXT NOT NULL DEFAULT 'maxmind',
    database_type       TEXT NOT NULL DEFAULT 'geolite2-city',
    database_path       TEXT NOT NULL DEFAULT '/data/geoip/GeoLite2-City.mmdb',
    auto_update_enabled BOOLEAN NOT NULL DEFAULT true,
    update_interval_hours INT NOT NULL DEFAULT 24,
    account_id_encrypted  TEXT NULL,
    license_key_encrypted TEXT NULL,
    download_url_template TEXT NULL,
    cache_enabled       BOOLEAN NOT NULL DEFAULT true,
    cache_ttl_hours     INT NOT NULL DEFAULT 24,
    cache_max_entries   INT NOT NULL DEFAULT 100000,
    last_update_at      TIMESTAMPTZ NULL,
    last_update_status  TEXT NOT NULL DEFAULT 'pending',
    last_update_error   TEXT NULL,
    database_version    TEXT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_geoip_settings_single_row CHECK (id = 1)
);

CREATE TRIGGER trg_geoip_settings_set_updated_at
    BEFORE UPDATE ON geoip_settings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

INSERT INTO geoip_settings (id) VALUES (1) ON CONFLICT DO NOTHING;

INSERT INTO permissions (name, slug) VALUES
    ('Read analytics', 'platform.analytics.read'),
    ('Manage analytics', 'platform.analytics.manage'),
    ('Read GeoIP settings', 'platform.geoip.read'),
    ('Manage GeoIP settings', 'platform.geoip.manage'),
    ('Manage event definitions', 'platform.events.manage')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug IN (
    'platform.analytics.read',
    'platform.analytics.manage',
    'platform.geoip.read',
    'platform.geoip.manage',
    'platform.events.manage'
  )
ON CONFLICT DO NOTHING;

INSERT INTO event_definitions (key, name, description, category, schema_version, enabled, is_public, aggregate_enabled, retention_days, required_properties)
VALUES
    ('card.view', 'Card View', 'Visitor viewed digital business card', 'card', 1, true, true, true, 365, '{}'),
    ('card.qr.scan', 'Card QR Scan', 'Visitor scanned card QR code', 'card', 1, true, true, true, 365, '{}'),
    ('contact.phone.click', 'Phone Click', 'Visitor clicked phone button', 'contact', 1, true, true, true, 365, '{}'),
    ('contact.email.click', 'Email Click', 'Visitor clicked email button', 'contact', 1, true, true, true, 365, '{}'),
    ('contact.website.click', 'Website Click', 'Visitor clicked website button', 'contact', 1, true, true, true, 365, '{}'),
    ('social.instagram.click', 'Instagram Click', 'Visitor clicked Instagram link', 'social', 1, true, true, true, 365, '{}'),
    ('vcard.download', 'vCard Download', 'Visitor downloaded contact vCard', 'contact', 1, true, true, true, 365, '{}'),
    ('user.login', 'User Login', 'User authenticated to the system', 'auth', 1, true, false, true, 730, '{}'),
    ('user.logout', 'User Logout', 'User logged out', 'auth', 1, true, false, true, 730, '{}'),
    ('subscription.started', 'Subscription Started', 'Subscription created or activated', 'billing', 1, true, false, true, 1095, '{}'),
    ('subscription.cancelled', 'Subscription Cancelled', 'Subscription cancelled', 'billing', 1, true, false, true, 1095, '{}'),
    ('payment.completed', 'Payment Completed', 'Payment successfully completed', 'billing', 1, true, false, true, 1095, '{"amount","currency"}'),
    ('payment.failed', 'Payment Failed', 'Payment transaction failed', 'billing', 1, true, false, true, 1095, '{}')
ON CONFLICT (key) DO NOTHING;
