ALTER TABLE subscriptions DROP COLUMN IF EXISTS saved_card_id;

DROP TABLE IF EXISTS payment_card_sessions;
DROP TABLE IF EXISTS payment_saved_cards;
DROP TABLE IF EXISTS payment_provider_customers;

DROP TYPE IF EXISTS payment_card_session_status;
