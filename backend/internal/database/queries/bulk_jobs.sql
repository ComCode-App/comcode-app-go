-- name: CreateBulkJob :one
INSERT INTO bulk_jobs (resource, action, actor_id, locale, status, target_json, account_id, params_json)
VALUES (sqlc.arg(resource), sqlc.arg(action), sqlc.arg(actor_id), sqlc.arg(locale), sqlc.arg(status),
        sqlc.arg(target_json), sqlc.narg(account_id), sqlc.arg(params_json))
RETURNING *;

-- name: GetPlatformBulkJob :one
-- A platform job (no account) of /v1/platform/bulk; app jobs of accounts are not found here.
SELECT * FROM bulk_jobs WHERE uuid = $1 AND account_id IS NULL;

-- name: GetAccountBulkJob :one
-- An app job of one account and one resource: each app route family reaches only its own
-- resource's jobs (TEC-1300); the actor check stays in the usecase.
SELECT * FROM bulk_jobs
WHERE uuid = sqlc.arg(uuid)
  AND account_id = sqlc.arg(account_id)
  AND resource = sqlc.arg(resource);

-- account-lint: cross-account bulk worker: job id from the queue
-- name: GetBulkJobByID :one
SELECT * FROM bulk_jobs WHERE id = $1;

-- account-lint: cross-account bulk worker: job id from the queue
-- name: MarkBulkJobProcessing :one
UPDATE bulk_jobs
SET status = 'processing'
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- account-lint: cross-account bulk worker: job id from the queue
-- name: MarkBulkJobCompleted :one
UPDATE bulk_jobs
SET status = 'completed',
    applied_at = NOW(),
    rollback_until = sqlc.narg(rollback_until),
    result_json = $2
WHERE id = $1
RETURNING *;

-- account-lint: cross-account bulk worker: job id from the queue
-- name: MarkBulkJobFailed :one
UPDATE bulk_jobs
SET status = 'failed',
    error = $2
WHERE id = $1
RETURNING *;

-- account-lint: cross-account bulk rollback: job already checked by JobAccess.allows
-- name: MarkBulkJobRolledBack :one
UPDATE bulk_jobs
SET status = $2
WHERE id = $1 AND status = 'completed'
RETURNING *;

-- name: ListBulkJobsForActor :many
-- Platform /v1/platform/bulk: the caller's own platform jobs (no account).
SELECT * FROM bulk_jobs
WHERE actor_id = $1 AND account_id IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountBulkJobsForActor :one
SELECT COUNT(*)::bigint FROM bulk_jobs WHERE actor_id = $1 AND account_id IS NULL;

-- name: ListAllBulkJobs :many
-- Platform /v1/platform/bulk admin list: every platform job (no account).
SELECT * FROM bulk_jobs
WHERE account_id IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountAllBulkJobs :one
SELECT COUNT(*)::bigint FROM bulk_jobs WHERE account_id IS NULL;

-- name: InsertBulkChange :one
INSERT INTO bulk_changes (job_id, entity_type, entity_uuid, op, previous_json)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListBulkChangesForJob :many
SELECT * FROM bulk_changes
WHERE job_id = $1
ORDER BY id ASC;

-- name: ListBulkJobsForAccount :many
-- App job history of one account and one resource; a non-nil actor_id limits it to that
-- actor's jobs.
SELECT * FROM bulk_jobs
WHERE account_id = sqlc.arg(account_id)
  AND resource = sqlc.arg(resource)
  AND (sqlc.narg(actor_id)::bigint IS NULL OR actor_id = sqlc.narg(actor_id)::bigint)
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountBulkJobsForAccount :one
SELECT COUNT(*)::bigint FROM bulk_jobs
WHERE account_id = sqlc.arg(account_id)
  AND resource = sqlc.arg(resource)
  AND (sqlc.narg(actor_id)::bigint IS NULL OR actor_id = sqlc.narg(actor_id)::bigint);
