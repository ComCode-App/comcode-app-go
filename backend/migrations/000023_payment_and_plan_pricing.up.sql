CREATE TYPE payment_provider AS ENUM ('paytr', 'iyzico');
CREATE TYPE active_payment_provider AS ENUM ('none', 'paytr', 'iyzico');
CREATE TYPE subscription_source AS ENUM ('manual', 'payment');
CREATE TYPE checkout_session_status AS ENUM ('pending', 'completed', 'failed', 'expired');

ALTER TABLE plans
    ADD COLUMN price_minor BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN currency CHAR(3) NOT NULL DEFAULT 'TRY',
    ADD COLUMN trial_days INT NOT NULL DEFAULT 0;

UPDATE plans SET price_minor = 99900, currency = 'TRY' WHERE slug = 'pro-monthly';
UPDATE plans SET price_minor = 999000, currency = 'TRY' WHERE slug = 'pro-yearly';

ALTER TABLE plans
    ADD CONSTRAINT chk_plans_free_price CHECK (
        (billing_interval = 'free' AND price_minor = 0)
        OR (billing_interval <> 'free' AND price_minor > 0)
    );

CREATE TABLE payment_settings (
    id               SMALLINT PRIMARY KEY DEFAULT 1,
    active_provider  active_payment_provider NOT NULL DEFAULT 'none',
    test_mode        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT payment_settings_singleton CHECK (id = 1)
);

INSERT INTO payment_settings (id) VALUES (1);

CREATE TRIGGER trg_payment_settings_set_updated_at
    BEFORE UPDATE ON payment_settings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE payment_provider_settings (
    provider          payment_provider PRIMARY KEY,
    enabled           BOOLEAN NOT NULL DEFAULT FALSE,
    merchant_id       TEXT NOT NULL DEFAULT '',
    base_url          TEXT NOT NULL DEFAULT '',
    merchant_key_enc  TEXT NULL,
    merchant_salt_enc TEXT NULL,
    api_key_enc       TEXT NULL,
    secret_key_enc    TEXT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO payment_provider_settings (provider) VALUES ('paytr'), ('iyzico');

CREATE TRIGGER trg_payment_provider_settings_set_updated_at
    BEFORE UPDATE ON payment_provider_settings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE payment_checkout_sessions (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID NOT NULL DEFAULT gen_random_uuid(),
    user_id         BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    plan_id         BIGINT NOT NULL REFERENCES plans (id),
    provider        payment_provider NOT NULL,
    status          checkout_session_status NOT NULL DEFAULT 'pending',
    merchant_oid    TEXT NOT NULL,
    provider_token  TEXT NULL,
    provider_ref    TEXT NULL,
    amount_minor    BIGINT NOT NULL,
    currency        CHAR(3) NOT NULL DEFAULT 'TRY',
    checkout_url    TEXT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at      TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_payment_checkout_sessions_uuid UNIQUE (uuid),
    CONSTRAINT uq_payment_checkout_sessions_merchant_oid UNIQUE (merchant_oid)
);

CREATE INDEX idx_payment_checkout_sessions_user ON payment_checkout_sessions (user_id);
CREATE INDEX idx_payment_checkout_sessions_status ON payment_checkout_sessions (status);

CREATE TRIGGER trg_payment_checkout_sessions_set_updated_at
    BEFORE UPDATE ON payment_checkout_sessions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

ALTER TABLE subscriptions
    ADD COLUMN source subscription_source NOT NULL DEFAULT 'manual',
    ADD COLUMN provider payment_provider NULL,
    ADD COLUMN provider_subscription_ref TEXT NULL,
    ADD COLUMN checkout_session_id BIGINT NULL REFERENCES payment_checkout_sessions (id),
    ADD COLUMN next_billing_at TIMESTAMPTZ NULL,
    ADD COLUMN canceled_at TIMESTAMPTZ NULL;

CREATE INDEX idx_subscriptions_checkout_session ON subscriptions (checkout_session_id)
    WHERE checkout_session_id IS NOT NULL;

INSERT INTO permissions (name, slug) VALUES
    ('Read platform payment settings', 'platform.payment.read'),
    ('Write platform payment settings', 'platform.payment.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug IN ('platform.payment.read', 'platform.payment.write');
