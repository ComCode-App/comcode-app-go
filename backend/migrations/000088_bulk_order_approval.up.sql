-- TEC-1317: company bulk orders wait for platform approval before payment.
-- orders.status has no CHECK constraint, so the new 'awaiting_approval' status needs no schema
-- change. Existing orders are not touched.

-- Why a super admin rejected the order (shown to the customer).
ALTER TABLE orders ADD COLUMN rejection_reason TEXT NULL;

-- The draft profile on this item was created by the order (not an existing profile matched by
-- e-mail); rejecting the order deletes only these.
ALTER TABLE order_items ADD COLUMN profile_created_by_order BOOLEAN NOT NULL DEFAULT false;

INSERT INTO permissions (name, slug) VALUES
    ('Approve or reject platform orders', 'platform.orders.approve')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug = 'platform.orders.approve'
ON CONFLICT DO NOTHING;

INSERT INTO notification_templates (code, channel, language, subject, body, active)
SELECT 'orders.status.awaiting_approval', c.channel, l.language,
       CASE l.language WHEN 'tr' THEN 'Sipariş {{order_number}}: {{status}}'
                       ELSE 'Order {{order_number}}: {{status}}' END,
       CASE l.language WHEN 'tr' THEN '{{customer_name}}, {{order_number}} numaralı siparişiniz alındı ve onay bekliyor. Onaylandığında ödeme yapabilirsiniz: {{order_url}}'
                       ELSE '{{customer_name}}, your order {{order_number}} was received and is awaiting approval. You can pay once it is approved: {{order_url}}' END,
       TRUE
FROM (VALUES ('email'), ('inapp')) AS c(channel)
CROSS JOIN (VALUES ('tr'), ('en')) AS l(language)
ON CONFLICT DO NOTHING;
