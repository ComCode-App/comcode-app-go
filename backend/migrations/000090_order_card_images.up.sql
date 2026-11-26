-- TEC-1320 (B): card design images chosen when the platform places an order. An item's own
-- image wins; otherwise the order's image applies; otherwise the default artwork. Provisioning
-- copies the result onto the card. Existing orders are not touched.
ALTER TABLE orders
    ADD COLUMN front_image_url TEXT NULL,
    ADD COLUMN back_image_url  TEXT NULL;
ALTER TABLE order_items
    ADD COLUMN front_image_url TEXT NULL,
    ADD COLUMN back_image_url  TEXT NULL;
