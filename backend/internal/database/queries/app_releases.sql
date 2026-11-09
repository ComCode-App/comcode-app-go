-- Deploy provenance (TEC-1094). One row per (version, git_commit); the server upserts it on
-- every boot (internal/platform/release.RecordBoot).

-- name: UpsertAppRelease :one
INSERT INTO app_releases (version, git_commit, built_at, notes)
VALUES (sqlc.arg(version), sqlc.arg(git_commit), sqlc.narg(built_at), sqlc.arg(notes))
ON CONFLICT (version, git_commit) DO UPDATE
SET last_seen_at = NOW(),
    boot_count   = app_releases.boot_count + 1,
    built_at     = COALESCE(EXCLUDED.built_at, app_releases.built_at),
    notes        = CASE WHEN EXCLUDED.notes <> '' THEN EXCLUDED.notes ELSE app_releases.notes END
RETURNING *;

-- name: ListAppReleases :many
SELECT * FROM app_releases
ORDER BY first_seen_at DESC, id DESC
LIMIT sqlc.arg(row_limit);
