-- Product Categories
CREATE TABLE product_categories (
    id            BIGSERIAL PRIMARY KEY,
    uuid          UUID         NOT NULL DEFAULT gen_random_uuid(),
    name          VARCHAR(150) NOT NULL,
    slug          VARCHAR(120) NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    icon          VARCHAR(64)  NOT NULL DEFAULT '',
    image_url     TEXT         NOT NULL DEFAULT '',
    display_order INT          NOT NULL DEFAULT 0,
    is_active     BOOLEAN      NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ  NULL,
    CONSTRAINT uq_product_categories_uuid UNIQUE (uuid)
);

CREATE UNIQUE INDEX uq_product_categories_slug_active
    ON product_categories (slug)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_product_categories_active ON product_categories (is_active, display_order)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_product_categories_set_updated_at
    BEFORE UPDATE ON product_categories
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Products
CREATE TABLE products (
    id                 BIGSERIAL PRIMARY KEY,
    uuid               UUID          NOT NULL DEFAULT gen_random_uuid(),
    category_id        BIGINT        NULL REFERENCES product_categories(id) ON DELETE SET NULL,
    name               VARCHAR(200)  NOT NULL,
    slug               VARCHAR(150)  NOT NULL,
    short_description  TEXT          NOT NULL DEFAULT '',
    description        TEXT          NOT NULL DEFAULT '',
    status             VARCHAR(32)   NOT NULL DEFAULT 'draft',
    product_type       VARCHAR(32)   NOT NULL DEFAULT 'physical_card',
    base_price         NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    currency           VARCHAR(8)    NOT NULL DEFAULT 'TRY',
    tax_rate           NUMERIC(5,2)  NOT NULL DEFAULT 20.00,
    stock_quantity     INT           NOT NULL DEFAULT 0,
    sku                VARCHAR(64)   NOT NULL DEFAULT '',
    allow_custom_print BOOLEAN       NOT NULL DEFAULT true,
    model_3d_file_url  TEXT          NOT NULL DEFAULT '',
    model_3d_usdz_url  TEXT          NOT NULL DEFAULT '',
    model_3d_viewbox   JSONB         NOT NULL DEFAULT '{"camera_orbit": {"pitch": 15, "yaw": 25, "distance": 2.5}, "fov": 45, "auto_rotate": true, "rotation_speed": 1.5, "stage_bounds": {"radius": 3.0, "height": 2.0, "shadow_intensity": 0.4}}'::jsonb,
    images             JSONB         NOT NULL DEFAULT '[]'::jsonb,
    meta_title         VARCHAR(255)  NOT NULL DEFAULT '',
    meta_description   TEXT          NOT NULL DEFAULT '',
    meta_keywords      VARCHAR(255)  NOT NULL DEFAULT '',
    canonical_url      TEXT          NOT NULL DEFAULT '',
    og_image_url       TEXT          NOT NULL DEFAULT '',
    display_order      INT           NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ   NULL,
    CONSTRAINT uq_products_uuid UNIQUE (uuid)
);

CREATE UNIQUE INDEX uq_products_slug_active
    ON products (slug)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_products_category ON products (category_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_products_status ON products (status, display_order)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_products_set_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Product Variants
CREATE TABLE product_variants (
    id                BIGSERIAL PRIMARY KEY,
    uuid              UUID          NOT NULL DEFAULT gen_random_uuid(),
    product_id        BIGINT        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name              VARCHAR(150)  NOT NULL,
    sku               VARCHAR(64)   NOT NULL DEFAULT '',
    price_delta       NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    stock_quantity    INT           NOT NULL DEFAULT 0,
    material          VARCHAR(64)   NOT NULL DEFAULT 'matte_pvc',
    color_code        VARCHAR(32)   NOT NULL DEFAULT '#000000',
    image_url         TEXT          NOT NULL DEFAULT '',
    model_3d_file_url TEXT          NOT NULL DEFAULT '',
    model_3d_usdz_url TEXT          NOT NULL DEFAULT '',
    model_3d_viewbox  JSONB         NOT NULL DEFAULT '{}'::jsonb,
    is_active         BOOLEAN       NOT NULL DEFAULT true,
    display_order     INT           NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_product_variants_uuid UNIQUE (uuid)
);

CREATE INDEX idx_product_variants_product_id ON product_variants (product_id, is_active, display_order);

CREATE TRIGGER trg_product_variants_set_updated_at
    BEFORE UPDATE ON product_variants
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Orders
CREATE TABLE orders (
    id                BIGSERIAL PRIMARY KEY,
    uuid              UUID          NOT NULL DEFAULT gen_random_uuid(),
    order_number      VARCHAR(64)   NOT NULL,
    user_id           BIGINT        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    company_id        BIGINT        NULL REFERENCES companies(id) ON DELETE SET NULL,
    status            VARCHAR(32)   NOT NULL DEFAULT 'pending',
    subtotal          NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    tax_total         NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    shipping_total    NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    discount_total    NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    grand_total       NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    currency          VARCHAR(8)    NOT NULL DEFAULT 'TRY',
    payment_status    VARCHAR(32)   NOT NULL DEFAULT 'unpaid',
    payment_provider  VARCHAR(32)   NOT NULL DEFAULT '',
    payment_reference VARCHAR(128)  NOT NULL DEFAULT '',
    shipping_address  JSONB         NOT NULL DEFAULT '{}'::jsonb,
    billing_address   JSONB         NOT NULL DEFAULT '{}'::jsonb,
    shipping_carrier  VARCHAR(64)   NOT NULL DEFAULT '',
    tracking_number   VARCHAR(128)  NOT NULL DEFAULT '',
    tracking_url      TEXT          NOT NULL DEFAULT '',
    customer_note     TEXT          NOT NULL DEFAULT '',
    admin_note        TEXT          NOT NULL DEFAULT '',
    paid_at           TIMESTAMPTZ   NULL,
    shipped_at        TIMESTAMPTZ   NULL,
    delivered_at      TIMESTAMPTZ   NULL,
    cancelled_at      TIMESTAMPTZ   NULL,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ   NULL,
    CONSTRAINT uq_orders_uuid UNIQUE (uuid)
);

CREATE UNIQUE INDEX uq_orders_order_number_active
    ON orders (order_number)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_orders_user ON orders (user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_orders_status ON orders (status, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_orders_set_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Order Items (Supports Multi-quantity per order)
CREATE TABLE order_items (
    id                BIGSERIAL PRIMARY KEY,
    uuid              UUID          NOT NULL DEFAULT gen_random_uuid(),
    order_id          BIGINT        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id        BIGINT        NULL REFERENCES products(id) ON DELETE SET NULL,
    variant_id        BIGINT        NULL REFERENCES product_variants(id) ON DELETE SET NULL,
    product_name      VARCHAR(200)  NOT NULL,
    variant_name      VARCHAR(150)  NOT NULL DEFAULT '',
    sku               VARCHAR(64)   NOT NULL DEFAULT '',
    quantity          INT           NOT NULL DEFAULT 1,
    unit_price        NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    total_price       NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    custom_print_data JSONB         NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_order_items_uuid UNIQUE (uuid)
);

CREATE INDEX idx_order_items_order ON order_items (order_id);

CREATE TRIGGER trg_order_items_set_updated_at
    BEFORE UPDATE ON order_items
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Order Asset Requests (File & Logo Request Engine)
CREATE TABLE order_asset_requests (
    id                BIGSERIAL PRIMARY KEY,
    uuid              UUID         NOT NULL DEFAULT gen_random_uuid(),
    order_id          BIGINT       NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    request_type      VARCHAR(64)  NOT NULL DEFAULT 'company_logo',
    title             VARCHAR(200) NOT NULL,
    description       TEXT         NOT NULL DEFAULT '',
    status            VARCHAR(32)  NOT NULL DEFAULT 'requested',
    token             VARCHAR(64)  NOT NULL,
    uploaded_file_url TEXT         NOT NULL DEFAULT '',
    rejection_reason  TEXT         NOT NULL DEFAULT '',
    requested_by      BIGINT       NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_order_asset_requests_uuid UNIQUE (uuid),
    CONSTRAINT uq_order_asset_requests_token UNIQUE (token)
);

CREATE INDEX idx_order_asset_requests_order ON order_asset_requests (order_id, status);

CREATE TRIGGER trg_order_asset_requests_set_updated_at
    BEFORE UPDATE ON order_asset_requests
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Seed Categories
INSERT INTO product_categories (name, slug, description, icon, display_order, is_active) VALUES
    ('NFC Kartlar', 'nfc-cards', 'Standart ve premium akıllı dijital kartlar', 'CreditCard', 1, true),
    ('Metal Kartlar', 'metal-cards', 'Paslanmaz çelik ve mat siyah lüks metal kartlar', 'Shield', 2, true),
    ('Ahşap & Bambu', 'wood-bamboo-cards', 'Doğa dostu ahşap ve bambu akıllı kartlar', 'Trees', 3, true),
    ('NFC Stand & Rozet', 'nfc-stands-badges', 'Masaüstü NFC standları, etiketler ve akıllı rozetler', 'QrCode', 4, true);

-- Seed Initial Product (ComCode Metal NFC Card)
INSERT INTO products (
    category_id, name, slug, short_description, description, status, product_type,
    base_price, currency, tax_rate, stock_quantity, sku, allow_custom_print,
    meta_title, meta_description, meta_keywords
) VALUES (
    (SELECT id FROM product_categories WHERE slug = 'metal-cards' LIMIT 1),
    'ComCode Metal NFC Kart',
    'comcode-metal-nfc-kart',
    'Lazer işlemeli, paslanmaz çelik özel tasarım premium NFC kart.',
    'Yüksek kaliteli paslanmaz çelik gövde, çift yönlü NFC çipi ve lazer kazıma ile ömür boyu silinmez kurumsal veya kişisel baskı.',
    'active',
    'physical_card',
    1499.00,
    'TRY',
    20.00,
    150,
    'CC-MET-001',
    true,
    'ComCode Metal NFC Kart | Akıllı Dijital Kartvizit',
    'Paslanmaz çelik lazer kazıma ComCode Metal NFC kartvizit ile profesyonel temas kurun.',
    'metal nfc kart, dijital kartvizit, akıllı kart, temassız kart'
);

-- Seed Variant for ComCode Metal NFC Card
INSERT INTO product_variants (
    product_id, name, sku, price_delta, stock_quantity, material, color_code, display_order, is_active
) VALUES 
    ((SELECT id FROM products WHERE slug = 'comcode-metal-nfc-kart' LIMIT 1), 'Mat Siyah Metal', 'CC-MET-BLK', 0.00, 75, 'metal_black', '#1c1917', 1, true),
    ((SELECT id FROM products WHERE slug = 'comcode-metal-nfc-kart' LIMIT 1), 'Altın Sarısı Metal', 'CC-MET-GLD', 250.00, 45, 'metal_gold', '#eab308', 2, true),
    ((SELECT id FROM products WHERE slug = 'comcode-metal-nfc-kart' LIMIT 1), 'Gümüş Çelik Metal', 'CC-MET-SLV', 0.00, 30, 'metal_silver', '#94a3b8', 3, true);

-- RBAC Permissions
INSERT INTO permissions (name, slug) VALUES
    ('Read platform categories', 'platform.categories.read'),
    ('Write platform categories', 'platform.categories.write'),
    ('Read platform products', 'platform.products.read'),
    ('Write platform products', 'platform.products.write'),
    ('Read platform orders', 'platform.orders.read'),
    ('Write platform orders', 'platform.orders.write'),
    ('Read app orders', 'app.orders.read'),
    ('Write app orders', 'app.orders.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug IN (
      'platform.categories.read', 'platform.categories.write',
      'platform.products.read', 'platform.products.write',
      'platform.orders.read', 'platform.orders.write',
      'app.orders.read', 'app.orders.write'
  );

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'subscriber'
  AND p.slug IN ('app.orders.read', 'app.orders.write');
