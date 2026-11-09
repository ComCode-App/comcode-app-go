-- TEC-584: committed order milestones awaiting best-effort notification enqueueing.
-- No backfill: existing orders are notified only for subsequent transitions.
CREATE TABLE order_notification_events (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    snapshot JSONB NOT NULL,
    dispatched_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_order_notification_events_order_status UNIQUE (order_id, status)
);
CREATE INDEX idx_order_notification_events_pending
    ON order_notification_events (account_id, order_id, id) WHERE dispatched_at IS NULL;
CREATE UNIQUE INDEX uq_notifications_order_status_channel
    ON notifications (source_event, channel) WHERE source_event LIKE 'orders.status:%';

CREATE FUNCTION capture_order_notification_event() RETURNS TRIGGER AS $$
DECLARE
    milestones TEXT[] := ARRAY[]::TEXT[];
    milestone TEXT;
BEGIN
    IF TG_OP = 'INSERT' THEN
        milestones := array_append(milestones, NEW.status);
    ELSE
        IF NEW.status IS DISTINCT FROM OLD.status THEN
            milestones := array_append(milestones, NEW.status);
        END IF;
        -- Payment is independent of the main status; preserve paid/refund notifications.
        IF NEW.payment_status IS DISTINCT FROM OLD.payment_status
           AND NEW.payment_status IN ('paid', 'refunded') THEN
            milestones := array_append(milestones, NEW.payment_status);
        END IF;
    END IF;
    FOREACH milestone IN ARRAY milestones LOOP
        INSERT INTO order_notification_events (account_id, order_id, status, snapshot)
        VALUES (NEW.account_id, NEW.id, milestone, jsonb_build_object(
            'order_uuid', NEW.uuid, 'order_number', NEW.order_number,
            'shipping_carrier', NEW.shipping_carrier,
            'tracking_number', NEW.tracking_number, 'tracking_url', NEW.tracking_url
        )) ON CONFLICT (order_id, status) DO NOTHING;
    END LOOP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_orders_capture_notification
    AFTER INSERT OR UPDATE OF status, payment_status ON orders
    FOR EACH ROW EXECUTE FUNCTION capture_order_notification_event();

INSERT INTO notification_templates (code, channel, language, subject, body, active)
SELECT 'orders.status.' || s.status, c.channel, l.language,
       CASE l.language WHEN 'tr' THEN 'Sipariş {{order_number}}: {{status}}'
                       ELSE 'Order {{order_number}}: {{status}}' END,
       CASE l.language WHEN 'tr' THEN '{{customer_name}}, {{order_number}} numaralı siparişinizin durumu: {{status}}. {{tracking_info}} Siparişlerinizi görüntüleyin: {{order_url}}'
                       ELSE '{{customer_name}}, your order {{order_number}} is now {{status}}. {{tracking_info}} View your orders: {{order_url}}' END,
       TRUE
FROM (VALUES ('pending'), ('paid'), ('design_pending'), ('design_approved'),
             ('in_production'), ('ready_to_ship'), ('shipped'), ('delivered'),
             ('cancelled'), ('refunded')) AS s(status)
CROSS JOIN (VALUES ('email'), ('inapp')) AS c(channel)
CROSS JOIN (VALUES ('tr'), ('en')) AS l(language);
