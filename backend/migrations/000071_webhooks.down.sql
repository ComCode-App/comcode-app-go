DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE slug IN ('app.webhooks.read', 'app.webhooks.write'));
DELETE FROM permissions WHERE slug IN ('app.webhooks.read', 'app.webhooks.write');
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
DROP TYPE IF EXISTS webhook_delivery_status;
