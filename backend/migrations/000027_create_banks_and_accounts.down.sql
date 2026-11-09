DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE slug IN ('platform.banks.read', 'platform.banks.write', 'app.bank_accounts.read', 'app.bank_accounts.write')
);

DELETE FROM permissions
WHERE slug IN ('platform.banks.read', 'platform.banks.write', 'app.bank_accounts.read', 'app.bank_accounts.write');

DROP TABLE IF EXISTS user_bank_accounts;
DROP TABLE IF EXISTS banks;
