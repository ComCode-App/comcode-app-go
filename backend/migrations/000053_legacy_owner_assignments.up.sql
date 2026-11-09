-- Audit identifiers intentionally have no resource FKs: rollback deletes those resources,
-- while the immutable journal must survive for review and idempotency.
CREATE TABLE legacy_owner_batches (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    mode TEXT NOT NULL CHECK (mode IN ('apply', 'rolled_back')),
    summary JSONB NOT NULL DEFAULT '{}'
);
CREATE TABLE legacy_owner_items (
    id BIGSERIAL PRIMARY KEY,
    batch_id UUID NOT NULL REFERENCES legacy_owner_batches(id),
    legacy_user_id TEXT NOT NULL,
    profile_id BIGINT NOT NULL,
    from_account_id BIGINT NOT NULL,
    to_account_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    created_user BOOLEAN NOT NULL,
    account_id BIGINT NOT NULL,
    created_account BOOLEAN NOT NULL,
    membership_id BIGINT NOT NULL,
    created_membership BOOLEAN NOT NULL,
    changes JSONB NOT NULL DEFAULT '[]',
    UNIQUE(batch_id, legacy_user_id)
);
CREATE INDEX idx_legacy_owner_items_batch ON legacy_owner_items(batch_id, id);
