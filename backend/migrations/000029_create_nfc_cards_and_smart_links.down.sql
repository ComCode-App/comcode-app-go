-- Rollback NFC Cards and Smart Links
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE slug IN (
        'app.cards.read', 'app.cards.write',
        'platform.cards.read', 'platform.cards.write'
    )
);

DELETE FROM permissions WHERE slug IN (
    'app.cards.read', 'app.cards.write',
    'platform.cards.read', 'platform.cards.write'
);

DROP TRIGGER IF EXISTS trg_nfc_cards_set_updated_at ON nfc_cards;
DROP TABLE IF EXISTS nfc_cards CASCADE;
