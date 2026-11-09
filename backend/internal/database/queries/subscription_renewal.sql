-- Worker-only unscoped discovery. Manual/legacy subscriptions are deliberately excluded.
-- account-lint: cross-account renewal worker, all accounts
-- name: ListDuePaymentSubscriptions :many
SELECT id FROM subscriptions
WHERE source='payment' AND status IN ('active','trialing','past_due')
 AND next_billing_at <= sqlc.arg(now_at)
 AND (cancel_at_period_end OR grace_until <= sqlc.arg(now_at)
      OR (COALESCE(retry_at,next_billing_at) <= sqlc.arg(now_at)))
ORDER BY next_billing_at,id LIMIT 100;

-- account-lint: cross-account renewal worker / plan change: subscription id from a claim made in that flow
-- name: LockRenewalSubscription :one
SELECT * FROM subscriptions WHERE id=$1 AND source='payment' FOR UPDATE;

-- name: LockAccountPaymentSubscription :one
SELECT * FROM subscriptions WHERE account_id=$1 AND source='payment'
 AND status IN ('active','trialing','past_due') ORDER BY created_at DESC LIMIT 1 FOR UPDATE;

-- account-lint: cross-account renewal worker
-- name: SaveRenewalSubscription :exec
UPDATE subscriptions SET status=$2,plan_id=$3,next_billing_at=$4,ends_at=$5,canceled_at=$6,
 period_started_at=$7,cancel_at_period_end=$8,pending_plan_id=$9,grace_until=$10,retry_at=$11,renewal_attempts=$12,dunning_started_at=$13
WHERE id=$1 AND source='payment';

-- name: CreateSubscriptionPaymentAttempt :one
INSERT INTO subscription_payment_attempts(account_id,subscription_id,period_at,attempt,kind,plan_id,amount_minor,currency)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING *;

-- account-lint: cross-account renewal worker / plan change: attempt id from a claim made in that flow
-- name: FinishSubscriptionPaymentAttempt :exec
UPDATE subscription_payment_attempts SET status=$2,provider_ref=$3,failure_code=$4 WHERE id=$1 AND status='processing';

-- account-lint: cross-account subscription id from the account's own subscription (LockAccountPaymentSubscription) or the renewal worker
-- name: HasUnresolvedSubscriptionCharge :one
SELECT EXISTS(SELECT 1 FROM subscription_payment_attempts WHERE subscription_id=$1 AND status IN ('processing','unknown'));

-- name: RenewalCard :one
SELECT sc.*, u.email, u.name FROM payment_saved_cards sc JOIN users u ON u.id=sc.user_id
WHERE sc.account_id=$1 AND sc.provider=$2
 AND (sc.id=sqlc.narg(saved_card_id) OR (sqlc.narg(saved_card_id)::bigint IS NULL AND sc.is_default))
ORDER BY sc.is_default DESC LIMIT 1;

-- name: ListBillingRecipients :many
SELECT user_id FROM account_members WHERE account_id=$1 AND role IN ('owner','admin');

-- account-lint: cross-account subscription id from the account's own subscription (LockAccountPaymentSubscription)
-- name: NextUpgradeAttempt :one
SELECT (COALESCE(MAX(attempt),0)+1)::integer FROM subscription_payment_attempts
WHERE subscription_id=$1 AND period_at=$2 AND kind='upgrade';
