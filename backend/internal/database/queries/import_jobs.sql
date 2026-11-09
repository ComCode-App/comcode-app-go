-- name: CreateImportJob :one
INSERT INTO import_jobs (resource, actor_id, format, locale, status, file_key, account_id)
VALUES (sqlc.arg(resource), sqlc.arg(actor_id), sqlc.arg(format), sqlc.arg(locale), 'uploaded',
        sqlc.arg(file_key), sqlc.narg(account_id))
RETURNING *;

-- account-lint: cross-account import upload: job id from the row just inserted for this account
-- name: UpdateImportJobFileKey :one
UPDATE import_jobs
SET file_key = $2
WHERE uuid = $1
RETURNING *;

-- account-lint: cross-account usecase getOwned checks account + actor; platform GetJob checks admin/actor
-- name: GetImportJobByUUID :one
SELECT * FROM import_jobs WHERE uuid = $1;

-- account-lint: cross-account import worker / Confirm: job id from an owned job (getOwned) or the queue
-- name: GetImportJobByID :one
SELECT * FROM import_jobs WHERE id = $1;

-- account-lint: cross-account import mapping: job already checked by getOwned
-- name: UpdateImportJobMapping :one
UPDATE import_jobs
SET mapping_json = $2,
    defaults_json = $3,
    status = 'mapped'
WHERE uuid = $1 AND status IN ('uploaded', 'mapped', 'previewed')
RETURNING *;

-- account-lint: cross-account import preview: job already checked by getOwned
-- name: UpdateImportJobPreview :one
UPDATE import_jobs
SET preview_json = $2,
    status = 'previewed'
WHERE uuid = $1 AND status IN ('mapped', 'previewed')
RETURNING *;

-- account-lint: cross-account import confirm: job already checked by getOwned
-- name: QueueImportJob :one
UPDATE import_jobs
SET status = 'queued'
WHERE uuid = $1 AND status = 'previewed'
RETURNING *;

-- account-lint: cross-account import worker: job id from the queue
-- name: MarkImportJobApplying :one
UPDATE import_jobs
SET status = 'applying'
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- account-lint: cross-account import worker: job id from the queue
-- name: MarkImportJobApplied :one
UPDATE import_jobs
SET status = 'applied',
    applied_at = NOW(),
    rollback_until = NOW() + INTERVAL '24 hours',
    preview_json = $2
WHERE id = $1
RETURNING *;

-- account-lint: cross-account import worker: job id from the queue
-- name: MarkImportJobFailed :one
UPDATE import_jobs
SET status = 'failed',
    error = $2
WHERE id = $1
RETURNING *;

-- account-lint: cross-account import rollback: job already checked by getOwned
-- name: MarkImportJobRolledBack :one
UPDATE import_jobs
SET status = 'rolled_back'
WHERE id = $1 AND status = 'applied'
RETURNING *;

-- account-lint: cross-account platform /v1/platform/imports: the caller's own jobs (actor_id)
-- name: ListImportJobsForActor :many
SELECT * FROM import_jobs
WHERE actor_id = $1
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- account-lint: cross-account platform /v1/platform/imports: the caller's own jobs (actor_id)
-- name: CountImportJobsForActor :one
SELECT COUNT(*)::bigint FROM import_jobs WHERE actor_id = $1;

-- account-lint: cross-account platform /v1/platform/imports admin list
-- name: ListAllImportJobs :many
SELECT * FROM import_jobs
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- account-lint: cross-account platform /v1/platform/imports admin list
-- name: CountAllImportJobs :one
SELECT COUNT(*)::bigint FROM import_jobs;

-- name: InsertImportChange :one
INSERT INTO import_changes (job_id, entity_type, entity_uuid, op, previous_json)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListImportChangesForJob :many
SELECT * FROM import_changes
WHERE job_id = $1
ORDER BY id ASC;

-- name: ListImportJobsForAccount :many
-- App import history of one account for one actor.
SELECT * FROM import_jobs
WHERE account_id = sqlc.arg(account_id) AND actor_id = sqlc.arg(actor_id)
  AND (sqlc.narg(resource)::text IS NULL OR resource = sqlc.narg(resource)::text)
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountImportJobsForAccount :one
SELECT COUNT(*)::bigint FROM import_jobs
WHERE account_id = sqlc.arg(account_id) AND actor_id = sqlc.arg(actor_id)
  AND (sqlc.narg(resource)::text IS NULL OR resource = sqlc.narg(resource)::text);
