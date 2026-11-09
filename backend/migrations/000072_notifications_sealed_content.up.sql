-- TEC-1099: secrets in notification rows.
--
-- Security emails (verification / password reset codes) and account invitations used to
-- store the rendered code or invitation link in plaintext in notifications.title/body (and
-- the invitation link in action_url). The account owner could read their own verification
-- code through GET /v1/notifications, and platform staff could read codes and links in the
-- notification screens and the export.
--
-- From now on the row keeps a masked rendering (the secret replaced by '••••••') and the
-- real subject and body live AES-GCM encrypted (APP_ENCRYPTION_KEY) in sealed_content,
-- which only the delivery worker decrypts. A successful send clears the column.
ALTER TABLE notifications ADD COLUMN sealed_content TEXT NULL;

-- Scrub rows written before this migration. The key lives in the app, so unsent rows cannot
-- be sealed from SQL: they are cancelled (codes expire within minutes, the user can request
-- a new one; invitations can be re-sent). This scrub is irreversible.
UPDATE notifications
SET status = CASE
        WHEN status IN ('queued', 'processing', 'failed') THEN 'cancelled'
        ELSE status
    END,
    cancelled_at = CASE
        WHEN status IN ('queued', 'processing', 'failed') THEN COALESCE(cancelled_at, NOW())
        ELSE cancelled_at
    END,
    title = regexp_replace(title, '\m[0-9]{6}\M', '••••••', 'g'),
    body = regexp_replace(body, '\m[0-9]{6}\M', '••••••', 'g')
WHERE template_code IN ('auth.email_verification', 'auth.password_reset');

UPDATE notifications
SET status = CASE
        WHEN status IN ('queued', 'processing', 'failed') THEN 'cancelled'
        ELSE status
    END,
    cancelled_at = CASE
        WHEN status IN ('queued', 'processing', 'failed') THEN COALESCE(cancelled_at, NOW())
        ELSE cancelled_at
    END,
    body = regexp_replace(body, '/invite/[^[:space:]]+', '/invite/••••••', 'g'),
    action_url = NULL
WHERE template_code = 'accounts.invite';
