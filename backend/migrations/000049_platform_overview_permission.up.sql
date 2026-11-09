-- TEC-594: business KPI overview on the /platform home (GET /v1/platform/overview).
INSERT INTO permissions (name, slug) VALUES
    ('Read platform overview', 'platform.overview.read')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug = 'platform.overview.read'
ON CONFLICT DO NOTHING;
