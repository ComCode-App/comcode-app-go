-- Account-based tenancy, contract phase (docs/TENANCY_AND_ACCESS.md). Every write path now sets
-- account_id, so the transitional fill triggers and the legacy company model are removed.
-- Remaining user_id columns mean the acting member (creator, placer, activator), not ownership.

DROP TRIGGER IF EXISTS trg_routes_fill_account_id ON routes;
DROP TRIGGER IF EXISTS trg_subscriptions_fill_account_id ON subscriptions;
DROP TRIGGER IF EXISTS trg_orders_fill_account_id ON orders;
DROP TRIGGER IF EXISTS trg_nfc_cards_fill_account_id ON nfc_cards;
DROP TRIGGER IF EXISTS trg_payment_provider_customers_fill_account_id ON payment_provider_customers;
DROP TRIGGER IF EXISTS trg_payment_card_sessions_fill_account_id ON payment_card_sessions;
DROP TRIGGER IF EXISTS trg_payment_saved_cards_fill_account_id ON payment_saved_cards;
DROP TRIGGER IF EXISTS trg_payment_checkout_sessions_fill_account_id ON payment_checkout_sessions;
DROP TRIGGER IF EXISTS trg_user_bank_accounts_fill_account_id ON user_bank_accounts;
DROP TRIGGER IF EXISTS trg_profiles_fill_account_id ON profiles;
DROP FUNCTION IF EXISTS fill_route_account_id();
DROP FUNCTION IF EXISTS fill_account_id_from_company_or_user();
DROP FUNCTION IF EXISTS fill_account_id_from_user();
DROP FUNCTION IF EXISTS company_account_id(BIGINT);
DROP FUNCTION IF EXISTS personal_account_id(BIGINT);

-- Ownership now lives on account_id.
ALTER TABLE nfc_cards     DROP COLUMN company_id;
ALTER TABLE orders        DROP COLUMN company_id;
ALTER TABLE routes        DROP COLUMN company_id;
ALTER TABLE subscriptions DROP COLUMN company_id;
ALTER TABLE subscriptions DROP COLUMN user_id;

-- Legacy company model.
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS uq_accounts_legacy_company;
ALTER TABLE accounts DROP COLUMN legacy_company_id;
DROP TABLE company_members;
DROP TABLE companies;
DROP TYPE company_member_role;
DROP TYPE company_status;
