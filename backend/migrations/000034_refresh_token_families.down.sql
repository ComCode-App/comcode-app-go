DROP INDEX IF EXISTS idx_refresh_tokens_family_active;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS family_uuid;
