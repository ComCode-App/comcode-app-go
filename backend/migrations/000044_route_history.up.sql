-- TEC-558: audit trail of public path changes (username renames, smart link renames). Card,
-- legacy and system paths are immutable and never appear here.
CREATE TABLE route_history (
    id                  BIGSERIAL    PRIMARY KEY,
    route_id            BIGINT       NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    route_type          VARCHAR(32)  NOT NULL,
    account_id          BIGINT       NULL REFERENCES accounts(id) ON DELETE SET NULL,
    old_path            VARCHAR(128) NOT NULL,
    new_path            VARCHAR(128) NOT NULL,
    changed_by_user_id  BIGINT       NULL REFERENCES users(id) ON DELETE SET NULL,
    reason              VARCHAR(32)  NOT NULL,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_route_history_reason CHECK (reason IN ('username_change', 'smart_link_rename'))
);

CREATE INDEX idx_route_history_old_path ON route_history (LOWER(old_path), created_at DESC);
CREATE INDEX idx_route_history_route ON route_history (route_id, created_at DESC);
