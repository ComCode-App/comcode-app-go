-- Legacy routes already moved to r/legacy/{hash} stay there; the alias still resolves.
CREATE OR REPLACE FUNCTION protect_immutable_routes()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.is_immutable THEN
        IF OLD.path <> NEW.path OR OLD.route_type <> NEW.route_type OR OLD.legacy_metadata <> NEW.legacy_metadata THEN
            RAISE EXCEPTION 'cannot modify immutable route identity or legacy metadata';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Data loss: drops the profile sector links set by the legacy sync.
DROP INDEX IF EXISTS idx_profiles_sector;
ALTER TABLE profiles DROP COLUMN IF EXISTS sector_id;
