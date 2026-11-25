-- TEC-596: bulk lock and unlock of the active account's cards (POST /v1/cards/bulk).
-- Granted to super_admin and to every role that may already lock a single card
-- (app.cards.write); no existing row is changed.
INSERT INTO permissions (name, slug) VALUES
    ('Bulk lock app cards', 'app.cards.bulk.lock'),
    ('Bulk unlock app cards', 'app.cards.bulk.unlock')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE (
        r.slug = 'super_admin'
        OR EXISTS (
            SELECT 1
            FROM role_permissions rp
            JOIN permissions w ON w.id = rp.permission_id
            WHERE rp.role_id = r.id AND w.slug = 'app.cards.write'
        )
    )
  AND p.slug IN ('app.cards.bulk.lock', 'app.cards.bulk.unlock')
ON CONFLICT DO NOTHING;
