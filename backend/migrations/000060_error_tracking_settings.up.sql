-- TEC-952: error tracking DSNs are managed from the platform admin panel ("Hata izleme").
-- One row. DSNs are encrypted with ENCRYPTION_KEY (crypto.SecretBox). With no backend DSN
-- stored, the API and worker fall back to SENTRY_DSN; enabled = false turns both off.
CREATE TABLE error_tracking_settings (
    id                 INT PRIMARY KEY DEFAULT 1,
    enabled            BOOLEAN NOT NULL DEFAULT true,
    backend_dsn_enc    TEXT NULL,
    frontend_dsn_enc   TEXT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_error_tracking_settings_single_row CHECK (id = 1)
);

CREATE TRIGGER trg_error_tracking_settings_set_updated_at
    BEFORE UPDATE ON error_tracking_settings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

INSERT INTO error_tracking_settings (id) VALUES (1) ON CONFLICT DO NOTHING;

-- Super admin only, like the OAuth integration secrets.
INSERT INTO permissions (name, slug) VALUES
    ('Read error tracking settings', 'platform.error_tracking.read'),
    ('Manage error tracking settings', 'platform.error_tracking.write')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug IN ('platform.error_tracking.read', 'platform.error_tracking.write')
ON CONFLICT DO NOTHING;
