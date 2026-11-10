-- Audiences accepted for id_tokens from the native (mobile) app (TEC-1180,
-- docs/design/mobile.md KARAR-4): the iOS bundle id on the 'apple' row, the
-- Google iOS / Android (and, for Android Credential Manager, web) OAuth client
-- ids on the 'google' row. Managed from the platform admin panel. An empty list
-- turns native sign-in off for the provider. Not secret.
ALTER TABLE oauth_provider_settings
    ADD COLUMN native_client_ids TEXT[] NOT NULL DEFAULT '{}';
