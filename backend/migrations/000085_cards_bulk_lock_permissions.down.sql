DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE slug IN ('app.cards.bulk.lock', 'app.cards.bulk.unlock'));
DELETE FROM permissions WHERE slug IN ('app.cards.bulk.lock', 'app.cards.bulk.unlock');
