-- NFC Cards and Smart Links
CREATE TABLE nfc_cards (
    id                   BIGSERIAL PRIMARY KEY,
    uuid                 UUID         NOT NULL DEFAULT gen_random_uuid(),
    user_id              BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id           BIGINT       NULL REFERENCES companies(id) ON DELETE SET NULL,
    name                 VARCHAR(150) NOT NULL,
    card_uid             VARCHAR(100) NULL,
    activation_code      VARCHAR(50)  NULL,
    smart_code           VARCHAR(32)  NOT NULL,
    target_type          VARCHAR(32)  NOT NULL DEFAULT 'profile',
    profile_id           BIGINT       NULL REFERENCES profiles(id) ON DELETE SET NULL,
    direct_url           TEXT         NOT NULL DEFAULT '',
    custom_data          JSONB        NOT NULL DEFAULT '{}'::jsonb,
    card_type            VARCHAR(50)  NOT NULL DEFAULT 'physical_nfc',
    material             VARCHAR(50)  NOT NULL DEFAULT 'matte_black',
    color_hex            VARCHAR(30)  NOT NULL DEFAULT '#18181b',
    is_active            BOOLEAN      NOT NULL DEFAULT true,
    is_locked            BOOLEAN      NOT NULL DEFAULT false,
    lead_capture_enabled BOOLEAN      NOT NULL DEFAULT false,
    passcode             VARCHAR(100) NULL,
    scan_count           BIGINT       NOT NULL DEFAULT 0,
    last_scanned_at      TIMESTAMPTZ  NULL,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ  NULL,
    CONSTRAINT uq_nfc_cards_uuid UNIQUE (uuid)
);

CREATE UNIQUE INDEX uq_nfc_cards_smart_code_active
    ON nfc_cards (smart_code)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uq_nfc_cards_activation_code_active
    ON nfc_cards (activation_code)
    WHERE deleted_at IS NULL AND activation_code IS NOT NULL AND activation_code <> '';

CREATE UNIQUE INDEX uq_nfc_cards_card_uid_active
    ON nfc_cards (card_uid)
    WHERE deleted_at IS NULL AND card_uid IS NOT NULL AND card_uid <> '';

CREATE INDEX idx_nfc_cards_user_active
    ON nfc_cards (user_id, is_active)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_nfc_cards_profile_id
    ON nfc_cards (profile_id)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_nfc_cards_set_updated_at
    BEFORE UPDATE ON nfc_cards
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- RBAC Permissions
INSERT INTO permissions (name, slug) VALUES
    ('Read app cards', 'app.cards.read'),
    ('Write app cards', 'app.cards.write'),
    ('Read platform cards', 'platform.cards.read'),
    ('Write platform cards', 'platform.cards.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug IN (
      'app.cards.read', 'app.cards.write',
      'platform.cards.read', 'platform.cards.write'
  );

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'subscriber'
  AND p.slug IN ('app.cards.read', 'app.cards.write');
