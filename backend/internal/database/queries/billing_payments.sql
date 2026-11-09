-- Payment history of an account (TEC-590): every successful charge, whichever table recorded it.
-- subscription_checkout: the first payment of a hosted checkout (payment_checkout_sessions).
-- subscription_renewal / subscription_upgrade: recurring and proration charges (attempts table).
-- order: an online card order payment that paid its order (metadata.order_result = applied; a
-- session whose money arrived for a closed or already paid order is a refund case, not a receipt).
-- order_manual: an order marked paid by hand (bank transfer) without an online payment.
-- account-lint: cross-account NOT EXISTS checks sessions of an order that is already account-scoped
-- name: ListAccountPaymentHistory :many
SELECT h.kind, h.uuid, h.provider, h.provider_ref, h.amount_minor, h.currency, h.paid_at,
       h.plan_name, h.billing_interval, h.order_number, h.order_uuid,
       COUNT(*) OVER()::bigint AS total
FROM (
    SELECT 'subscription_checkout'::text AS kind, cs.uuid, cs.provider::text AS provider,
           COALESCE(cs.provider_ref, '')::text AS provider_ref, cs.amount_minor::bigint AS amount_minor,
           cs.currency::text AS currency, COALESCE(cs.completed_at, cs.updated_at)::timestamptz AS paid_at,
           p.name::text AS plan_name, p.billing_interval::text AS billing_interval,
           ''::text AS order_number, NULL::uuid AS order_uuid
    FROM payment_checkout_sessions cs
    JOIN subscriptions s ON s.checkout_session_id = cs.id
    JOIN plans p ON p.id = cs.plan_id
    WHERE s.account_id = sqlc.arg(account_id) AND cs.status = 'completed'
  UNION ALL
    SELECT ('subscription_' || a.kind)::text, a.uuid, COALESCE(s.provider::text, '')::text,
           COALESCE(a.provider_ref, '')::text, a.amount_minor::bigint, a.currency::text, a.updated_at::timestamptz,
           p.name::text, p.billing_interval::text, ''::text, NULL::uuid
    FROM subscription_payment_attempts a
    JOIN subscriptions s ON s.id = a.subscription_id
    JOIN plans p ON p.id = a.plan_id
    WHERE a.account_id = sqlc.arg(account_id) AND a.status = 'succeeded'
  UNION ALL
    SELECT 'order'::text, ops.uuid, ops.provider::text, COALESCE(ops.provider_ref, '')::text,
           ops.amount_minor::bigint, ops.currency::text, COALESCE(ops.completed_at, ops.updated_at)::timestamptz,
           ''::text, ''::text, o.order_number::text, o.uuid
    FROM order_payment_sessions ops
    JOIN orders o ON o.id = ops.order_id
    WHERE ops.account_id = sqlc.arg(account_id) AND ops.status = 'completed'
      AND COALESCE(ops.metadata->>'order_result', 'applied') = 'applied'
  UNION ALL
    SELECT 'order_manual'::text, o.uuid, o.payment_provider::text, o.payment_reference::text,
           ROUND(o.grand_total * 100)::bigint, o.currency::text, o.paid_at::timestamptz,
           ''::text, ''::text, o.order_number::text, o.uuid
    FROM orders o
    WHERE o.account_id = sqlc.arg(account_id) AND o.deleted_at IS NULL AND o.paid_at IS NOT NULL
      AND o.payment_status IN ('paid', 'refunded')
      AND NOT EXISTS (
          SELECT 1 FROM order_payment_sessions x
          WHERE x.order_id = o.id AND x.status = 'completed'
            AND COALESCE(x.metadata->>'order_result', 'applied') = 'applied'
      )
) h
WHERE sqlc.narg(payment_uuid)::uuid IS NULL OR h.uuid = sqlc.narg(payment_uuid)::uuid
ORDER BY h.paid_at DESC, h.uuid
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
