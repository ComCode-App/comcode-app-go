-- TEC-1094: deploy provenance. CI bakes the release (v0.N on main, sha-<short> elsewhere, "dev"
-- locally), the commit, the build time and auto-generated notes into the backend image
-- (internal/platform/release). On every boot the server upserts one row per (version,
-- git_commit): first_seen_at is when that build first started against this database,
-- last_seen_at / boot_count move on each restart. Recording is fail-soft and never blocks boot.
CREATE TABLE app_releases (
    id            BIGSERIAL   PRIMARY KEY,
    version       TEXT        NOT NULL,
    git_commit    TEXT        NOT NULL DEFAULT '',
    built_at      TIMESTAMPTZ NULL,
    notes         TEXT        NOT NULL DEFAULT '',
    boot_count    BIGINT      NOT NULL DEFAULT 1,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_app_releases_version_commit UNIQUE (version, git_commit)
);

CREATE INDEX idx_app_releases_first_seen ON app_releases (first_seen_at DESC);
