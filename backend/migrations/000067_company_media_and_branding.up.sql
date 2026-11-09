-- TEC-946: company media library and company branding (account_branding).
--
-- company_media holds files a company account owns (logo, cover, corporate images, documents).
-- Bytes live in object storage under accounts/{account_uuid}/media/; this row is the metadata.
-- account_branding is the company look employee profiles inherit on their public page
-- (TEC-601): theme, layout, primary color, and a logo and cover picked from the library.
-- locked = employees always render the company look; otherwise it is only their default.
CREATE TABLE company_media (
    id                  BIGSERIAL    PRIMARY KEY,
    uuid                UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    account_id          BIGINT       NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    kind                TEXT         NOT NULL CHECK (kind IN ('logo', 'cover', 'image', 'document')),
    name                VARCHAR(255) NOT NULL CHECK (btrim(name) <> ''),
    object_key          TEXT         NOT NULL UNIQUE,
    content_type        VARCHAR(120) NOT NULL,
    size_bytes          BIGINT       NOT NULL CHECK (size_bytes > 0),
    uploaded_by_user_id BIGINT       NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (account_id, id)
);
CREATE INDEX idx_company_media_account ON company_media (account_id, created_at DESC);
CREATE TRIGGER trg_company_media_updated BEFORE UPDATE ON company_media
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE account_branding (
    account_id         BIGINT      PRIMARY KEY REFERENCES accounts (id) ON DELETE CASCADE,
    theme              TEXT        NOT NULL DEFAULT '',
    layout_mode        TEXT        NOT NULL DEFAULT '',
    primary_color      VARCHAR(7)  NOT NULL DEFAULT '' CHECK (primary_color = '' OR primary_color ~ '^#[0-9a-f]{6}$'),
    logo_media_id      BIGINT      NULL,
    cover_media_id     BIGINT      NULL,
    locked             BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_by_user_id BIGINT      NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Logo and cover come from the same account's library; deleting the file clears the pick.
    FOREIGN KEY (account_id, logo_media_id) REFERENCES company_media (account_id, id) ON DELETE SET NULL (logo_media_id),
    FOREIGN KEY (account_id, cover_media_id) REFERENCES company_media (account_id, id) ON DELETE SET NULL (cover_media_id)
);
CREATE TRIGGER trg_account_branding_updated BEFORE UPDATE ON account_branding
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Only company accounts have a media library and branding.
CREATE OR REPLACE FUNCTION guard_company_branding_account()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM accounts WHERE id = NEW.account_id AND type = 'company') THEN
        RAISE EXCEPTION 'company media and branding need a company account' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_company_media_company BEFORE INSERT ON company_media
    FOR EACH ROW EXECUTE FUNCTION guard_company_branding_account();
CREATE TRIGGER trg_account_branding_company BEFORE INSERT ON account_branding
    FOR EACH ROW EXECUTE FUNCTION guard_company_branding_account();

INSERT INTO permissions (name, slug) VALUES
    ('Read company branding and media', 'app.company_branding.read'),
    ('Manage company branding and media', 'app.company_branding.write')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('super_admin', 'subscriber')
  AND p.slug IN ('app.company_branding.read', 'app.company_branding.write')
ON CONFLICT DO NOTHING;
