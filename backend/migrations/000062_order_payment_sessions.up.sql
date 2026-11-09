-- TEC-943: card orders are paid online through the payment providers used for subscriptions.
-- One row per checkout attempt of an order; an order may have several (a failed or abandoned
-- attempt leaves the order payable). merchant_oid starts with "ord" so callbacks can tell order
-- payments from subscription ("chk") and card-registration ("card") sessions.
CREATE TABLE order_payment_sessions (
    id             BIGSERIAL PRIMARY KEY,
    uuid           UUID                    NOT NULL DEFAULT gen_random_uuid(),
    account_id     BIGINT                  NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    order_id       BIGINT                  NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    user_id        BIGINT                  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider       payment_provider        NOT NULL,
    status         checkout_session_status NOT NULL DEFAULT 'pending',
    merchant_oid   TEXT                    NOT NULL,
    provider_token TEXT                    NULL,
    provider_ref   TEXT                    NULL,
    amount_minor   BIGINT                  NOT NULL CHECK (amount_minor > 0),
    currency       CHAR(3)                 NOT NULL DEFAULT 'TRY',
    checkout_url   TEXT                    NULL,
    metadata       JSONB                   NOT NULL DEFAULT '{}'::jsonb,
    expires_at     TIMESTAMPTZ             NOT NULL,
    completed_at   TIMESTAMPTZ             NULL,
    created_at     TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_order_payment_sessions_uuid UNIQUE (uuid),
    CONSTRAINT uq_order_payment_sessions_merchant_oid UNIQUE (merchant_oid),
    CONSTRAINT chk_order_payment_sessions_merchant_oid CHECK (merchant_oid LIKE 'ord%')
);

CREATE INDEX idx_order_payment_sessions_order ON order_payment_sessions (order_id, created_at DESC);
CREATE INDEX idx_order_payment_sessions_provider_token ON order_payment_sessions (provider_token)
    WHERE provider_token IS NOT NULL;

CREATE TRIGGER trg_order_payment_sessions_set_updated_at
    BEFORE UPDATE ON order_payment_sessions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
