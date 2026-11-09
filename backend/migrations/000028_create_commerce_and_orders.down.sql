DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE slug IN (
        'platform.categories.read', 'platform.categories.write',
        'platform.products.read', 'platform.products.write',
        'platform.orders.read', 'platform.orders.write',
        'app.orders.read', 'app.orders.write'
    )
);

DELETE FROM permissions WHERE slug IN (
    'platform.categories.read', 'platform.categories.write',
    'platform.products.read', 'platform.products.write',
    'platform.orders.read', 'platform.orders.write',
    'app.orders.read', 'app.orders.write'
);

DROP TABLE IF EXISTS order_asset_requests CASCADE;
DROP TABLE IF EXISTS order_items CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS product_variants CASCADE;
DROP TABLE IF EXISTS products CASCADE;
DROP TABLE IF EXISTS product_categories CASCADE;
