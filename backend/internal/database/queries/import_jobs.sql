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

-- name: GetPlatformImportJob :one
-- A platform import job (no account) of /v1/platform/imports; app jobs of accounts are not
-- found here (TEC-1302).
SELECT * FROM import_jobs WHERE uuid = $1 AND account_id IS NULL;

-- name: GetAccountImportJob :one
-- An app import job of one account and one resource: each app route family reaches only its
-- own resource's jobs (TEC-1302); the actor check stays in the usecase.
SELECT * FROM import_jobs
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id)
  AND resource = sqlc.arg(resource);

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

-- name: ListImportJobsForActor :many
-- Platform /v1/platform/imports: the caller's own platform jobs (no account).
SELECT * FROM import_jobs
WHERE actor_id = $1 AND account_id IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountImportJobsForActor :one
SELECT COUNT(*)::bigint FROM import_jobs WHERE actor_id = $1 AND account_id IS NULL;

-- name: ListAllImportJobs :many
-- Platform /v1/platform/imports admin list: every platform job (no account).
SELECT * FROM import_jobs
WHERE account_id IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountAllImportJobs :one
SELECT COUNT(*)::bigint FROM import_jobs WHERE account_id IS NULL;

-- name: InsertImportChange :one
INSERT INTO import_changes (job_id, entity_type, entity_uuid, op, previous_json)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListImportChangesForJob :many
SELECT * FROM import_changes
WHERE job_id = $1
ORDER BY id ASC;

-- name: ListImportJobsForAccount :many
-- App import history of one account, one resource and one actor.
SELECT * FROM import_jobs
WHERE account_id = sqlc.arg(account_id) AND actor_id = sqlc.arg(actor_id)
  AND resource = sqlc.arg(resource)
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountImportJobsForAccount :one
SELECT COUNT(*)::bigint FROM import_jobs
WHERE account_id = sqlc.arg(account_id) AND actor_id = sqlc.arg(actor_id)
  AND resource = sqlc.arg(resource);
