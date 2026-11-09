-- TEC-941: monthly quotas for lead forms, contact exchanges and lead exports. Each is an int
-- feature per plan: -1 unlimited, 0 off, N per calendar month (Europe/Istanbul). Every plan
-- defines every feature (TEC-942), so all existing plans get a value here.
INSERT INTO feature_definitions (key, value_type, label, description, sort_order) VALUES
    ('leads.monthly_max',        'int', 'Monthly leads',
     'Lead form submissions an account can see per calendar month. Over the quota they are kept but locked (-1 = unlimited, 0 = form off)', 60),
    ('exchange.monthly_max',     'int', 'Monthly contact exchanges',
     'Contact exchanges an account can see per calendar month. Over the quota they are kept but locked (-1 = unlimited, 0 = exchange off)', 70),
    ('leads.export.monthly_max', 'int', 'Monthly lead exports',
     'Lead exports an account can run per calendar month (-1 = unlimited, 0 = export off)', 80)
ON CONFLICT (key) DO NOTHING;

-- Free: 20 leads, 20 exchanges and 2 exports a month. Every other plan: 1000 / 1000 / 100.
INSERT INTO plan_limits (plan_id, feature_key, value_int)
SELECT p.id, v.key, CASE WHEN p.slug = 'free' THEN v.free_value ELSE v.paid_value END
FROM plans p
CROSS JOIN (VALUES
    ('leads.monthly_max',        20, 1000),
    ('exchange.monthly_max',     20, 1000),
    ('leads.export.monthly_max',  2,  100)
) AS v(key, free_value, paid_value)
ON CONFLICT (plan_id, feature_key) DO NOTHING;

-- Monthly usage per account and feature. period is the first day of the calendar month in
-- Europe/Istanbul; a new month starts a new row, which is the reset. used counts events (leads
-- received, exports run); unlocked counts the leads of the month the owner may see, so deleting
-- a visible lead does not free a slot for a locked one.
CREATE TABLE account_monthly_usage (
    account_id  BIGINT      NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    feature_key TEXT        NOT NULL REFERENCES feature_definitions(key) ON DELETE CASCADE,
    period      DATE        NOT NULL,
    used        INTEGER     NOT NULL DEFAULT 0,
    unlocked    INTEGER     NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, feature_key, period),
    CONSTRAINT chk_account_monthly_usage_counts CHECK (used >= 0 AND unlocked >= 0)
);

-- A lead or exchange received over the quota is kept but locked: the owner sees it with its
-- contact details masked until a plan upgrade or a new month unlocks it. Unlocking is one-way.
ALTER TABLE leads ADD COLUMN locked BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX idx_leads_account_locked ON leads (account_id, kind, created_at) WHERE locked;

INSERT INTO notification_templates (code, channel, language, subject, body, active) VALUES
    ('entitlements.quota_warning', 'inapp', 'tr', 'Aylık kotanız %{{percent}} doldu',
     'Bu ay {{used}}/{{limit}} {{feature}} kullandınız. Kota dolunca {{after}}', TRUE),
    ('entitlements.quota_warning', 'realtime', 'tr', 'Aylık kotanız %{{percent}} doldu',
     'Bu ay {{used}}/{{limit}} {{feature}} kullandınız.', TRUE),
    ('entitlements.quota_warning', 'email', 'tr', 'Aylık {{feature}} kotanız %{{percent}} doldu',
     'Merhaba,

{{account}} hesabı bu ay {{used}}/{{limit}} {{feature}} kullandı. Kota dolunca {{after}}

Kota {{resets}} tarihinde sıfırlanır. Daha yüksek kota için planınızı yükseltebilirsiniz.', TRUE),
    ('entitlements.quota_reached', 'inapp', 'tr', 'Aylık kotanız doldu',
     'Bu ay {{limit}} {{feature}} kotanızın tamamını kullandınız. {{after_reached}}', TRUE),
    ('entitlements.quota_reached', 'realtime', 'tr', 'Aylık kotanız doldu',
     'Bu ay {{limit}} {{feature}} kotanızın tamamını kullandınız.', TRUE),
    ('entitlements.quota_reached', 'email', 'tr', 'Aylık {{feature}} kotanız doldu',
     'Merhaba,

{{account}} hesabı bu ay {{limit}} {{feature}} kotasının tamamını kullandı. {{after_reached}}

Kota {{resets}} tarihinde sıfırlanır. Planınızı yükselttiğinizde kilitli kayıtlar hemen açılır.', TRUE),
    ('entitlements.quota_warning', 'inapp', 'en', '{{percent}}% of your monthly quota used',
     'You have used {{used}}/{{limit}} {{feature}} this month. Once the quota is used up, {{after}}', TRUE),
    ('entitlements.quota_warning', 'realtime', 'en', '{{percent}}% of your monthly quota used',
     'You have used {{used}}/{{limit}} {{feature}} this month.', TRUE),
    ('entitlements.quota_warning', 'email', 'en', '{{percent}}% of your monthly {{feature}} quota used',
     'Hello,

The {{account}} account has used {{used}}/{{limit}} {{feature}} this month. Once the quota is used up, {{after}}

The quota resets on {{resets}}. Upgrade your plan for a higher quota.', TRUE),
    ('entitlements.quota_reached', 'inapp', 'en', 'Monthly quota used up',
     'You have used all {{limit}} {{feature}} of your monthly quota. {{after_reached}}', TRUE),
    ('entitlements.quota_reached', 'realtime', 'en', 'Monthly quota used up',
     'You have used all {{limit}} {{feature}} of your monthly quota.', TRUE),
    ('entitlements.quota_reached', 'email', 'en', 'Monthly {{feature}} quota used up',
     'Hello,

The {{account}} account has used all {{limit}} {{feature}} of its monthly quota. {{after_reached}}

The quota resets on {{resets}}. Upgrading your plan unlocks locked records right away.', TRUE);
