-- TEC-950: platform staff can release a quarantined path before its 30-day hold ends. A deleted
-- smart link (TEC-559) and a renamed link's old path in route_history (TEC-558) stop holding the
-- path once this is set; the rows themselves are kept for the audit trail.
ALTER TABLE routes ADD COLUMN quarantine_released_at TIMESTAMPTZ NULL;
ALTER TABLE route_history ADD COLUMN quarantine_released_at TIMESTAMPTZ NULL;
