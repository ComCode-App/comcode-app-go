DELETE FROM notification_templates WHERE code = 'auth.refresh_reuse';

-- Data-loss note: device labels and the rotation marker are dropped; sessions stay valid and
-- every session falls back to the web refresh lifetime on its next refresh.
ALTER TABLE refresh_tokens
    DROP CONSTRAINT chk_refresh_tokens_app_version,
    DROP CONSTRAINT chk_refresh_tokens_device_name,
    DROP CONSTRAINT chk_refresh_tokens_client_type,
    DROP COLUMN rotated_at,
    DROP COLUMN app_version,
    DROP COLUMN device_name,
    DROP COLUMN client_type;
