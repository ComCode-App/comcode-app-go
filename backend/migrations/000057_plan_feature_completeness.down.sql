-- Restores nfc.enabled with the 000022 seed values (true on paid plans, false on free). The 0/false
-- rows backfilled by the up migration are kept: they behave like the missing rows they replaced.
INSERT INTO feature_definitions (key, value_type, label, description, sort_order) VALUES
    ('nfc.enabled', 'bool', 'NFC enabled', 'Whether NFC features are available', 20)
ON CONFLICT (key) DO NOTHING;

INSERT INTO plan_limits (plan_id, feature_key, value_bool)
SELECT p.id, 'nfc.enabled', p.billing_interval <> 'free'
FROM plans p
ON CONFLICT (plan_id, feature_key) DO NOTHING;
