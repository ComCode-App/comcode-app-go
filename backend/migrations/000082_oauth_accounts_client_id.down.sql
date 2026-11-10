-- Data-loss note: drops the Apple client id of stored refresh tokens; native
-- tokens stored before the rollback can then only be revoked with the web
-- Services ID (which Apple refuses for a bundle id token).
ALTER TABLE oauth_accounts
    DROP COLUMN IF EXISTS client_id;
