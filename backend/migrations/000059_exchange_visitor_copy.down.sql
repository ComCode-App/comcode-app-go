DELETE FROM notifications WHERE source_event LIKE 'leads.exchange_copy:%';

DELETE FROM notification_templates WHERE code = 'leads.exchange_copy';

DROP INDEX IF EXISTS idx_notifications_exchange_copy;
