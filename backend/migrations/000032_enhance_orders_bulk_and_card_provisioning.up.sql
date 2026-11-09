-- Enhance Orders and Order Items for Bulk Operations, Recipient Details and Route/Card Provisioning
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS order_type VARCHAR(32) NOT NULL DEFAULT 'standard',
    ADD COLUMN IF NOT EXISTS fulfillment_status VARCHAR(32) NOT NULL DEFAULT 'unfulfilled',
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Enhance Order Items with Recipient, Card, Route, Profile and Provisioning State
ALTER TABLE order_items
    ADD COLUMN IF NOT EXISTS recipient_name VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS recipient_email VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS recipient_phone VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS recipient_title VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS recipient_department VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS profile_id BIGINT NULL REFERENCES profiles(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS card_id BIGINT NULL REFERENCES nfc_cards(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS fulfillment_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS provisioning_status VARCHAR(32) NOT NULL DEFAULT 'unprovisioned',
    ADD COLUMN IF NOT EXISTS provisioning_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(128) NOT NULL DEFAULT '';

-- Enhance NFC Cards with Order Association and Lifecycle Status
ALTER TABLE nfc_cards
    ADD COLUMN IF NOT EXISTS order_id BIGINT NULL REFERENCES orders(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS order_item_id BIGINT NULL REFERENCES order_items(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS status VARCHAR(32) NOT NULL DEFAULT 'active';

-- Indexes for Fast Lookups and Bulk Provisioning
CREATE INDEX IF NOT EXISTS idx_orders_type_fulfillment ON orders (order_type, fulfillment_status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_order_items_card_id ON order_items (card_id) WHERE card_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_order_items_profile_id ON order_items (profile_id) WHERE profile_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_order_items_provisioning_status ON order_items (order_id, provisioning_status);
CREATE INDEX IF NOT EXISTS idx_nfc_cards_order_id ON nfc_cards (order_id) WHERE order_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_nfc_cards_order_item_id ON nfc_cards (order_item_id) WHERE order_item_id IS NOT NULL;
