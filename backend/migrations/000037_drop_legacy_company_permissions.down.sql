-- Restores the legacy slugs (granted to super_admin and subscriber as originally seeded);
-- custom role grants are not restored.
INSERT INTO permissions (name, slug) VALUES
    ('Read platform companies', 'platform.companies.read'),
    ('Write platform companies', 'platform.companies.write'),
    ('Read company customers', 'app.company.customers.read')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug = 'super_admin' AND p.slug IN ('platform.companies.read', 'platform.companies.write')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.slug = 'app.company.customers.read'
WHERE r.slug = 'subscriber'
ON CONFLICT DO NOTHING;
