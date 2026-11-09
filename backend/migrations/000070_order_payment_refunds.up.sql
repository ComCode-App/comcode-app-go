-- TEC-935: refunds of online card order payments (order_payment_sessions, TEC-943) through the
-- provider's refund API. One row per refund request; a failed request can be retried on the same
-- row (same reference_no, so PayTR rejects a duplicate). Only succeeded and pending refunds count
-- against what is still refundable, and at most one refund per payment is pending at a time.
CREATE TYPE order_refund_status AS ENUM ('pending', 'succeeded', 'failed');

CREATE TABLE order_payment_refunds (
    id                   BIGSERIAL PRIMARY KEY,
    uuid                 UUID                NOT NULL DEFAULT gen_random_uuid(),
    account_id           BIGINT              NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    order_id             BIGINT              NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    session_id           BIGINT              NOT NULL REFERENCES order_payment_sessions(id) ON DELETE CASCADE,
    provider             payment_provider    NOT NULL,
    status               order_refund_status NOT NULL DEFAULT 'pending',
    amount_minor         BIGINT              NOT NULL CHECK (amount_minor > 0),
    currency             CHAR(3)             NOT NULL DEFAULT 'TRY',
    -- Sent to the provider (PayTR reference_no, iyzico conversationId).
    reference_no         TEXT                NOT NULL,
    -- Chosen by the caller; repeating a request with the same key returns the same refund.
    idempotency_key      TEXT                NOT NULL,
    reason               TEXT                NOT NULL DEFAULT '',
    provider_ref         TEXT                NULL,
    error_code           TEXT                NULL,
    error_message        TEXT                NULL,
    attempts             INT                 NOT NULL DEFAULT 1,
    requested_by_user_id BIGINT              NULL REFERENCES users(id) ON DELETE SET NULL,
    completed_at         TIMESTAMPTZ         NULL,
    created_at           TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_order_payment_refunds_uuid UNIQUE (uuid),
    CONSTRAINT uq_order_payment_refunds_reference_no UNIQUE (reference_no),
    CONSTRAINT uq_order_payment_refunds_idempotency_key UNIQUE (idempotency_key)
);

CREATE INDEX idx_order_payment_refunds_order ON order_payment_refunds (order_id, created_at DESC);
CREATE UNIQUE INDEX uq_order_payment_refunds_one_pending ON order_payment_refunds (session_id)
    WHERE status = 'pending';

CREATE TRIGGER trg_order_payment_refunds_set_updated_at
    BEFORE UPDATE ON order_payment_refunds
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
