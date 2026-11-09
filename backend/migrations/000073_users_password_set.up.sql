-- TEC-1105: password_set tells a password the user chose apart from the random
-- hash that OAuth sign-up (and legacy / CSV user imports) store, because
-- password_hash stays NOT NULL. It drives has_password on
-- GET /v1/auth/identities and the LAST_SIGN_IN_METHOD guard on unlinking the
-- last OAuth identity or deleting the last passkey.
ALTER TABLE users ADD COLUMN password_set BOOLEAN NOT NULL DEFAULT TRUE;

-- Backfill: an OAuth sign-up links its provider identity right after the user
-- row is created. A password reset done since then cannot be detected here;
-- those users set a password again (every password update flips it to TRUE).
UPDATE users u
SET password_set = FALSE
WHERE EXISTS (
    SELECT 1 FROM oauth_accounts oa
    WHERE oa.user_id = u.id
      AND oa.created_at <= u.created_at + INTERVAL '1 minute'
);
