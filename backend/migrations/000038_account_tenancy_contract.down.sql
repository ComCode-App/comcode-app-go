-- Restores the legacy company schema shape only. Data loss: company rows, company memberships and
-- per-row company/user ownership on subscriptions are not reconstructed (account_id stays the owner).
CREATE TYPE company_member_role AS ENUM ('manager', 'customer');
CREATE TYPE company_status AS ENUM ('active', 'inactive');

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

CREATE TABLE company_members (
    id         BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       company_member_role NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_company_members_company_user UNIQUE (company_id, user_id)
);

ALTER TABLE accounts ADD COLUMN legacy_company_id BIGINT NULL;
ALTER TABLE accounts ADD CONSTRAINT uq_accounts_legacy_company UNIQUE (legacy_company_id);

ALTER TABLE subscriptions ADD COLUMN user_id BIGINT NULL REFERENCES users (id) ON DELETE CASCADE;
ALTER TABLE subscriptions ADD COLUMN company_id BIGINT NULL REFERENCES companies (id) ON DELETE CASCADE;
ALTER TABLE routes        ADD COLUMN company_id BIGINT NULL REFERENCES companies (id) ON DELETE SET NULL;
ALTER TABLE orders        ADD COLUMN company_id BIGINT NULL REFERENCES companies (id) ON DELETE SET NULL;
ALTER TABLE nfc_cards     ADD COLUMN company_id BIGINT NULL REFERENCES companies (id) ON DELETE SET NULL;

CREATE OR REPLACE FUNCTION personal_account_id(p_user_id BIGINT)
RETURNS BIGINT AS $$
    SELECT id FROM accounts WHERE personal_user_id = p_user_id;
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION company_account_id(p_company_id BIGINT)
RETURNS BIGINT AS $$
    SELECT id FROM accounts WHERE legacy_company_id = p_company_id;
$$ LANGUAGE sql STABLE;
