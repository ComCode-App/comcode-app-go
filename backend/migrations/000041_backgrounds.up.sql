-- TEC-599: saved Background Studio designs. account_id NULL = platform gallery preset.
-- `document` is the studio's BackgroundDocument JSON, validated by the API (kind, shader type
-- whitelist, safe string values, size limit).
CREATE TABLE backgrounds (
    id          BIGSERIAL PRIMARY KEY,
    uuid        UUID         NOT NULL DEFAULT gen_random_uuid(),
    account_id  BIGINT       NULL REFERENCES accounts (id) ON DELETE CASCADE,
    name        VARCHAR(120) NOT NULL,
    kind        VARCHAR(16)  NOT NULL,
    document    JSONB        NOT NULL,
    created_by  BIGINT       NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_backgrounds_uuid UNIQUE (uuid),
    CONSTRAINT chk_backgrounds_kind CHECK (kind IN ('solid', 'gradient', 'video', 'shader'))
);

CREATE INDEX idx_backgrounds_account ON backgrounds (account_id, created_at DESC);

CREATE TRIGGER trg_backgrounds_set_updated_at
    BEFORE UPDATE ON backgrounds
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
