-- Removes saved QR appearance; route identities and printed codes remain valid.
ALTER TABLE routes DROP COLUMN qr_style;
