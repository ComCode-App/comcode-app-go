-- TEC-1327 (part A): ComCode Business, the company's own link page at /{username}.
--
-- business_pages is one page per company account (draft or published) with page-level
-- translations ({"tr": {"name": .., "tagline": ..}, "en": {..}}) and a theme ('' = company
-- branding). business_page_blocks are its ordered, switchable blocks; config holds the block's
-- settings (button target, media picks, selected bank accounts, ...) and translations its texts.
-- The username is a 'business_page' row in the global routes registry, so it shares one
-- namespace with profile usernames, cards and smart links. Only new tables and new allowed
-- values: no existing row is changed.
CREATE TABLE business_pages (
    id                 BIGSERIAL   PRIMARY KEY,
    uuid               UUID        NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    account_id         BIGINT      NOT NULL UNIQUE REFERENCES accounts (id) ON DELETE CASCADE,
    status             TEXT        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    theme              TEXT        NOT NULL DEFAULT '',
    primary_locale     VARCHAR(8)  NOT NULL DEFAULT 'tr' CHECK (primary_locale IN ('tr', 'en')),
    translations       JSONB       NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(translations) = 'object'),
    published_at       TIMESTAMPTZ NULL,
    updated_by_user_id BIGINT      NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, id)
);
CREATE TRIGGER trg_business_pages_updated BEFORE UPDATE ON business_pages
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- Only company accounts have a business page (same guard as company media and branding).
CREATE TRIGGER trg_business_pages_company BEFORE INSERT ON business_pages
    FOR EACH ROW EXECUTE FUNCTION guard_company_branding_account();

CREATE TABLE business_page_blocks (
    id           BIGSERIAL   PRIMARY KEY,
    uuid         UUID        NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    account_id   BIGINT      NOT NULL,
    page_id      BIGINT      NOT NULL,
    type         TEXT        NOT NULL CHECK (type IN ('header', 'button', 'social', 'offices', 'team', 'bank_accounts')),
    position     INT         NOT NULL CHECK (position >= 0),
    enabled      BOOLEAN     NOT NULL DEFAULT TRUE,
    config       JSONB       NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(config) = 'object'),
    translations JSONB       NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(translations) = 'object'),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (account_id, page_id) REFERENCES business_pages (account_id, id) ON DELETE CASCADE,
    UNIQUE (page_id, position)
);
CREATE INDEX idx_business_page_blocks_account ON business_page_blocks (account_id);
CREATE TRIGGER trg_business_page_blocks_updated BEFORE UPDATE ON business_page_blocks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Route registry: a business page username is route_type 'business_page', target 'business_page'.
-- At most one live (active or locked) business page route per account.
ALTER TABLE routes DROP CONSTRAINT chk_routes_route_type;
ALTER TABLE routes ADD CONSTRAINT chk_routes_route_type CHECK (route_type IN (
    'canonical_profile', 'custom_smart_link', 'card_route', 'legacy_route', 'system_reserved', 'business_page'));
ALTER TABLE routes DROP CONSTRAINT chk_routes_target_type;
ALTER TABLE routes ADD CONSTRAINT chk_routes_target_type CHECK (target_type IN (
    'profile', 'direct_url', 'vcard_download', 'social_hub', 'custom_modules', 'business_page'));
CREATE UNIQUE INDEX uq_routes_business_page_live
    ON routes (account_id)
    WHERE route_type = 'business_page' AND status IN ('active', 'locked') AND deleted_at IS NULL;

INSERT INTO permissions (name, slug) VALUES
    ('Read the business page', 'app.business_page.read'),
    ('Manage the business page', 'app.business_page.write')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('super_admin', 'subscriber')
  AND p.slug IN ('app.business_page.read', 'app.business_page.write')
ON CONFLICT DO NOTHING;

-- Plan feature: on for every existing plan (business pages are company-only already); the
-- platform can turn it off per plan.
INSERT INTO feature_definitions (key, value_type, label, description, sort_order) VALUES
    ('business_page.enabled', 'bool', 'Business page',
     'Company business (link) page at /{username} with blocks, buttons and a lead form', 100)
ON CONFLICT (key) DO NOTHING;

INSERT INTO plan_limits (plan_id, feature_key, value_bool)
SELECT p.id, 'business_page.enabled', true
FROM plans p
ON CONFLICT (plan_id, feature_key) DO NOTHING;
