-- TEC-596: export of the platform order list (POST /v1/platform/orders/export).
INSERT INTO permissions (name, slug) VALUES
    ('Export platform orders', 'platform.orders.export')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug = 'platform.orders.export'
ON CONFLICT DO NOTHING;
