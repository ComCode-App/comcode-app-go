-- Holding-account imports have no login/creator user. Ownership remains account-scoped.
ALTER TABLE profiles ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE user_bank_accounts ALTER COLUMN user_id DROP NOT NULL;
CREATE TABLE legacy_sync_users (
    legacy_user_id TEXT PRIMARY KEY,
    profile_id BIGINT NOT NULL UNIQUE REFERENCES profiles(id),
    legacy_hash TEXT NOT NULL UNIQUE,
    content_hash TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Deduplication identifiers only; no IPs or raw legacy events are retained.
CREATE TABLE legacy_sync_hit_ids (legacy_hit_id BIGINT PRIMARY KEY);
