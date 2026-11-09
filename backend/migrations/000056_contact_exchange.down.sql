-- Drops every contact exchange (personal data) together with the columns only they use.
DELETE FROM notification_templates WHERE code = 'leads.exchange_received';

DELETE FROM leads WHERE kind = 'exchange';

ALTER TABLE leads
    DROP CONSTRAINT IF EXISTS chk_leads_kind,
    DROP COLUMN IF EXISTS sender_profile_id,
    DROP COLUMN IF EXISTS sender_user_id,
    DROP COLUMN IF EXISTS job_title,
    DROP COLUMN IF EXISTS company,
    DROP COLUMN IF EXISTS kind;
