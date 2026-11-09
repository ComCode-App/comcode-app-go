-- TEC-574: company analytics (account totals and per-employee comparison).
-- No analytics schema change: the view sums the existing event_aggregates_daily rows of the
-- company's own profiles, routes and cards. Only the read permission is new.
INSERT INTO permissions (name, slug) VALUES
    ('Read company analytics', 'app.company_analytics.read')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('super_admin', 'subscriber')
  AND p.slug = 'app.company_analytics.read'
ON CONFLICT DO NOTHING;
