-- Billing belongs to accounts (docs/TENANCY_AND_ACCESS.md): a subscription's subject is its
-- account, and provider customers / saved cards are unique per account.

-- user_id / company_id remain only as legacy columns until the contract migration.
ALTER TABLE subscriptions DROP CONSTRAINT chk_subscriptions_subject;

-- One provider customer per account and provider (a user may pay for several accounts).
ALTER TABLE payment_provider_customers DROP CONSTRAINT payment_provider_customers_pkey;
ALTER TABLE payment_provider_customers ADD CONSTRAINT payment_provider_customers_pkey PRIMARY KEY (account_id, provider);

ALTER TABLE payment_saved_cards DROP CONSTRAINT uq_payment_saved_cards_provider_ref;
ALTER TABLE payment_saved_cards
    ADD CONSTRAINT uq_payment_saved_cards_account_provider_ref UNIQUE (account_id, provider, provider_card_ref);
