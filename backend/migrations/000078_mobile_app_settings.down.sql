-- Drops the mobile version gate settings (data loss: the configured versions and store URLs).
ALTER TABLE app_settings
    DROP COLUMN IF EXISTS mobile_android_store_url,
    DROP COLUMN IF EXISTS mobile_ios_store_url,
    DROP COLUMN IF EXISTS mobile_latest_version,
    DROP COLUMN IF EXISTS mobile_min_version;
