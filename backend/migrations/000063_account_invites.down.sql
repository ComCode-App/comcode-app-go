-- Drops every invite, pending ones included. Profiles created for invited employees stay in
-- their company account, unlinked.
DELETE FROM notification_templates WHERE code = 'accounts.invite';
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE slug IN ('app.account_invites.read', 'app.account_invites.write'));
DELETE FROM permissions WHERE slug IN ('app.account_invites.read', 'app.account_invites.write');
DROP TABLE IF EXISTS account_invites;
DROP FUNCTION IF EXISTS guard_account_invites_company();
