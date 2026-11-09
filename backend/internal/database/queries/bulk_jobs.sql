-- name: CreateBulkJob :one
INSERT INTO bulk_jobs (resource, action, actor_id, locale, status, target_json, account_id, params_json)
VALUES (sqlc.arg(resource), sqlc.arg(action), sqlc.arg(actor_id), sqlc.arg(locale), sqlc.arg(status),
        sqlc.arg(target_json), sqlc.narg(account_id), sqlc.arg(params_json))
RETURNING *;

-- account-lint: cross-account usecase checks JobAccess.allows (account + actor) or platform admin/actor
-- name: GetBulkJobByUUID :one
SELECT * FROM bulk_jobs WHERE uuid = $1;

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

-- account-lint: cross-account platform /v1/platform/bulk: the caller's own jobs (actor_id)
-- name: ListBulkJobsForActor :many
SELECT * FROM bulk_jobs
WHERE actor_id = $1
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- account-lint: cross-account platform /v1/platform/bulk: the caller's own jobs (actor_id)
-- name: CountBulkJobsForActor :one
SELECT COUNT(*)::bigint FROM bulk_jobs WHERE actor_id = $1;

-- account-lint: cross-account platform /v1/platform/bulk admin list
-- name: ListAllBulkJobs :many
SELECT * FROM bulk_jobs
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- account-lint: cross-account platform /v1/platform/bulk admin list
-- name: CountAllBulkJobs :one
SELECT COUNT(*)::bigint FROM bulk_jobs;

-- name: InsertBulkChange :one
INSERT INTO bulk_changes (job_id, entity_type, entity_uuid, op, previous_json)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListBulkChangesForJob :many
SELECT * FROM bulk_changes
WHERE job_id = $1
ORDER BY id ASC;

-- name: ListBulkJobsForAccount :many
-- App job history of one account; a non-nil actor_id limits it to that actor's jobs.
SELECT * FROM bulk_jobs
WHERE account_id = sqlc.arg(account_id)
  AND (sqlc.narg(actor_id)::bigint IS NULL OR actor_id = sqlc.narg(actor_id)::bigint)
  AND (sqlc.narg(resource)::text IS NULL OR resource = sqlc.narg(resource)::text)
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountBulkJobsForAccount :one
SELECT COUNT(*)::bigint FROM bulk_jobs
WHERE account_id = sqlc.arg(account_id)
  AND (sqlc.narg(actor_id)::bigint IS NULL OR actor_id = sqlc.narg(actor_id)::bigint)
  AND (sqlc.narg(resource)::text IS NULL OR resource = sqlc.narg(resource)::text);
