-- TEC-1320 (A): per-card front and back design images. NULL = the ComCode default artwork.
-- Existing cards are not touched.
ALTER TABLE nfc_cards
    ADD COLUMN front_image_url TEXT NULL,
    ADD COLUMN back_image_url  TEXT NULL;
