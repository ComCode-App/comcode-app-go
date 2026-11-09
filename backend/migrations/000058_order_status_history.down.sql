-- Drops the recorded order status transitions; they cannot be rebuilt from orders.
DROP INDEX IF EXISTS idx_orders_created_at;
DROP TABLE order_status_history;
