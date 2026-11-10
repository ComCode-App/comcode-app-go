-- Apple token exchange and revocation (TEC-1181, docs/design/mobile.md §2.3).
-- client_id is the Apple client the stored refresh token (refresh_token_enc,
-- encrypted with APP_ENCRYPTION_KEY) was issued to: the iOS bundle id for a
-- native sign-in. Apple revokes a token only with the same client_id, so it is
-- kept next to the token. NULL for rows written by the web (Auth.js) flow,
-- which revoke with the web Services ID. Not secret.
ALTER TABLE oauth_accounts
    ADD COLUMN client_id TEXT NULL;
