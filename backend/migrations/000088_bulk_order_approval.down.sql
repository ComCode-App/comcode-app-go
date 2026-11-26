-- Data-loss note: drops rejection reasons and the "created by order" flag of draft profiles.
-- Orders left in 'awaiting_approval' keep that status; move them by hand if needed.
DELETE FROM notification_templates WHERE code = 'orders.status.awaiting_approval';
DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE slug = 'platform.orders.approve');
DELETE FROM permissions WHERE slug = 'platform.orders.approve';
ALTER TABLE order_items DROP COLUMN IF EXISTS profile_created_by_order;
ALTER TABLE orders DROP COLUMN IF EXISTS rejection_reason;
