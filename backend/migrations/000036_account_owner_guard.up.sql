-- Every account keeps at least one owner (docs/TENANCY_AND_ACCESS.md). Checked at commit
-- (deferred), so a single transaction may swap owners, and concurrent demotions that would
-- leave no owner fail. Accounts deleted in the same transaction (cascades) are skipped.
CREATE OR REPLACE FUNCTION ensure_account_keeps_owner()
RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM accounts WHERE id = OLD.account_id)
       AND NOT EXISTS (
           SELECT 1 FROM account_members
           WHERE account_id = OLD.account_id AND role = 'owner'
       ) THEN
        RAISE EXCEPTION 'account % must keep at least one owner', OLD.account_id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_account_members_keep_owner
    AFTER UPDATE OR DELETE ON account_members
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION ensure_account_keeps_owner();
