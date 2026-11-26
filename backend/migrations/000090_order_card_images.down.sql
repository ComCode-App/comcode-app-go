-- Data-loss note: drops the order design image URLs (the objects stay in storage).
ALTER TABLE order_items
    DROP COLUMN IF EXISTS back_image_url,
    DROP COLUMN IF EXISTS front_image_url;
ALTER TABLE orders
    DROP COLUMN IF EXISTS back_image_url,
    DROP COLUMN IF EXISTS front_image_url;
