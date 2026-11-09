-- Fails if an account-scoped row violates the legacy per-user uniqueness or subject rules.
ALTER TABLE payment_saved_cards DROP CONSTRAINT IF EXISTS uq_payment_saved_cards_account_provider_ref;
ALTER TABLE payment_saved_cards
    ADD CONSTRAINT uq_payment_saved_cards_provider_ref UNIQUE (user_id, provider, provider_card_ref);

ALTER TABLE payment_provider_customers DROP CONSTRAINT payment_provider_customers_pkey;
ALTER TABLE payment_provider_customers ADD CONSTRAINT payment_provider_customers_pkey PRIMARY KEY (user_id, provider);

ALTER TABLE subscriptions
    ADD CONSTRAINT chk_subscriptions_subject CHECK (
        (user_id IS NOT NULL AND company_id IS NULL)
        OR (user_id IS NULL AND company_id IS NOT NULL)
    );
