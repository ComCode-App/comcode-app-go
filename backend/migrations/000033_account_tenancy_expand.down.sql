-- Reverts the expand phase. Data loss: accounts, memberships and session
-- active accounts are dropped; legacy user_id/company_id ownership remains intact.

DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE slug IN ('platform.accounts.read', 'platform.accounts.write'));
DELETE FROM permissions WHERE slug IN ('platform.accounts.read', 'platform.accounts.write');

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

ALTER TABLE routes                     DROP COLUMN IF EXISTS account_id;
ALTER TABLE subscriptions              DROP COLUMN IF EXISTS account_id;
ALTER TABLE orders                     DROP COLUMN IF EXISTS account_id;
ALTER TABLE nfc_cards                  DROP COLUMN IF EXISTS account_id;
ALTER TABLE payment_provider_customers DROP COLUMN IF EXISTS account_id;
ALTER TABLE payment_card_sessions      DROP COLUMN IF EXISTS account_id;
ALTER TABLE payment_saved_cards        DROP COLUMN IF EXISTS account_id;
ALTER TABLE payment_checkout_sessions  DROP COLUMN IF EXISTS account_id;
ALTER TABLE user_bank_accounts         DROP COLUMN IF EXISTS account_id;
ALTER TABLE profiles                   DROP COLUMN IF EXISTS account_id;

DROP FUNCTION IF EXISTS company_account_id(BIGINT);
DROP FUNCTION IF EXISTS personal_account_id(BIGINT);

ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS active_account_id;

DROP TRIGGER IF EXISTS trg_users_create_personal_account ON users;
DROP FUNCTION IF EXISTS create_personal_account();

DROP TABLE IF EXISTS account_members;
DROP FUNCTION IF EXISTS guard_personal_account_members();
DROP TABLE IF EXISTS accounts;

DROP TYPE IF EXISTS account_role;
DROP TYPE IF EXISTS account_status;
DROP TYPE IF EXISTS account_type;
