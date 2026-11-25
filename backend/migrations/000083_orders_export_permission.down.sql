DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE slug = 'platform.orders.export');
DELETE FROM permissions WHERE slug = 'platform.orders.export';
