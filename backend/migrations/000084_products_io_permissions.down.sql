DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE slug IN ('platform.products.export', 'platform.products.import'));
DELETE FROM permissions WHERE slug IN ('platform.products.export', 'platform.products.import');
