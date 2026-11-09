CREATE TYPE payment_card_session_status AS ENUM ('pending', 'completed', 'failed');

CREATE TABLE payment_provider_customers (
    user_id               BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider              payment_provider NOT NULL,
    provider_customer_ref TEXT NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, provider)
);

CREATE TRIGGER trg_payment_provider_customers_set_updated_at
    BEFORE UPDATE ON payment_provider_customers
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE payment_saved_cards (
    id                    BIGSERIAL PRIMARY KEY,
    uuid                  UUID NOT NULL DEFAULT gen_random_uuid(),
    user_id               BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider              payment_provider NOT NULL,
    provider_card_ref     TEXT NOT NULL,
    provider_customer_ref TEXT NOT NULL,
    last4                 TEXT NOT NULL DEFAULT '',
    brand                 TEXT NOT NULL DEFAULT '',
    exp_month             SMALLINT NULL,
    exp_year              SMALLINT NULL,
    is_default            BOOLEAN NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_payment_saved_cards_uuid UNIQUE (uuid),
    CONSTRAINT uq_payment_saved_cards_provider_ref UNIQUE (user_id, provider, provider_card_ref)
);

CREATE INDEX idx_payment_saved_cards_user ON payment_saved_cards (user_id);

CREATE TRIGGER trg_payment_saved_cards_set_updated_at
    BEFORE UPDATE ON payment_saved_cards
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE payment_card_sessions (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID NOT NULL DEFAULT gen_random_uuid(),
    user_id         BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider        payment_provider NOT NULL,
    status          payment_card_session_status NOT NULL DEFAULT 'pending',
    merchant_oid    TEXT NOT NULL,
    provider_token  TEXT NULL,
    checkout_url    TEXT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_payment_card_sessions_uuid UNIQUE (uuid),
    CONSTRAINT uq_payment_card_sessions_merchant_oid UNIQUE (merchant_oid)
);

CREATE INDEX idx_payment_card_sessions_user ON payment_card_sessions (user_id);

CREATE TRIGGER trg_payment_card_sessions_set_updated_at
    BEFORE UPDATE ON payment_card_sessions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

ALTER TABLE subscriptions
    ADD COLUMN saved_card_id BIGINT NULL REFERENCES payment_saved_cards (id);

CREATE INDEX idx_subscriptions_saved_card ON subscriptions (saved_card_id)
    WHERE saved_card_id IS NOT NULL;
