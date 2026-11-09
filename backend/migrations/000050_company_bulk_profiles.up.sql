-- TEC-573: bulk updates and employee import for company profiles.

-- Account-scoped engine jobs. A NULL account_id is a platform job (users, roles, ...); app jobs
-- always carry the active company account they were started in, and their action parameters.
ALTER TABLE bulk_jobs
    ADD COLUMN account_id  BIGINT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    ADD COLUMN params_json JSONB  NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX bulk_jobs_account_idx ON bulk_jobs (account_id, created_at DESC) WHERE account_id IS NOT NULL;

ALTER TABLE import_jobs
    ADD COLUMN account_id BIGINT NULL REFERENCES accounts (id) ON DELETE CASCADE;
CREATE INDEX import_jobs_account_idx ON import_jobs (account_id, created_at DESC) WHERE account_id IS NOT NULL;

-- Which company profile is a member's (employee's) card. profiles.user_id is the creator, so the
-- link is explicit: at most one profile per member and one member per profile, same account only.
ALTER TABLE profiles ADD CONSTRAINT uq_profiles_account_id UNIQUE (account_id, id);
CREATE TABLE company_member_profiles (
    account_id BIGINT      NOT NULL,
    member_id  BIGINT      PRIMARY KEY,
    profile_id BIGINT      NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (account_id, member_id) REFERENCES account_members (account_id, id) ON DELETE CASCADE,
    FOREIGN KEY (account_id, profile_id) REFERENCES profiles (account_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_company_member_profiles_account ON company_member_profiles (account_id);
CREATE TRIGGER trg_company_member_profiles_updated BEFORE UPDATE ON company_member_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_member_profiles_company BEFORE INSERT OR UPDATE ON company_member_profiles
    FOR EACH ROW EXECUTE FUNCTION guard_company_structure();

INSERT INTO permissions (name, slug) VALUES
    ('Bulk update company profiles', 'app.company_profiles.bulk'),
    ('Import company employees', 'app.company_profiles.import')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('super_admin', 'subscriber')
  AND p.slug IN ('app.company_profiles.bulk', 'app.company_profiles.import')
ON CONFLICT DO NOTHING;
