ALTER TABLE subscriptions
    ADD COLUMN period_started_at timestamptz,
    ADD COLUMN cancel_at_period_end boolean NOT NULL DEFAULT false,
    ADD COLUMN pending_plan_id bigint REFERENCES plans(id),
    ADD COLUMN grace_until timestamptz,
    ADD COLUMN dunning_started_at timestamptz,
    ADD COLUMN retry_at timestamptz,
    ADD COLUMN renewal_attempts integer NOT NULL DEFAULT 0;
UPDATE subscriptions SET period_started_at = starts_at WHERE source = 'payment';
CREATE INDEX idx_subscriptions_due_payment ON subscriptions(next_billing_at)
    WHERE source = 'payment' AND status IN ('active', 'trialing', 'past_due');
CREATE TABLE subscription_payment_attempts (
    id bigserial PRIMARY KEY,
    uuid uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    account_id bigint NOT NULL REFERENCES accounts(id),
    subscription_id bigint NOT NULL REFERENCES subscriptions(id),
    period_at timestamptz NOT NULL,
    attempt integer NOT NULL,
    kind text NOT NULL CHECK (kind IN ('renewal', 'upgrade')),
    plan_id bigint NOT NULL REFERENCES plans(id),
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL,
    status text NOT NULL DEFAULT 'processing' CHECK (status IN ('processing','succeeded','failed','unknown')),
    provider_ref text,
    failure_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(subscription_id, period_at, kind, attempt)
);
CREATE TRIGGER trg_subscription_payment_attempts_updated BEFORE UPDATE ON subscription_payment_attempts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
INSERT INTO permissions(name,slug) VALUES ('Manage account billing','app.billing.write');
INSERT INTO role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug IN ('super_admin','subscriber') AND p.slug='app.billing.write' ON CONFLICT DO NOTHING;
CREATE UNIQUE INDEX uq_notifications_billing_event_channel ON notifications(source_event,channel)
    WHERE source_event LIKE 'billing:%';
INSERT INTO notification_templates(code,channel,language,subject,body,active)
SELECT e.code,c.channel,l.language,
 CASE WHEN l.language='tr' THEN e.tr ELSE e.en END,
 CASE WHEN l.language='tr' THEN e.tr || '. Aboneliğinizi faturalama sayfasından yönetin.'
 ELSE e.en || '. Manage your subscription on the billing page.' END,true
FROM (VALUES ('subscription.renewed','Abonelik yenilendi','Subscription renewed'),
 ('payment.succeeded','Ödeme alındı','Payment succeeded'),
 ('payment.failed','Ödeme alınamadı','Payment failed'),
 ('subscription.past_due','Abonelik ödemesi gecikti','Subscription payment overdue'),
 ('subscription.expired','Abonelik sona erdi','Subscription expired'),
 ('subscription.canceled','Abonelik iptal edildi','Subscription canceled')) e(code,tr,en)
CROSS JOIN (VALUES ('email'),('inapp')) c(channel)
CROSS JOIN (VALUES ('tr'),('en')) l(language);
