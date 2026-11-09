-- Remove direct scan counters from nfc_cards; Analytics Event Store is the single source of truth
ALTER TABLE nfc_cards DROP COLUMN IF EXISTS scan_count;
ALTER TABLE nfc_cards DROP COLUMN IF EXISTS last_scanned_at;

-- Ensure route.scan event definition is registered in analytics
INSERT INTO event_definitions (key, name, description, category, schema_version, enabled, is_public, aggregate_enabled, retention_days, required_properties)
VALUES
    ('route.scan', 'Route / Smart Link Scan', 'Visitor accessed a route or smart link', 'card', 1, true, true, true, 365, '{}')
ON CONFLICT (key) DO NOTHING;
