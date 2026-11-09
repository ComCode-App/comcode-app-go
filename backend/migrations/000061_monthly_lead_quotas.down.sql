DELETE FROM notification_templates WHERE code IN ('entitlements.quota_warning', 'entitlements.quota_reached');
DROP INDEX IF EXISTS idx_leads_account_locked;
ALTER TABLE leads DROP COLUMN IF EXISTS locked;
DROP TABLE IF EXISTS account_monthly_usage;
DELETE FROM plan_limits WHERE feature_key IN ('leads.monthly_max', 'exchange.monthly_max', 'leads.export.monthly_max');
DELETE FROM feature_definitions WHERE key IN ('leads.monthly_max', 'exchange.monthly_max', 'leads.export.monthly_max');
