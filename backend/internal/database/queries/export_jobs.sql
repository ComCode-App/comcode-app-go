-- name: CreateExportJob :one
INSERT INTO export_jobs (resource, actor_id, format, query_json, locale, status, expires_at)
VALUES ($1, $2, $3, $4, $5, 'queued', $6)
RETURNING *;

-- name: GetPlatformExportJob :one
-- A platform export job of /v1/platform/exports: every resource except the app shell's
-- (app.*), which download through /v1/exports (TEC-1302).
SELECT * FROM export_jobs WHERE uuid = $1 AND resource NOT LIKE 'app.%';

-- name: GetAppExportJob :one
-- An app export job (app.*) of one account, for /v1/exports. export_jobs has no account
-- column: app exports carry the requesting account in query_json.account_id, set by the
-- server (a fixed query key the request cannot override).
SELECT * FROM export_jobs
WHERE uuid = sqlc.arg(uuid)
  AND resource LIKE 'app.%'
  AND query_json->>'account_id' = sqlc.arg(account_id)::text;

-- name: GetExportJobByID :one
SELECT * FROM export_jobs WHERE id = $1;

-- name: MarkExportJobProcessing :one
UPDATE export_jobs
SET status = 'processing'
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: MarkExportJobCompleted :one
UPDATE export_jobs
SET status = 'completed',
    file_key = $2,
    row_count = $3
WHERE id = $1
RETURNING *;

-- name: MarkExportJobFailed :one
UPDATE export_jobs
SET status = 'failed',
    error = $2
WHERE id = $1
RETURNING *;

-- name: ListExportJobsForActor :many
-- Platform /v1/platform/exports: the caller's own platform exports (no app.* jobs).
SELECT * FROM export_jobs
WHERE actor_id = $1 AND resource NOT LIKE 'app.%'
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountExportJobsForActor :one
SELECT COUNT(*)::bigint FROM export_jobs WHERE actor_id = $1 AND resource NOT LIKE 'app.%';

-- name: ListAllExportJobs :many
-- Platform /v1/platform/exports admin list: every platform export (no app.* jobs).
SELECT * FROM export_jobs
WHERE resource NOT LIKE 'app.%'
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountAllExportJobs :one
SELECT COUNT(*)::bigint FROM export_jobs WHERE resource NOT LIKE 'app.%';
