-- Destructive rollback: member-profile links and the account scope of bulk/import jobs are lost.
-- Account-scoped jobs are deleted first so no app job survives as a platform job.
DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE slug IN ('app.company_profiles.bulk', 'app.company_profiles.import')
);
DELETE FROM permissions WHERE slug IN ('app.company_profiles.bulk', 'app.company_profiles.import');

DROP TABLE company_member_profiles;
ALTER TABLE profiles DROP CONSTRAINT uq_profiles_account_id;

DELETE FROM import_jobs WHERE account_id IS NOT NULL;
DROP INDEX import_jobs_account_idx;
ALTER TABLE import_jobs DROP COLUMN account_id;

DELETE FROM bulk_jobs WHERE account_id IS NOT NULL;
DROP INDEX bulk_jobs_account_idx;
ALTER TABLE bulk_jobs DROP COLUMN params_json, DROP COLUMN account_id;
