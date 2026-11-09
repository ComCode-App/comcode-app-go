-- Drops the stored DSNs (the API and worker go back to SENTRY_DSN only) and the permissions,
-- with every role assignment of them.
DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions
    WHERE slug IN ('platform.error_tracking.read', 'platform.error_tracking.write')
);
DELETE FROM permissions WHERE slug IN ('platform.error_tracking.read', 'platform.error_tracking.write');

DROP TABLE IF EXISTS error_tracking_settings;
