CREATE TABLE social_platforms (
    id            BIGSERIAL PRIMARY KEY,
    uuid          UUID         NOT NULL DEFAULT gen_random_uuid(),
    name          VARCHAR(100) NOT NULL,
    slug          VARCHAR(100) NOT NULL,
    category      VARCHAR(64)  NOT NULL DEFAULT 'social',
    url_pattern   TEXT         NOT NULL DEFAULT '',
    icon_url      TEXT         NOT NULL DEFAULT '',
    placeholder   VARCHAR(255) NOT NULL DEFAULT '',
    description   TEXT         NOT NULL DEFAULT '',
    display_order INT          NOT NULL DEFAULT 0,
    is_active     BOOLEAN      NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ  NULL,
    CONSTRAINT uq_social_platforms_uuid UNIQUE (uuid)
);

CREATE UNIQUE INDEX uq_social_platforms_slug_active
    ON social_platforms (slug)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_social_platforms_active
    ON social_platforms (is_active, display_order)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_social_platforms_set_updated_at
    BEFORE UPDATE ON social_platforms
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE profiles (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID         NOT NULL DEFAULT gen_random_uuid(),
    user_id         BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    primary_locale  VARCHAR(8)   NOT NULL DEFAULT 'tr',
    birthday        DATE         NULL,
    gender          VARCHAR(32)  NOT NULL DEFAULT '',
    avatar_url      TEXT         NOT NULL DEFAULT '',
    cover_url       TEXT         NOT NULL DEFAULT '',
    is_primary      BOOLEAN      NOT NULL DEFAULT false,
    completion_rate INT          NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ  NULL,
    CONSTRAINT uq_profiles_uuid UNIQUE (uuid),
    CONSTRAINT chk_profiles_primary_locale CHECK (primary_locale IN ('tr', 'en'))
);

CREATE INDEX idx_profiles_user_id ON profiles (user_id)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_profiles_set_updated_at
    BEFORE UPDATE ON profiles
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE profile_translations (
    id            BIGSERIAL PRIMARY KEY,
    profile_id    BIGINT       NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    locale        VARCHAR(8)   NOT NULL,
    prefix        VARCHAR(32)  NOT NULL DEFAULT '',
    first_name    VARCHAR(100) NOT NULL DEFAULT '',
    last_name     VARCHAR(100) NOT NULL DEFAULT '',
    middle_name   VARCHAR(100) NOT NULL DEFAULT '',
    suffix        VARCHAR(32)  NOT NULL DEFAULT '',
    nickname      VARCHAR(100) NOT NULL DEFAULT '',
    company_name  VARCHAR(255) NOT NULL DEFAULT '',
    department    VARCHAR(255) NOT NULL DEFAULT '',
    title         VARCHAR(255) NOT NULL DEFAULT '',
    role          VARCHAR(255) NOT NULL DEFAULT '',
    bio           TEXT         NOT NULL DEFAULT '',
    note          TEXT         NOT NULL DEFAULT '',
    is_default    BOOLEAN      NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_profile_translations_locale UNIQUE (profile_id, locale),
    CONSTRAINT chk_profile_translations_locale CHECK (locale IN ('tr', 'en'))
);

CREATE INDEX idx_profile_translations_profile_id ON profile_translations (profile_id);

CREATE TRIGGER trg_profile_translations_set_updated_at
    BEFORE UPDATE ON profile_translations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE profile_addresses (
    id            BIGSERIAL PRIMARY KEY,
    uuid          UUID         NOT NULL DEFAULT gen_random_uuid(),
    profile_id    BIGINT       NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    type          VARCHAR(32)  NOT NULL DEFAULT 'work',
    label         VARCHAR(100) NOT NULL DEFAULT '',
    street        TEXT         NOT NULL DEFAULT '',
    extended      TEXT         NOT NULL DEFAULT '',
    city          VARCHAR(100) NOT NULL DEFAULT '',
    region        VARCHAR(100) NOT NULL DEFAULT '',
    postal_code   VARCHAR(32)  NOT NULL DEFAULT '',
    country       VARCHAR(100) NOT NULL DEFAULT '',
    is_primary    BOOLEAN      NOT NULL DEFAULT false,
    display_order INT          NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_profile_addresses_uuid UNIQUE (uuid)
);

CREATE INDEX idx_profile_addresses_profile_id ON profile_addresses (profile_id);

CREATE TRIGGER trg_profile_addresses_set_updated_at
    BEFORE UPDATE ON profile_addresses
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE profile_contact_details (
    id            BIGSERIAL PRIMARY KEY,
    uuid          UUID         NOT NULL DEFAULT gen_random_uuid(),
    profile_id    BIGINT       NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    category      VARCHAR(32)  NOT NULL DEFAULT 'phone',
    type          VARCHAR(32)  NOT NULL DEFAULT 'work',
    label         VARCHAR(100) NOT NULL DEFAULT '',
    value         TEXT         NOT NULL,
    is_primary    BOOLEAN      NOT NULL DEFAULT false,
    display_order INT          NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_profile_contact_details_uuid UNIQUE (uuid),
    CONSTRAINT chk_profile_contact_details_category CHECK (category IN ('phone', 'email', 'url', 'whatsapp', 'other'))
);

CREATE INDEX idx_profile_contact_details_profile_id ON profile_contact_details (profile_id);

CREATE TRIGGER trg_profile_contact_details_set_updated_at
    BEFORE UPDATE ON profile_contact_details
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE profile_social_links (
    id            BIGSERIAL PRIMARY KEY,
    uuid          UUID         NOT NULL DEFAULT gen_random_uuid(),
    profile_id    BIGINT       NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    platform_id   BIGINT       NULL REFERENCES social_platforms(id) ON DELETE SET NULL,
    platform      VARCHAR(64)  NOT NULL DEFAULT 'custom',
    label         VARCHAR(100) NOT NULL DEFAULT '',
    url           TEXT         NOT NULL,
    icon_url      TEXT         NOT NULL DEFAULT '',
    display_order INT          NOT NULL DEFAULT 0,
    is_active     BOOLEAN      NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_profile_social_links_uuid UNIQUE (uuid)
);

CREATE INDEX idx_profile_social_links_profile_id ON profile_social_links (profile_id);

CREATE TRIGGER trg_profile_social_links_set_updated_at
    BEFORE UPDATE ON profile_social_links
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

INSERT INTO permissions (name, slug) VALUES
    ('Read profiles', 'app.profiles.read'),
    ('Write profiles', 'app.profiles.write'),
    ('Read social platforms', 'platform.social_platforms.read'),
    ('Write social platforms', 'platform.social_platforms.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('super_admin')
  AND p.slug IN ('app.profiles.read', 'app.profiles.write', 'platform.social_platforms.read', 'platform.social_platforms.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('subscriber')
  AND p.slug IN ('app.profiles.read', 'app.profiles.write');
