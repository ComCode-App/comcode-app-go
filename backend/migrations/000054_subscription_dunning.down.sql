-- Destructive: removes renewal payment audit history and pending billing changes.
DELETE FROM notification_templates WHERE code IN ('subscription.renewed','payment.succeeded','payment.failed','subscription.past_due','subscription.expired','subscription.canceled');
DROP INDEX uq_notifications_billing_event_channel;
DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE slug='app.billing.write');
DELETE FROM permissions WHERE slug='app.billing.write';
DROP TABLE subscription_payment_attempts;
DROP INDEX idx_subscriptions_due_payment;
ALTER TABLE subscriptions DROP COLUMN period_started_at, DROP COLUMN cancel_at_period_end,
 DROP COLUMN pending_plan_id, DROP COLUMN grace_until, DROP COLUMN dunning_started_at, DROP COLUMN retry_at, DROP COLUMN renewal_attempts;
