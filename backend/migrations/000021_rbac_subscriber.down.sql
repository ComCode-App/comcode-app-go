DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE slug IN (
        'platform.plans.read',
        'platform.plans.write',
        'platform.subscriptions.read',
        'platform.subscriptions.write',
        'platform.companies.read',
        'platform.companies.write',
        'app.company.customers.read'
    )
);

DELETE FROM permissions WHERE slug IN (
    'platform.plans.read',
    'platform.plans.write',
    'platform.subscriptions.read',
    'platform.subscriptions.write',
    'platform.companies.read',
    'platform.companies.write',
    'app.company.customers.read'
);

UPDATE roles
SET slug = 'cms_user',
    name = 'CMS User',
    description = 'Default CMS access for content users',
    is_system = false
WHERE slug = 'subscriber';
