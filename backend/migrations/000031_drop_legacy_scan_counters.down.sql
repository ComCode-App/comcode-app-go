ALTER TABLE nfc_cards ADD COLUMN scan_count BIGINT NOT NULL DEFAULT 0;
ALTER TABLE nfc_cards ADD COLUMN last_scanned_at TIMESTAMPTZ NULL;

DELETE FROM event_definitions WHERE key = 'route.scan';
