-- TEC-548: public profile pages at /{username}.
-- A username is a canonical_profile row in the global routes registry, so it shares one
-- namespace (and one uniqueness rule) with card routes and smart links. Renaming retires the
-- old row instead of deleting it: the old path keeps redirecting and cannot be taken by others.

-- At most one live (active or locked) canonical route per profile.
CREATE UNIQUE INDEX uq_routes_canonical_profile_live
    ON routes (profile_id)
    WHERE route_type = 'canonical_profile' AND status IN ('active', 'locked') AND deleted_at IS NULL;

-- Bank accounts the owner chose to show on a profile's public page (none by default).
CREATE TABLE profile_bank_accounts (
    profile_id      BIGINT NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    bank_account_id BIGINT NOT NULL REFERENCES user_bank_accounts (id) ON DELETE CASCADE,
    display_order   INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (profile_id, bank_account_id)
);
CREATE INDEX idx_profile_bank_accounts_bank_account ON profile_bank_accounts (bank_account_id);

-- Public interaction events (POST /v1/events).
INSERT INTO event_definitions (key, name, description, category, schema_version, enabled, is_public, aggregate_enabled, retention_days, required_properties)
VALUES
    ('contact.whatsapp.click', 'WhatsApp Click', 'Visitor clicked a WhatsApp contact', 'contact', 1, true, true, true, 365, '{}'),
    ('contact.address.click', 'Address Click', 'Visitor opened an address in maps', 'contact', 1, true, true, true, 365, '{}'),
    ('social.link.click', 'Social Link Click', 'Visitor clicked a social link (property: platform)', 'social', 1, true, true, true, 365, '{}'),
    ('bank.iban.copy', 'IBAN Copy', 'Visitor copied an IBAN', 'contact', 1, true, true, true, 365, '{}')
ON CONFLICT (key) DO NOTHING;

-- Give every existing profile a username derived from its default translation.
DO $$
DECLARE
    rec       RECORD;
    base      TEXT;
    candidate TEXT;
    n         INT;
    reserved  TEXT[] := ARRAY[
        'api','v1','v2','internal','system','admin','health','healthz','readyz','assets','static','images',
        '_next','favicon.ico','robots.txt','sitemap.xml','manifest.json','apple-icon','icon','icon.svg',
        'c','s','u','r','p','share','vcard','download','platform','login','register','auth','logout',
        'verify-email','reset-password','forgot-password','settings','billing','orders','cards','profiles',
        'profile','account','accounts','background-studio','bank-accounts','forbidden','unauthorized','help',
        'support','pricing','about','terms','privacy','kvkk','contact','blog','comcode','www','mail','root',
        'null','undefined'];
BEGIN
    FOR rec IN
        SELECT p.id, p.uuid, p.account_id, p.user_id,
               COALESCE(NULLIF(trim(pt.first_name || ' ' || pt.last_name), ''), pt.company_name, '') AS display_name
        FROM profiles p
        LEFT JOIN LATERAL (
            SELECT t.first_name, t.last_name, t.company_name
            FROM profile_translations t
            WHERE t.profile_id = p.id
            ORDER BY t.is_default DESC, (t.locale = p.primary_locale) DESC
            LIMIT 1
        ) pt ON true
        WHERE p.deleted_at IS NULL
          AND NOT EXISTS (
              SELECT 1 FROM routes r
              WHERE r.profile_id = p.id AND r.route_type = 'canonical_profile' AND r.deleted_at IS NULL)
        ORDER BY p.created_at
    LOOP
        base := lower(translate(rec.display_name, 'çÇğĞıIİöÖşŞüÜâÂîÎûÛ', 'ccggiiioossuuaaiiuu'));
        base := trim(BOTH '-' FROM regexp_replace(base, '[^a-z0-9]+', '-', 'g'));
        base := trim(BOTH '-' FROM left(base, 36));
        IF length(base) < 3 THEN
            base := 'p-' || left(replace(rec.uuid::text, '-', ''), 8);
        END IF;

        candidate := base;
        n := 1;
        WHILE candidate = ANY (reserved)
           OR EXISTS (SELECT 1 FROM routes r WHERE lower(trim(r.path)) = candidate AND r.deleted_at IS NULL)
        LOOP
            n := n + 1;
            candidate := base || '-' || n;
        END LOOP;

        INSERT INTO routes (path, route_type, status, target_type, profile_id, account_id, owner_user_id, presentation_config)
        VALUES (candidate, 'canonical_profile', 'active', 'profile', rec.id, rec.account_id, rec.user_id,
                '{"visible_modules": ["bio", "contacts", "social_links", "addresses", "bank_accounts"]}'::jsonb);
    END LOOP;
END $$;

-- Cards created in the app never got a c/{smart_code} route, so their QR/NFC links were 404.
-- Give every card without one an active route (provisioned cards already have a reserved one).
INSERT INTO routes (path, route_type, status, is_immutable, target_type, profile_id, card_id, account_id, owner_user_id)
SELECT 'c/' || c.smart_code, 'card_route', 'active', true, 'profile', c.profile_id, c.id, c.account_id, c.user_id
FROM nfc_cards c
WHERE c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM routes r WHERE r.card_id = c.id AND r.route_type = 'card_route' AND r.deleted_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM routes r WHERE lower(trim(r.path)) = lower('c/' || c.smart_code) AND r.deleted_at IS NULL);
