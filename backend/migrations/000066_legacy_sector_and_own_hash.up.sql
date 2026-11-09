-- TEC-955: the legacy sync links a profile to the sector its legacy company icon names
-- (teknoloji.png -> technology). Deleting a sector unlinks profiles instead of failing.
ALTER TABLE profiles ADD COLUMN sector_id BIGINT REFERENCES sectors(id) ON DELETE SET NULL;
CREATE INDEX idx_profiles_sector ON profiles(sector_id) WHERE sector_id IS NOT NULL;

-- TEC-956 option E: when a legacy profile's username is its own printed hash, the canonical
-- username takes the root path and the immutable legacy route moves to its collision alias
-- (r/legacy/{hash}), which already reserves the printed identity. That one move is the only
-- path change an immutable route may make; type and provenance stay frozen.
CREATE OR REPLACE FUNCTION protect_immutable_routes()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.is_immutable THEN
        IF (OLD.path <> NEW.path AND NOT (OLD.route_type = 'legacy_route' AND OLD.path NOT LIKE 'r/legacy/%'
                                         AND NEW.path = 'r/legacy/' || OLD.path))
           OR OLD.route_type <> NEW.route_type OR OLD.legacy_metadata <> NEW.legacy_metadata THEN
            RAISE EXCEPTION 'cannot modify immutable route identity or legacy metadata';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
