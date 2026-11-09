-- TEC-588: plan quotas enforced by the API. Limits are fail-closed: a plan without a key gets
-- nothing, so every existing plan receives a value here. -1 means unlimited.
INSERT INTO feature_definitions (key, value_type, label, description, sort_order) VALUES
    ('profiles.max',           'int', 'Maximum profiles',    'Profiles an account can create',                        30),
    ('smart_links.max',        'int', 'Maximum smart links', 'Custom smart links an account can create',              40),
    ('analytics.history_days', 'int', 'Analytics history',   'Days of analytics history an account can query (-1 = all)', 50)
ON CONFLICT (key) DO NOTHING;

UPDATE feature_definitions
SET label = 'Maximum active cards',
    description = 'NFC cards that can be active in an account at the same time'
WHERE key = 'cards.max';

INSERT INTO plan_limits (plan_id, feature_key, value_int)
SELECT p.id, v.key, CASE WHEN p.slug = 'free' THEN v.free_value ELSE v.paid_value END
FROM plans p
CROSS JOIN (VALUES
    ('profiles.max',           1,  10),
    ('smart_links.max',        3,  -1),
    ('analytics.history_days', 30, 365)
) AS v(key, free_value, paid_value)
ON CONFLICT (plan_id, feature_key) DO NOTHING;
