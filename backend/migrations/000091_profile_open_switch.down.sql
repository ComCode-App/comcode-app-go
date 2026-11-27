-- Data-loss note: drops the open/closed switch and the company lock; every profile is open again.
DELETE FROM event_definitions WHERE key = 'profile.swipe';
ALTER TABLE account_branding DROP COLUMN IF EXISTS profiles_always_open;
ALTER TABLE profiles DROP COLUMN IF EXISTS is_open;
