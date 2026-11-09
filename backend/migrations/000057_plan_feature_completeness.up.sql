-- TEC-942: nfc.enabled is removed (NFC cards work on every plan; nothing ever gated on it).
DELETE FROM plan_limits WHERE feature_key = 'nfc.enabled';
DELETE FROM feature_definitions WHERE key = 'nfc.enabled';

-- Every plan must now define every feature. A plan missing a feature already got nothing for it
-- (fail-closed), so the backfill writes the value that keeps that behaviour: 0 for int, false for
-- bool. A migration that adds a feature must likewise write a value for every existing plan.
INSERT INTO plan_limits (plan_id, feature_key, value_int, value_bool)
SELECT p.id, f.key,
       CASE WHEN f.value_type = 'int' THEN 0 END,
       CASE WHEN f.value_type = 'bool' THEN false END
FROM plans p
CROSS JOIN feature_definitions f
ON CONFLICT (plan_id, feature_key) DO NOTHING;
