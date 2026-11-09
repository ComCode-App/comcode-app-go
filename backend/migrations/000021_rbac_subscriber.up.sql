UPDATE roles
SET slug = 'subscriber',
    name = 'Subscriber',
    description = 'Default app access for end users',
    is_system = true
WHERE slug = 'cms_user';

INSERT INTO permissions (name, slug) VALUES
    ('Read platform plans', 'platform.plans.read'),
    ('Write platform plans', 'platform.plans.write'),
    ('Read platform subscriptions', 'platform.subscriptions.read'),
    ('Write platform subscriptions', 'platform.subscriptions.write'),
    ('Read platform companies', 'platform.companies.read'),
    ('Write platform companies', 'platform.companies.write'),
    ('Read company customers', 'app.company.customers.read');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug IN (
    'platform.plans.read',
    'platform.plans.write',
    'platform.subscriptions.read',
    'platform.subscriptions.write',
    'platform.companies.read',
    'platform.companies.write'
  );

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.slug = 'app.company.customers.read'
WHERE r.slug = 'subscriber';
