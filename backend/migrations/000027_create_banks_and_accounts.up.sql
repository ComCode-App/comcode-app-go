CREATE TABLE banks (
    id            BIGSERIAL PRIMARY KEY,
    uuid          UUID         NOT NULL DEFAULT gen_random_uuid(),
    name          VARCHAR(150) NOT NULL,
    slug          VARCHAR(100) NOT NULL,
    code          VARCHAR(32)  NOT NULL DEFAULT '',
    country_code  VARCHAR(8)   NOT NULL DEFAULT 'TR',
    bic_swift     VARCHAR(32)  NOT NULL DEFAULT '',
    icon_url      TEXT         NOT NULL DEFAULT '',
    brand_color   VARCHAR(32)  NOT NULL DEFAULT '',
    iban_pattern  VARCHAR(255) NOT NULL DEFAULT '',
    description   TEXT         NOT NULL DEFAULT '',
    display_order INT          NOT NULL DEFAULT 0,
    is_active     BOOLEAN      NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ  NULL,
    CONSTRAINT uq_banks_uuid UNIQUE (uuid)
);

CREATE UNIQUE INDEX uq_banks_slug_active
    ON banks (slug)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_banks_code ON banks (code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_banks_country_active ON banks (country_code, is_active, display_order)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_banks_set_updated_at
    BEFORE UPDATE ON banks
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_bank_accounts (
    id             BIGSERIAL PRIMARY KEY,
    uuid           UUID         NOT NULL DEFAULT gen_random_uuid(),
    user_id        BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bank_id        BIGINT       NULL REFERENCES banks(id) ON DELETE SET NULL,
    account_holder VARCHAR(255) NOT NULL,
    iban           VARCHAR(64)  NOT NULL,
    account_number VARCHAR(64)  NOT NULL DEFAULT '',
    branch_code    VARCHAR(32)  NOT NULL DEFAULT '',
    currency       VARCHAR(8)   NOT NULL DEFAULT 'TRY',
    title          VARCHAR(150) NOT NULL DEFAULT '',
    is_primary     BOOLEAN      NOT NULL DEFAULT false,
    is_active      BOOLEAN      NOT NULL DEFAULT true,
    display_order  INT          NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ  NULL,
    CONSTRAINT uq_user_bank_accounts_uuid UNIQUE (uuid)
);

CREATE INDEX idx_user_bank_accounts_user_id ON user_bank_accounts (user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_user_bank_accounts_bank_id ON user_bank_accounts (bank_id);

CREATE TRIGGER trg_user_bank_accounts_set_updated_at
    BEFORE UPDATE ON user_bank_accounts
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Seed Banks (Turkey & Global Popular)
INSERT INTO banks (name, slug, code, country_code, bic_swift, brand_color, display_order, is_active) VALUES
    ('T.C. Ziraat Bankası', 'ziraat-bankasi', '00010', 'TR', 'TCZBTR2A', '#e11d48', 1, true),
    ('Türkiye İş Bankası', 'is-bankasi', '00064', 'TR', 'ISBKTRIS', '#1e3a8a', 2, true),
    ('Garanti BBVA', 'garanti-bbva', '00062', 'TR', 'TGBATRTR', '#059669', 3, true),
    ('Akbank', 'akbank', '00046', 'TR', 'AKBKTRIS', '#dc2626', 4, true),
    ('Yapı Kredi', 'yapi-kredi', '00067', 'TR', 'YKCITRIS', '#2563eb', 5, true),
    ('QNB Finansbank', 'qnb-finansbank', '00111', 'TR', 'FINBTRIS', '#7c2d12', 6, true),
    ('VakıfBank', 'vakifbank', '00015', 'TR', 'TVBATR2A', '#d97706', 7, true),
    ('Halkbank', 'halkbank', '00012', 'TR', 'TRHBTR2A', '#0284c7', 8, true),
    ('DenizBank', 'denizbank', '00134', 'TR', 'DENITRIS', '#0369a1', 9, true),
    ('Kuveyt Türk', 'kuveyt-turk', '00205', 'TR', 'KTEFTRIS', '#047857', 10, true),
    ('Enpara.com', 'enpara', '00148', 'TR', 'ENPATRIS', '#ea580c', 11, true),
    ('Papara', 'papara', '00812', 'TR', 'PAPATRIS', '#9333ea', 12, true),
    ('Wise', 'wise', 'WISE', 'GB', 'TRWIGB2L', '#10b981', 13, true),
    ('Revolut', 'revolut', 'REVO', 'LT', 'REVOLT21', '#0ea5e9', 14, true),
    ('Deutsche Bank', 'deutsche-bank', 'DEUT', 'DE', 'DEUTDEDD', '#1d4ed8', 15, true);

-- RBAC Permissions
INSERT INTO permissions (name, slug) VALUES
    ('Read platform banks', 'platform.banks.read'),
    ('Write platform banks', 'platform.banks.write'),
    ('Read bank accounts', 'app.bank_accounts.read'),
    ('Write bank accounts', 'app.bank_accounts.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('super_admin')
  AND p.slug IN ('platform.banks.read', 'platform.banks.write', 'app.bank_accounts.read', 'app.bank_accounts.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('subscriber')
  AND p.slug IN ('app.bank_accounts.read', 'app.bank_accounts.write');
