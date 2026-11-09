-- Refresh token families: rotation keeps one family per device session, so the
-- access token's sid (the family) stays valid across rotation and a device can
-- be revoked as a whole. See docs/TENANCY_AND_ACCESS.md#session--tokens.

ALTER TABLE refresh_tokens
    ADD COLUMN family_uuid UUID NULL;

UPDATE refresh_tokens SET family_uuid = uuid;

ALTER TABLE refresh_tokens
    ALTER COLUMN family_uuid SET NOT NULL,
    ALTER COLUMN family_uuid SET DEFAULT gen_random_uuid();

CREATE INDEX idx_refresh_tokens_family_active
    ON refresh_tokens (family_uuid)
    WHERE revoked_at IS NULL;
