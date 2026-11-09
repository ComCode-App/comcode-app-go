-- TEC-944: one row per order status transition, written by the orders module in the same
-- transaction as the status change. from_status is NULL for the creation row.
CREATE TABLE order_status_history (
    id                 BIGSERIAL PRIMARY KEY,
    account_id         BIGINT      NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    order_id           BIGINT      NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    from_status        TEXT        NULL,
    to_status          TEXT        NOT NULL,
    changed_by_user_id BIGINT      NULL REFERENCES users(id) ON DELETE SET NULL,
    changed_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_order_status_history_order ON order_status_history (order_id, changed_at);

-- The platform conversion funnel reads orders opened in a period.
CREATE INDEX idx_orders_created_at ON orders (created_at) WHERE deleted_at IS NULL;

-- Backfill: existing orders get a single row for their current status only (their earlier
-- transitions were never recorded). from_status stays NULL; the funnel treats such rows as
-- "reached" but leaves them out of the stage durations. changed_at uses the matching milestone
-- timestamp when the order has one.
INSERT INTO order_status_history (account_id, order_id, from_status, to_status, changed_at)
SELECT o.account_id, o.id, NULL, o.status,
       COALESCE(CASE o.status
                    WHEN 'pending'   THEN o.created_at
                    WHEN 'paid'      THEN o.paid_at
                    WHEN 'shipped'   THEN o.shipped_at
                    WHEN 'delivered' THEN o.delivered_at
                    WHEN 'cancelled' THEN o.cancelled_at
                END, o.updated_at)
FROM orders o;
