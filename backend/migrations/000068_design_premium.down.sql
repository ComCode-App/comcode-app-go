ALTER TABLE backgrounds DROP COLUMN IF EXISTS thumbnail_key;
DELETE FROM plan_limits WHERE feature_key = 'design.premium';
DELETE FROM feature_definitions WHERE key = 'design.premium';
