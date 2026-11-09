-- Drops every captured lead (personal data); there is no way back after this.
DELETE FROM notification_templates WHERE code = 'leads.received';

DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE slug IN ('app.leads.read', 'app.leads.write'));

DELETE FROM permissions WHERE slug IN ('app.leads.read', 'app.leads.write');

DROP TABLE IF EXISTS leads;
