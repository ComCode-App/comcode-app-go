-- name: GetErrorTrackingSettings :one
SELECT * FROM error_tracking_settings
WHERE id = 1;

-- name: UpdateErrorTrackingSettings :one
UPDATE error_tracking_settings
SET enabled = sqlc.arg(enabled),
    backend_dsn_enc = sqlc.narg(backend_dsn_enc),
    frontend_dsn_enc = sqlc.narg(frontend_dsn_enc)
WHERE id = 1
RETURNING *;
