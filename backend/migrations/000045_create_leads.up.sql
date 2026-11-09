-- TEC-552: lead capture. Visitors of a public profile, card or smart link with
-- presentation_config.lead_capture.enabled (cards: nfc_cards.lead_capture_enabled) leave their
-- contact details. Leads belong to the account that owns the route. They hold personal data, so
-- they are hard-deleted, and they keep the KVKK consent evidence (timestamp + consent text version).
CREATE TABLE leads (
    id               BIGSERIAL    PRIMARY KEY,
    uuid             UUID         NOT NULL DEFAULT gen_random_uuid(),
    account_id       BIGINT       NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    route_id         BIGINT       NULL REFERENCES routes(id) ON DELETE SET NULL,
    profile_id       BIGINT       NULL REFERENCES profiles(id) ON DELETE SET NULL,
    card_id          BIGINT       NULL REFERENCES nfc_cards(id) ON DELETE SET NULL,
    source_type      VARCHAR(16)  NOT NULL,
    route_path       VARCHAR(128) NOT NULL,
    name             VARCHAR(150) NOT NULL,
    email            VARCHAR(254) NOT NULL DEFAULT '',
    phone            VARCHAR(32)  NOT NULL DEFAULT '',
    message          TEXT         NOT NULL DEFAULT '',
    locale           VARCHAR(8)   NOT NULL DEFAULT '',
    consent_at       TIMESTAMPTZ  NOT NULL,
    consent_version  VARCHAR(32)  NOT NULL,
    ip_address       INET         NULL,
    user_agent       TEXT         NOT NULL DEFAULT '',
    read_at          TIMESTAMPTZ  NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_leads_uuid UNIQUE (uuid),
    CONSTRAINT chk_leads_source_type CHECK (source_type IN ('profile', 'card', 'smart_link')),
    CONSTRAINT chk_leads_contact CHECK (email <> '' OR phone <> '')
);

CREATE INDEX idx_leads_account_created ON leads (account_id, created_at DESC);
CREATE INDEX idx_leads_account_unread ON leads (account_id) WHERE read_at IS NULL;

CREATE TRIGGER trg_leads_set_updated_at
    BEFORE UPDATE ON leads
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

INSERT INTO permissions (name, slug) VALUES
    ('Read leads', 'app.leads.read'),
    ('Write leads', 'app.leads.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('super_admin', 'subscriber')
  AND p.slug IN ('app.leads.read', 'app.leads.write');

INSERT INTO notification_templates (code, channel, language, subject, body, active) VALUES
    ('leads.received', 'inapp', 'en', 'New lead',
     '{{name}} left their contact details via {{source}}.', TRUE),
    ('leads.received', 'realtime', 'en', 'New lead',
     '{{name}} left their contact details.', TRUE),
    ('leads.received', 'inapp', 'tr', 'Yeni iletişim talebi',
     '{{name}}, {{source}} üzerinden iletişim bilgilerini bıraktı.', TRUE),
    ('leads.received', 'realtime', 'tr', 'Yeni iletişim talebi',
     '{{name}} iletişim bilgilerini bıraktı.', TRUE);
