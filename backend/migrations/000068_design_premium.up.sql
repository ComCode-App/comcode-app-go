-- TEC-947: premium visuals. Basic themes and static/color backgrounds are free; premium themes,
-- animated shaders and video backgrounds need a plan with design.premium. Every plan defines
-- every feature (TEC-942): free gets false, every other existing plan true.
INSERT INTO feature_definitions (key, value_type, label, description, sort_order) VALUES
    ('design.premium', 'bool', 'Premium design',
     'Premium public page themes and animated (shader or video) backgrounds. Without it the public page shows the basic theme and background; saved choices are kept', 90)
ON CONFLICT (key) DO NOTHING;

INSERT INTO plan_limits (plan_id, feature_key, value_bool)
SELECT p.id, 'design.premium', p.slug <> 'free'
FROM plans p
ON CONFLICT (plan_id, feature_key) DO NOTHING;

-- TEC-599: static thumbnail per saved design (object storage key), shown on devices that cannot
-- run the shader. NULL = not generated yet (video designs use their poster instead).
ALTER TABLE backgrounds ADD COLUMN thumbnail_key TEXT NULL;
