CREATE TYPE feature_value_type AS ENUM ('bool', 'int');
CREATE TYPE billing_interval AS ENUM ('free', 'month', 'year', 'lifetime');
CREATE TYPE subscription_status AS ENUM ('active', 'trialing', 'past_due', 'canceled');
CREATE TYPE company_member_role AS ENUM ('manager', 'customer');
CREATE TYPE company_status AS ENUM ('active', 'inactive');

CREATE TABLE feature_definitions (
    key          TEXT         PRIMARY KEY,
    value_type   feature_value_type NOT NULL,
    label        VARCHAR(200) NOT NULL,
    description  TEXT         NULL,
    sort_order   INT          NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE plans (
    id               BIGSERIAL PRIMARY KEY,
    uuid             UUID         NOT NULL DEFAULT gen_random_uuid(),
    slug             VARCHAR(100) NOT NULL,
    name             VARCHAR(200) NOT NULL,
    description      TEXT         NULL,
    billing_interval billing_interval NOT NULL DEFAULT 'free',
    is_active        BOOLEAN      NOT NULL DEFAULT true,
    sort_order       INT          NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_plans_uuid UNIQUE (uuid),
    CONSTRAINT uq_plans_slug UNIQUE (slug)
);

CREATE TRIGGER trg_plans_set_updated_at
    BEFORE UPDATE ON plans
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE plan_limits (
    plan_id      BIGINT NOT NULL REFERENCES plans (id) ON DELETE CASCADE,
    feature_key  TEXT   NOT NULL REFERENCES feature_definitions (key) ON DELETE CASCADE,
    value_int    INT    NULL,
    value_bool   BOOLEAN NULL,
    PRIMARY KEY (plan_id, feature_key),
    CONSTRAINT chk_plan_limits_value CHECK (
        (value_int IS NOT NULL AND value_bool IS NULL)
        OR (value_int IS NULL AND value_bool IS NOT NULL)
    )
);

CREATE TABLE companies (
    id         BIGSERIAL PRIMARY KEY,
    uuid       UUID         NOT NULL DEFAULT gen_random_uuid(),
    name       VARCHAR(200) NOT NULL,
    slug       VARCHAR(100) NOT NULL,
    status     company_status NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_companies_uuid UNIQUE (uuid),
    CONSTRAINT uq_companies_slug UNIQUE (slug)
);

CREATE TRIGGER trg_companies_set_updated_at
    BEFORE UPDATE ON companies
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE company_members (
    id         BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       company_member_role NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_company_members_company_user UNIQUE (company_id, user_id)
);

CREATE INDEX idx_company_members_user ON company_members (user_id);
CREATE INDEX idx_company_members_company ON company_members (company_id);

CREATE TABLE subscriptions (
    id         BIGSERIAL PRIMARY KEY,
    uuid       UUID NOT NULL DEFAULT gen_random_uuid(),
    plan_id    BIGINT NOT NULL REFERENCES plans (id),
    status     subscription_status NOT NULL DEFAULT 'active',
    user_id    BIGINT NULL REFERENCES users (id) ON DELETE CASCADE,
    company_id BIGINT NULL REFERENCES companies (id) ON DELETE CASCADE,
    starts_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_subscriptions_uuid UNIQUE (uuid),
    CONSTRAINT chk_subscriptions_subject CHECK (
        (user_id IS NOT NULL AND company_id IS NULL)
        OR (user_id IS NULL AND company_id IS NOT NULL)
    )
);

CREATE TRIGGER trg_subscriptions_set_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE UNIQUE INDEX uq_subscriptions_active_user
    ON subscriptions (user_id)
    WHERE status = 'active' AND user_id IS NOT NULL;

CREATE UNIQUE INDEX uq_subscriptions_active_company
    ON subscriptions (company_id)
    WHERE status = 'active' AND company_id IS NOT NULL;

CREATE INDEX idx_subscriptions_plan ON subscriptions (plan_id);
CREATE INDEX idx_subscriptions_user ON subscriptions (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_subscriptions_company ON subscriptions (company_id) WHERE company_id IS NOT NULL;

INSERT INTO feature_definitions (key, value_type, label, description, sort_order) VALUES
    ('cards.max', 'int', 'Maximum cards', 'Maximum number of digital business cards', 10),
    ('nfc.enabled', 'bool', 'NFC enabled', 'Whether NFC features are available', 20);

INSERT INTO plans (slug, name, description, billing_interval, is_active, sort_order) VALUES
    ('free', 'Free', 'Free tier with basic limits', 'free', true, 0),
    ('pro-monthly', 'Pro Monthly', 'Pro plan billed monthly', 'month', true, 10),
    ('pro-yearly', 'Pro Yearly', 'Pro plan billed yearly', 'year', true, 20);

INSERT INTO plan_limits (plan_id, feature_key, value_int, value_bool)
SELECT p.id, 'cards.max', 1, NULL
FROM plans p WHERE p.slug = 'free';

INSERT INTO plan_limits (plan_id, feature_key, value_int, value_bool)
SELECT p.id, 'nfc.enabled', NULL, false
FROM plans p WHERE p.slug = 'free';

INSERT INTO plan_limits (plan_id, feature_key, value_int, value_bool)
SELECT p.id, 'cards.max', 10, NULL
FROM plans p WHERE p.slug IN ('pro-monthly', 'pro-yearly');

INSERT INTO plan_limits (plan_id, feature_key, value_int, value_bool)
SELECT p.id, 'nfc.enabled', NULL, true
FROM plans p WHERE p.slug IN ('pro-monthly', 'pro-yearly');
