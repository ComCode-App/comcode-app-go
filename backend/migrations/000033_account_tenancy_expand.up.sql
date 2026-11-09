-- Account-based tenancy, expand phase (docs/TENANCY_AND_ACCESS.md).
-- Adds accounts + memberships, gives every user a personal account, migrates
-- companies to company accounts and backfills account_id on owned resources.
-- Legacy ownership columns stay until the contract migration; the
-- *_fill_account_id triggers keep not-yet-migrated write paths valid.

CREATE TYPE account_type AS ENUM ('personal', 'company');
CREATE TYPE account_status AS ENUM ('active', 'suspended');
CREATE TYPE account_role AS ENUM ('owner', 'admin', 'member');

CREATE TABLE accounts (
    id                BIGSERIAL PRIMARY KEY,
    uuid              UUID           NOT NULL DEFAULT gen_random_uuid(),
    type              account_type   NOT NULL,
    name              VARCHAR(200)   NOT NULL,
    slug              VARCHAR(100)   NOT NULL,
    status            account_status NOT NULL DEFAULT 'active',
    personal_user_id  BIGINT         NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Transitional: maps legacy companies.id; dropped in the contract migration.
    legacy_company_id BIGINT         NULL,
    created_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_accounts_uuid UNIQUE (uuid),
    CONSTRAINT uq_accounts_slug UNIQUE (slug),
    CONSTRAINT uq_accounts_personal_user UNIQUE (personal_user_id),
    CONSTRAINT uq_accounts_legacy_company UNIQUE (legacy_company_id),
    CONSTRAINT chk_accounts_personal_user CHECK ((type = 'personal') = (personal_user_id IS NOT NULL))
);

CREATE TRIGGER trg_accounts_set_updated_at
    BEFORE UPDATE ON accounts
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE account_members (
    id         BIGSERIAL PRIMARY KEY,
    account_id BIGINT       NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    user_id    BIGINT       NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       account_role NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_account_members_account_user UNIQUE (account_id, user_id)
);

CREATE INDEX idx_account_members_user ON account_members (user_id);

CREATE TRIGGER trg_account_members_set_updated_at
    BEFORE UPDATE ON account_members
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- A personal account has exactly one member: its user, as owner.
CREATE OR REPLACE FUNCTION guard_personal_account_members()
RETURNS TRIGGER AS $$
DECLARE
    acc accounts%ROWTYPE;
BEGIN
    SELECT * INTO acc FROM accounts WHERE id = NEW.account_id;
    IF acc.type = 'personal' AND (NEW.user_id <> acc.personal_user_id OR NEW.role <> 'owner') THEN
        RAISE EXCEPTION 'personal account % only admits its user as owner', acc.uuid
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_account_members_guard_personal
    BEFORE INSERT OR UPDATE ON account_members
    FOR EACH ROW
    EXECUTE FUNCTION guard_personal_account_members();

-- Every user gets a personal account, whatever code path created the user.
CREATE OR REPLACE FUNCTION create_personal_account()
RETURNS TRIGGER AS $$
DECLARE
    new_account_id BIGINT;
BEGIN
    INSERT INTO accounts (type, name, slug, personal_user_id)
    VALUES (
        'personal',
        COALESCE(NULLIF(TRIM(NEW.name || ' ' || NEW.surname), ''), NEW.email),
        'u-' || NEW.uuid::text,
        NEW.id
    )
    RETURNING id INTO new_account_id;

    INSERT INTO account_members (account_id, user_id, role)
    VALUES (new_account_id, NEW.id, 'owner');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_create_personal_account
    AFTER INSERT ON users
    FOR EACH ROW
    EXECUTE FUNCTION create_personal_account();

-- Backfill personal accounts for existing users.
INSERT INTO accounts (type, name, slug, personal_user_id)
SELECT 'personal',
       COALESCE(NULLIF(TRIM(u.name || ' ' || u.surname), ''), u.email),
       'u-' || u.uuid::text,
       u.id
FROM users u;

INSERT INTO account_members (account_id, user_id, role)
SELECT a.id, a.personal_user_id, 'owner'
FROM accounts a
WHERE a.type = 'personal';

-- Companies become company accounts (same uuid and slug).
INSERT INTO accounts (uuid, type, name, slug, status, legacy_company_id, created_at)
SELECT c.uuid,
       'company',
       c.name,
       c.slug,
       CASE WHEN c.status = 'active' THEN 'active'::account_status ELSE 'suspended'::account_status END,
       c.id,
       c.created_at
FROM companies c;

-- The earliest manager of each company becomes owner, other managers admins, customers members.
INSERT INTO account_members (account_id, user_id, role, created_at)
SELECT a.id,
       cm.user_id,
       CASE
           WHEN cm.role = 'manager' AND cm.id = first_manager.id THEN 'owner'::account_role
           WHEN cm.role = 'manager' THEN 'admin'::account_role
           ELSE 'member'::account_role
       END,
       cm.created_at
FROM company_members cm
JOIN accounts a ON a.legacy_company_id = cm.company_id
LEFT JOIN LATERAL (
    SELECT id FROM company_members m
    WHERE m.company_id = cm.company_id AND m.role = 'manager'
    ORDER BY m.created_at, m.id
    LIMIT 1
) first_manager ON true;

-- Session-level active account.
ALTER TABLE refresh_tokens
    ADD COLUMN active_account_id BIGINT NULL REFERENCES accounts (id) ON DELETE SET NULL;

-- Resolution helpers used by backfill and the transitional fill triggers.
CREATE OR REPLACE FUNCTION personal_account_id(p_user_id BIGINT)
RETURNS BIGINT AS $$
    SELECT id FROM accounts WHERE personal_user_id = p_user_id;
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION company_account_id(p_company_id BIGINT)
RETURNS BIGINT AS $$
    SELECT id FROM accounts WHERE legacy_company_id = p_company_id;
$$ LANGUAGE sql STABLE;

-- Owned resources --------------------------------------------------------

-- Tables whose legacy owner is user_id only.
ALTER TABLE profiles                   ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE CASCADE;
ALTER TABLE user_bank_accounts         ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE CASCADE;
ALTER TABLE payment_checkout_sessions  ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE CASCADE;
ALTER TABLE payment_saved_cards        ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE CASCADE;
ALTER TABLE payment_card_sessions      ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE CASCADE;
ALTER TABLE payment_provider_customers ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE CASCADE;

UPDATE profiles                   SET account_id = personal_account_id(user_id);
UPDATE user_bank_accounts         SET account_id = personal_account_id(user_id);
UPDATE payment_checkout_sessions  SET account_id = personal_account_id(user_id);
UPDATE payment_saved_cards        SET account_id = personal_account_id(user_id);
UPDATE payment_card_sessions      SET account_id = personal_account_id(user_id);
UPDATE payment_provider_customers SET account_id = personal_account_id(user_id);

-- Tables with an optional legacy company owner.
ALTER TABLE nfc_cards     ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE RESTRICT;
ALTER TABLE orders        ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE RESTRICT;
ALTER TABLE subscriptions ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE CASCADE;

UPDATE nfc_cards SET account_id = COALESCE(company_account_id(company_id), personal_account_id(user_id));
UPDATE orders    SET account_id = COALESCE(company_account_id(company_id), personal_account_id(user_id));
UPDATE subscriptions SET account_id = COALESCE(company_account_id(company_id), personal_account_id(user_id));

-- Routes may be unowned (system reserved, unclaimed legacy routes).
ALTER TABLE routes ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE SET NULL;

UPDATE routes
SET account_id = COALESCE(company_account_id(company_id), personal_account_id(owner_user_id))
WHERE company_id IS NOT NULL OR owner_user_id IS NOT NULL;

ALTER TABLE profiles                   ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE user_bank_accounts         ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE payment_checkout_sessions  ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE payment_saved_cards        ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE payment_card_sessions      ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE payment_provider_customers ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE nfc_cards                  ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE orders                     ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE subscriptions              ALTER COLUMN account_id SET NOT NULL;

CREATE INDEX idx_profiles_account ON profiles (account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_user_bank_accounts_account ON user_bank_accounts (account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_payment_checkout_sessions_account ON payment_checkout_sessions (account_id);
CREATE INDEX idx_payment_saved_cards_account ON payment_saved_cards (account_id);
CREATE INDEX idx_payment_card_sessions_account ON payment_card_sessions (account_id);
CREATE INDEX idx_nfc_cards_account_active ON nfc_cards (account_id, is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_orders_account ON orders (account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_routes_account ON routes (account_id, route_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_subscriptions_account ON subscriptions (account_id);
CREATE UNIQUE INDEX uq_subscriptions_active_account ON subscriptions (account_id) WHERE status = 'active';

-- Transitional fill triggers ------------------------------------------------
-- Write paths that still set only user_id/company_id get account_id resolved.
-- Removed in the contract migration once every module writes account_id.

CREATE OR REPLACE FUNCTION fill_account_id_from_user()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.account_id IS NULL THEN
        NEW.account_id := personal_account_id(NEW.user_id);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- On UPDATE, a legacy owner change (e.g. card activation rewriting user_id)
-- re-derives account_id unless the statement set account_id itself.
CREATE OR REPLACE FUNCTION fill_account_id_from_company_or_user()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.account_id IS NULL
       OR (TG_OP = 'UPDATE'
           AND NEW.account_id IS NOT DISTINCT FROM OLD.account_id
           AND (NEW.user_id IS DISTINCT FROM OLD.user_id OR NEW.company_id IS DISTINCT FROM OLD.company_id)) THEN
        NEW.account_id := COALESCE(company_account_id(NEW.company_id), personal_account_id(NEW.user_id));
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fill_route_account_id()
RETURNS TRIGGER AS $$
BEGIN
    IF (NEW.account_id IS NULL
        OR (TG_OP = 'UPDATE'
            AND NEW.account_id IS NOT DISTINCT FROM OLD.account_id
            AND (NEW.owner_user_id IS DISTINCT FROM OLD.owner_user_id OR NEW.company_id IS DISTINCT FROM OLD.company_id)))
       AND (NEW.company_id IS NOT NULL OR NEW.owner_user_id IS NOT NULL) THEN
        NEW.account_id := COALESCE(company_account_id(NEW.company_id), personal_account_id(NEW.owner_user_id));
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_profiles_fill_account_id BEFORE INSERT ON profiles
    FOR EACH ROW EXECUTE FUNCTION fill_account_id_from_user();
CREATE TRIGGER trg_user_bank_accounts_fill_account_id BEFORE INSERT ON user_bank_accounts
    FOR EACH ROW EXECUTE FUNCTION fill_account_id_from_user();
CREATE TRIGGER trg_payment_checkout_sessions_fill_account_id BEFORE INSERT ON payment_checkout_sessions
    FOR EACH ROW EXECUTE FUNCTION fill_account_id_from_user();
CREATE TRIGGER trg_payment_saved_cards_fill_account_id BEFORE INSERT ON payment_saved_cards
    FOR EACH ROW EXECUTE FUNCTION fill_account_id_from_user();
CREATE TRIGGER trg_payment_card_sessions_fill_account_id BEFORE INSERT ON payment_card_sessions
    FOR EACH ROW EXECUTE FUNCTION fill_account_id_from_user();
CREATE TRIGGER trg_payment_provider_customers_fill_account_id BEFORE INSERT ON payment_provider_customers
    FOR EACH ROW EXECUTE FUNCTION fill_account_id_from_user();
CREATE TRIGGER trg_nfc_cards_fill_account_id BEFORE INSERT OR UPDATE OF user_id, company_id ON nfc_cards
    FOR EACH ROW EXECUTE FUNCTION fill_account_id_from_company_or_user();
CREATE TRIGGER trg_orders_fill_account_id BEFORE INSERT ON orders
    FOR EACH ROW EXECUTE FUNCTION fill_account_id_from_company_or_user();
CREATE TRIGGER trg_subscriptions_fill_account_id BEFORE INSERT ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION fill_account_id_from_company_or_user();
CREATE TRIGGER trg_routes_fill_account_id BEFORE INSERT OR UPDATE OF owner_user_id, company_id ON routes
    FOR EACH ROW EXECUTE FUNCTION fill_route_account_id();

-- Permissions ------------------------------------------------------------

INSERT INTO permissions (name, slug) VALUES
    ('Read platform accounts', 'platform.accounts.read'),
    ('Write platform accounts', 'platform.accounts.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug IN ('platform.accounts.read', 'platform.accounts.write');
