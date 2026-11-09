DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE slug IN ('platform.payment.read', 'platform.payment.write')
);

DELETE FROM permissions WHERE slug IN ('platform.payment.read', 'platform.payment.write');

ALTER TABLE subscriptions
    DROP COLUMN IF EXISTS canceled_at,
    DROP COLUMN IF EXISTS next_billing_at,
    DROP COLUMN IF EXISTS checkout_session_id,
    DROP COLUMN IF EXISTS provider_subscription_ref,
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS source;

DROP TABLE IF EXISTS payment_checkout_sessions;
DROP TABLE IF EXISTS payment_provider_settings;
DROP TABLE IF EXISTS payment_settings;

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS chk_plans_free_price,
    DROP COLUMN IF EXISTS trial_days,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS price_minor;

DROP TYPE IF EXISTS checkout_session_status;
DROP TYPE IF EXISTS subscription_source;
DROP TYPE IF EXISTS active_payment_provider;
DROP TYPE IF EXISTS payment_provider;
