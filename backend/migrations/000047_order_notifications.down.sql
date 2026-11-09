-- Drops pending events and their deduplication history. Already sent notifications
-- remain; reapplying this migration cannot undo external delivery.
DROP TRIGGER trg_orders_capture_notification ON orders;
DROP FUNCTION capture_order_notification_event();
DROP TABLE order_notification_events;
DROP INDEX uq_notifications_order_status_channel;
DELETE FROM notification_templates WHERE code LIKE 'orders.status.%';
