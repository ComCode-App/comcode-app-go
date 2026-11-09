-- Drops the library metadata and branding. Objects under accounts/{uuid}/media/ stay in
-- storage; profiles that reference one keep the URL.
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE slug IN ('app.company_branding.read', 'app.company_branding.write'));
DELETE FROM permissions WHERE slug IN ('app.company_branding.read', 'app.company_branding.write');
DROP TABLE IF EXISTS account_branding;
DROP TABLE IF EXISTS company_media;
DROP FUNCTION IF EXISTS guard_company_branding_account();
