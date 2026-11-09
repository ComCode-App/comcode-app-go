-- name: ListPendingOrderNotifications :many
SELECT e.*, customer.id AS customer_id, customer.name AS customer_name,
       customer.surname AS customer_surname, customer.locale AS customer_locale
FROM order_notification_events e
JOIN orders o ON o.id = e.order_id AND o.account_id = e.account_id AND o.deleted_at IS NULL
JOIN LATERAL (
    SELECT u.id, u.name, u.surname, u.locale
    FROM account_members m JOIN users u ON u.id = m.user_id
    WHERE m.account_id = e.account_id AND u.deleted_at IS NULL
      AND (m.user_id = o.user_id OR m.role = 'owner')
    ORDER BY (m.user_id = o.user_id) DESC, m.id ASC
    LIMIT 1
) customer ON TRUE
WHERE e.account_id = sqlc.arg(account_id) AND e.order_id = sqlc.arg(order_id)
  AND e.dispatched_at IS NULL
ORDER BY e.id;

-- name: MarkOrderNotificationDispatched :exec
UPDATE order_notification_events SET dispatched_at = NOW()
WHERE account_id = sqlc.arg(account_id) AND order_id = sqlc.arg(order_id)
  AND id = sqlc.arg(id) AND dispatched_at IS NULL;
