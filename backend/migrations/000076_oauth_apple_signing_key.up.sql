-- Sign in with Apple signing key (.p8) managed from the platform admin panel (TEC-1123).
-- The web (Auth.js) client_secret JWT is generated from this key on demand, so
-- the hand-made JWT pasted into client_secret_enc (which expires after at most
-- six months) becomes an optional fallback.
-- apple_private_key_enc is encrypted with the app SecretBox (APP_ENCRYPTION_KEY),
-- like client_secret_enc, and is never returned by the API. Only meaningful on
-- the 'apple' row.
ALTER TABLE oauth_provider_settings
    ADD COLUMN apple_team_id         TEXT NOT NULL DEFAULT '',
    ADD COLUMN apple_key_id          TEXT NOT NULL DEFAULT '',
    ADD COLUMN apple_private_key_enc TEXT NULL;
