-- Data-loss note: drops the card design image URLs (the objects stay in storage).
ALTER TABLE nfc_cards
    DROP COLUMN IF EXISTS back_image_url,
    DROP COLUMN IF EXISTS front_image_url;
