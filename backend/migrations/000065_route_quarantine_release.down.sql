-- Released paths become quarantined again until their original 30 days pass.
ALTER TABLE route_history DROP COLUMN IF EXISTS quarantine_released_at;
ALTER TABLE routes DROP COLUMN IF EXISTS quarantine_released_at;
