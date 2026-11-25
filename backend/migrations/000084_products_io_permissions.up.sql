-- TEC-596: export and import of the platform product catalog
-- (POST /v1/platform/products/export, POST /v1/platform/products/import).
INSERT INTO permissions (name, slug) VALUES
    ('Export platform products', 'platform.products.export'),
    ('Import platform products', 'platform.products.import')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug IN ('platform.products.export', 'platform.products.import')
ON CONFLICT DO NOTHING;
