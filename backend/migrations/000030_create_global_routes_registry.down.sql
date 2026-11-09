DROP TRIGGER IF EXISTS trg_routes_protect_immutable ON routes;
DROP FUNCTION IF EXISTS protect_immutable_routes();
DROP TRIGGER IF EXISTS trg_routes_set_updated_at ON routes;
DROP TABLE IF EXISTS routes CASCADE;

DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE slug IN (
        'app.routes.read', 'app.routes.write',
        'platform.routes.read', 'platform.routes.write'
    )
);

DELETE FROM permissions WHERE slug IN (
    'app.routes.read', 'app.routes.write',
    'platform.routes.read', 'platform.routes.write'
);
