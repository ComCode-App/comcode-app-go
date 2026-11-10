-- TEC-1186 (mobile B9): Expo push devices and their pending push tickets.
--
-- Devices are user-level, not account-level: notifications and notification_preferences target
-- users, so neither table is in account_tables.txt. Browser Web Push stays in push_subscriptions.
--
-- session_family_uuid is the refresh token family (refresh_tokens.family_uuid, the access
-- token's sid) the device registered from. Logout, session revocation (also refresh reuse
-- detection) and revoking all sessions set disabled_at on that family's devices in the same
-- statement as the refresh tokens (queries/auth_tokens.sql), so a signed-out or remotely
-- revoked phone gets no more pushes. Registering again from a live session re-enables the row.
-- A token registered by another user moves to the caller (the device changed hands).
CREATE TABLE push_devices (
    id                  BIGSERIAL    PRIMARY KEY,
    uuid                UUID         NOT NULL DEFAULT gen_random_uuid(),
    user_id             BIGINT       NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token               VARCHAR(255) NOT NULL,
    platform            TEXT         NOT NULL,
    locale              TEXT         NULL,
    app_version         TEXT         NULL,
    device_name         TEXT         NULL,
    session_family_uuid UUID         NULL,
    last_seen_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    disabled_at         TIMESTAMPTZ  NULL,
    disabled_reason     TEXT         NULL,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_push_devices_uuid UNIQUE (uuid),
    CONSTRAINT uq_push_devices_token UNIQUE (token),
    CONSTRAINT chk_push_devices_platform CHECK (platform IN ('ios', 'android')),
    CONSTRAINT chk_push_devices_locale CHECK (locale IN ('tr', 'en')),
    CONSTRAINT chk_push_devices_app_version CHECK (char_length(app_version) <= 32),
    CONSTRAINT chk_push_devices_device_name CHECK (char_length(device_name) <= 100),
    -- signed_out: logout; session_revoked: one session revoked (sessions list, refresh reuse,
    -- end of impersonation); all_sessions_revoked: password change / reset, sign out everywhere;
    -- device_not_registered: Expo receipt (B10).
    CONSTRAINT chk_push_devices_disabled_reason CHECK (
        disabled_reason IN ('signed_out', 'session_revoked', 'all_sessions_revoked', 'device_not_registered')
    ),
    CONSTRAINT chk_push_devices_disabled CHECK ((disabled_at IS NULL) = (disabled_reason IS NULL))
);

CREATE INDEX idx_push_devices_user ON push_devices (user_id);
CREATE INDEX idx_push_devices_family_active ON push_devices (session_family_uuid)
    WHERE disabled_at IS NULL;

CREATE TRIGGER trg_push_devices_set_updated_at
    BEFORE UPDATE ON push_devices
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Expo push tickets awaiting their receipt (B10 checks them about 15 minutes after sending;
-- DeviceNotRegistered disables the device). Rows are deleted once processed.
CREATE TABLE push_tickets (
    id              BIGSERIAL   PRIMARY KEY,
    ticket_id       VARCHAR(64) NOT NULL,
    push_device_id  BIGINT      NOT NULL REFERENCES push_devices (id) ON DELETE CASCADE,
    notification_id BIGINT      NULL REFERENCES notifications (id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_push_tickets_ticket_id UNIQUE (ticket_id)
);

CREATE INDEX idx_push_tickets_created_at ON push_tickets (created_at);
CREATE INDEX idx_push_tickets_device ON push_tickets (push_device_id);
