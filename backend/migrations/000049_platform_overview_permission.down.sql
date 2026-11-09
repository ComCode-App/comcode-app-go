-- Removes the permission and every role assignment of it.
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE slug = 'platform.overview.read');

DELETE FROM permissions WHERE slug = 'platform.overview.read';
