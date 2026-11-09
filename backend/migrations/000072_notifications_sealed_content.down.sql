-- TEC-1099. Data-loss note: the up migration's scrub of codes and invitation links is
-- irreversible; this only removes the column. Undelivered sealed rows are cancelled first,
-- because the previous code would send their masked stand-in instead of the real content.
UPDATE notifications
SET status = 'cancelled',
    cancelled_at = COALESCE(cancelled_at, NOW())
WHERE sealed_content IS NOT NULL
  AND status IN ('queued', 'processing', 'failed');

ALTER TABLE notifications DROP COLUMN IF EXISTS sealed_content;
