-- TEC-582: a card's printed/encoded identity never changes. smart_code is in the /c/{code} URL
-- written to the NFC chip and the QR; card_uid is the chip serial once recorded. Routes are
-- already guarded by protect_immutable_routes (000030).
CREATE OR REPLACE FUNCTION protect_card_identity()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.smart_code IS DISTINCT FROM NEW.smart_code THEN
        RAISE EXCEPTION 'cannot modify card smart_code';
    END IF;
    IF OLD.card_uid IS NOT NULL AND OLD.card_uid IS DISTINCT FROM NEW.card_uid THEN
        RAISE EXCEPTION 'cannot modify card_uid once set';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_nfc_cards_protect_identity
    BEFORE UPDATE ON nfc_cards
    FOR EACH ROW
    EXECUTE FUNCTION protect_card_identity();
