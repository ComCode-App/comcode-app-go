DROP INDEX IF EXISTS idx_nfc_cards_order_item_id;
DROP INDEX IF EXISTS idx_nfc_cards_order_id;
DROP INDEX IF EXISTS idx_order_items_provisioning_status;
DROP INDEX IF EXISTS idx_order_items_profile_id;
DROP INDEX IF EXISTS idx_order_items_card_id;
DROP INDEX IF EXISTS idx_orders_type_fulfillment;

ALTER TABLE nfc_cards
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS order_item_id,
    DROP COLUMN IF EXISTS order_id;

ALTER TABLE order_items
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN IF EXISTS provisioning_error,
    DROP COLUMN IF EXISTS provisioning_status,
    DROP COLUMN IF EXISTS fulfillment_status,
    DROP COLUMN IF EXISTS card_id,
    DROP COLUMN IF EXISTS profile_id,
    DROP COLUMN IF EXISTS recipient_department,
    DROP COLUMN IF EXISTS recipient_title,
    DROP COLUMN IF EXISTS recipient_phone,
    DROP COLUMN IF EXISTS recipient_email,
    DROP COLUMN IF EXISTS recipient_name;

ALTER TABLE orders
    DROP COLUMN IF EXISTS metadata,
    DROP COLUMN IF EXISTS fulfillment_status,
    DROP COLUMN IF EXISTS order_type;
