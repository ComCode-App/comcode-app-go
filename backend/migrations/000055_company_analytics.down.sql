DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE slug = 'app.company_analytics.read');
DELETE FROM permissions WHERE slug = 'app.company_analytics.read';
