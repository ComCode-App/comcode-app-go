-- Refuse rollback until holding imports are removed or assigned a real creator.
ALTER TABLE profiles ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE user_bank_accounts ALTER COLUMN user_id SET NOT NULL;
DROP TABLE legacy_sync_hit_ids;
DROP TABLE legacy_sync_users;
