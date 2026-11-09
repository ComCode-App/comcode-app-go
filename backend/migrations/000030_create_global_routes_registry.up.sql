-- Global Routes Registry
CREATE TABLE routes (
    id                   BIGSERIAL PRIMARY KEY,
    uuid                 UUID         NOT NULL DEFAULT gen_random_uuid(),
    path                 VARCHAR(128) NOT NULL,
    route_type           VARCHAR(32)  NOT NULL,
    status               VARCHAR(32)  NOT NULL DEFAULT 'active',
    is_immutable         BOOLEAN      NOT NULL DEFAULT false,
    
    -- Target Association
    target_type          VARCHAR(32)  NOT NULL DEFAULT 'profile',
    target_id            UUID         NULL,
    profile_id           BIGINT       NULL REFERENCES profiles(id) ON DELETE SET NULL,
    card_id              BIGINT       NULL REFERENCES nfc_cards(id) ON DELETE SET NULL,
    direct_url           TEXT         NOT NULL DEFAULT '',
    
    -- Ownership
    owner_user_id        BIGINT       NULL REFERENCES users(id) ON DELETE SET NULL,
    company_id           BIGINT       NULL REFERENCES companies(id) ON DELETE SET NULL,
    
    -- Presentation & Legacy
    presentation_config  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    legacy_metadata      JSONB        NOT NULL DEFAULT '{}'::jsonb,
    
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ  NULL,
    
    CONSTRAINT uq_routes_uuid UNIQUE (uuid),
    CONSTRAINT chk_routes_route_type CHECK (route_type IN ('canonical_profile', 'custom_smart_link', 'card_route', 'legacy_route', 'system_reserved')),
    CONSTRAINT chk_routes_status CHECK (status IN ('reserved', 'active', 'locked', 'retired')),
    CONSTRAINT chk_routes_target_type CHECK (target_type IN ('profile', 'direct_url', 'vcard_download', 'social_hub', 'custom_modules'))
);

-- Case-insensitive Unique Index for global collision prevention
CREATE UNIQUE INDEX uq_routes_normalized_path_active
    ON routes (LOWER(TRIM(path)))
    WHERE deleted_at IS NULL;

CREATE INDEX idx_routes_lookup ON routes (LOWER(TRIM(path)), status) WHERE deleted_at IS NULL;
CREATE INDEX idx_routes_owner ON routes (owner_user_id, route_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_routes_card ON routes (card_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_routes_profile ON routes (profile_id) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_routes_set_updated_at
    BEFORE UPDATE ON routes
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Trigger to protect immutable routes and legacy metadata from unauthorized mutation
CREATE OR REPLACE FUNCTION protect_immutable_routes()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.is_immutable THEN
        IF OLD.path <> NEW.path OR OLD.route_type <> NEW.route_type OR OLD.legacy_metadata <> NEW.legacy_metadata THEN
            RAISE EXCEPTION 'cannot modify immutable route identity or legacy metadata';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_routes_protect_immutable
    BEFORE UPDATE ON routes
    FOR EACH ROW
    EXECUTE FUNCTION protect_immutable_routes();

-- Seed System Reserved Routes to prevent root collisions
INSERT INTO routes (path, route_type, status, is_immutable, target_type) VALUES
    ('platform', 'system_reserved', 'active', true, 'direct_url'),
    ('api', 'system_reserved', 'active', true, 'direct_url'),
    ('v1', 'system_reserved', 'active', true, 'direct_url'),
    ('v2', 'system_reserved', 'active', true, 'direct_url'),
    ('auth', 'system_reserved', 'active', true, 'direct_url'),
    ('login', 'system_reserved', 'active', true, 'direct_url'),
    ('register', 'system_reserved', 'active', true, 'direct_url'),
    ('logout', 'system_reserved', 'active', true, 'direct_url'),
    ('settings', 'system_reserved', 'active', true, 'direct_url'),
    ('billing', 'system_reserved', 'active', true, 'direct_url'),
    ('orders', 'system_reserved', 'active', true, 'direct_url'),
    ('cards', 'system_reserved', 'active', true, 'direct_url'),
    ('profiles', 'system_reserved', 'active', true, 'direct_url'),
    ('c', 'system_reserved', 'active', true, 'direct_url'),
    ('s', 'system_reserved', 'active', true, 'direct_url'),
    ('u', 'system_reserved', 'active', true, 'direct_url'),
    ('r', 'system_reserved', 'active', true, 'direct_url'),
    ('share', 'system_reserved', 'active', true, 'direct_url'),
    ('vcard', 'system_reserved', 'active', true, 'direct_url'),
    ('health', 'system_reserved', 'active', true, 'direct_url'),
    ('healthz', 'system_reserved', 'active', true, 'direct_url'),
    ('readyz', 'system_reserved', 'active', true, 'direct_url'),
    ('assets', 'system_reserved', 'active', true, 'direct_url'),
    ('static', 'system_reserved', 'active', true, 'direct_url'),
    ('favicon.ico', 'system_reserved', 'active', true, 'direct_url'),
    ('robots.txt', 'system_reserved', 'active', true, 'direct_url'),
    ('sitemap.xml', 'system_reserved', 'active', true, 'direct_url');

-- RBAC Permissions
INSERT INTO permissions (name, slug) VALUES
    ('Read app routes', 'app.routes.read'),
    ('Write app routes', 'app.routes.write'),
    ('Read platform routes', 'platform.routes.read'),
    ('Write platform routes', 'platform.routes.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug IN (
      'app.routes.read', 'app.routes.write',
      'platform.routes.read', 'platform.routes.write'
  );

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'subscriber'
  AND p.slug IN ('app.routes.read', 'app.routes.write');
