-- Drops the native audiences (native Apple / Google sign-in stops until they are re-entered).
ALTER TABLE oauth_provider_settings
    DROP COLUMN IF EXISTS native_client_ids;
