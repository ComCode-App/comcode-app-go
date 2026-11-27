-- TEC-1326: profile swipe. A person's open profiles (personal account + company member links)
-- are shown together on the public page; a closed profile is never shown. Defaults keep today's
-- behaviour: every profile open, no company lock.
ALTER TABLE profiles ADD COLUMN is_open BOOLEAN NOT NULL DEFAULT TRUE;

-- Company setting: members cannot close their company profile (it always counts as open).
ALTER TABLE account_branding ADD COLUMN profiles_always_open BOOLEAN NOT NULL DEFAULT FALSE;

-- Visitor moved between a person's open profiles.
INSERT INTO event_definitions (key, name, description, category, schema_version, enabled, is_public, aggregate_enabled, retention_days, required_properties)
VALUES
    ('profile.swipe', 'Profile Swipe', 'Visitor swiped to another open profile of the same person (properties: from)', 'card', 1, true, true, true, 365, '{}')
ON CONFLICT (key) DO NOTHING;
