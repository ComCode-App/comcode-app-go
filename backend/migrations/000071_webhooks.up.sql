-- TEC-603: outbound webhooks. An account registers HTTPS endpoints subscribed to events from the
-- webhook registry (lead.created, card.scanned, order.*). Every matching event becomes one
-- delivery row per endpoint; the worker POSTs it with an HMAC-SHA256 signature and retries with
-- backoff until it succeeds or the attempts run out. Deliveries are the audit log and can be
-- re-sent by hand. The signing secret is stored encrypted and shown to the user only once.
CREATE TYPE webhook_delivery_status AS ENUM ('pending', 'succeeded', 'failed', 'exhausted');

CREATE TABLE webhook_endpoints (
    id                   BIGSERIAL   PRIMARY KEY,
    uuid                 UUID        NOT NULL DEFAULT gen_random_uuid(),
    account_id           BIGINT      NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    url                  TEXT        NOT NULL,
    description          TEXT        NOT NULL DEFAULT '',
    secret_enc           TEXT        NOT NULL,
    -- Last characters of the secret so the user can tell which one is configured.
    secret_hint          VARCHAR(8)  NOT NULL DEFAULT '',
    events               TEXT[]      NOT NULL DEFAULT '{}',
    is_active            BOOLEAN     NOT NULL DEFAULT TRUE,
    consecutive_failures INT         NOT NULL DEFAULT 0,
    last_delivery_at     TIMESTAMPTZ NULL,
    last_success_at      TIMESTAMPTZ NULL,
    created_by_user_id   BIGINT      NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ NULL,
    CONSTRAINT uq_webhook_endpoints_uuid UNIQUE (uuid)
);

CREATE INDEX idx_webhook_endpoints_account ON webhook_endpoints (account_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_webhook_endpoints_set_updated_at
    BEFORE UPDATE ON webhook_endpoints
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE webhook_deliveries (
    id               BIGSERIAL               PRIMARY KEY,
    uuid             UUID                    NOT NULL DEFAULT gen_random_uuid(),
    endpoint_id      BIGINT                  NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    account_id       BIGINT                  NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    event_name       TEXT                    NOT NULL,
    -- One event id per domain event; every subscribed endpoint gets its own delivery.
    event_id         UUID                    NOT NULL,
    payload          JSONB                   NOT NULL,
    status           webhook_delivery_status NOT NULL DEFAULT 'pending',
    attempts         INT                     NOT NULL DEFAULT 0,
    max_attempts     INT                     NOT NULL DEFAULT 8,
    next_attempt_at  TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    last_status_code INT                     NULL,
    last_error       TEXT                    NOT NULL DEFAULT '',
    last_attempt_at  TIMESTAMPTZ             NULL,
    delivered_at     TIMESTAMPTZ             NULL,
    created_at       TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_webhook_deliveries_uuid UNIQUE (uuid)
);

CREATE INDEX idx_webhook_deliveries_endpoint ON webhook_deliveries (endpoint_id, created_at DESC);
CREATE INDEX idx_webhook_deliveries_account_created ON webhook_deliveries (account_id, created_at DESC);

CREATE TRIGGER trg_webhook_deliveries_set_updated_at
    BEFORE UPDATE ON webhook_deliveries
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- RBAC: any member reads, owners/admins write (checked in the usecase on top of the slug).
INSERT INTO permissions (name, slug) VALUES
    ('Read webhooks', 'app.webhooks.read'),
    ('Manage webhooks', 'app.webhooks.write');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('super_admin', 'subscriber')
  AND p.slug IN ('app.webhooks.read', 'app.webhooks.write')
ON CONFLICT DO NOTHING;
