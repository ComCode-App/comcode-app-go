-- Company management moved to /v1/platform/accounts (TEC-617/618). Roles that could manage
-- companies keep equivalent access through platform.accounts.*; the legacy slugs are removed.
INSERT INTO role_permissions (role_id, permission_id)
SELECT DISTINCT rp.role_id, acc.id
FROM role_permissions rp
JOIN permissions old ON old.id = rp.permission_id
JOIN permissions acc ON acc.slug = REPLACE(old.slug, 'platform.companies.', 'platform.accounts.')
WHERE old.slug IN ('platform.companies.read', 'platform.companies.write')
ON CONFLICT DO NOTHING;

DELETE FROM permissions
WHERE slug IN ('platform.companies.read', 'platform.companies.write', 'app.company.customers.read');
