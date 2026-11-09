-- TEC-945: email invitations to company accounts.
--
-- An owner/admin invites any email address with an account role. The link carries a random
-- token; only its SHA-256 is stored. An invite is pending until it is accepted or revoked and
-- expires 7 days after it was (re)sent; "expired" is derived from expires_at, not stored.
-- The employee import (TEC-573) creates the employee profile at once for an email without a
-- user and attaches it here (profile_id, office_id, group_id); accepting links it to the new member.
CREATE TABLE account_invites (
    id                  BIGSERIAL    PRIMARY KEY,
    uuid                UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    account_id          BIGINT       NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    email               VARCHAR(255) NOT NULL CHECK (email = lower(btrim(email)) AND email <> ''),
    role                account_role NOT NULL DEFAULT 'member',
    token_hash          BYTEA        NOT NULL UNIQUE,
    status              TEXT         NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'revoked')),
    source              TEXT         NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'import')),
    invited_by_user_id  BIGINT       NULL REFERENCES users (id) ON DELETE SET NULL,
    profile_id          BIGINT       NULL,
    office_id           BIGINT       NULL,
    group_id            BIGINT       NULL,
    expires_at          TIMESTAMPTZ  NOT NULL,
    last_sent_at        TIMESTAMPTZ  NULL,
    send_count          INTEGER      NOT NULL DEFAULT 0,
    accepted_at         TIMESTAMPTZ  NULL,
    accepted_by_user_id BIGINT       NULL REFERENCES users (id) ON DELETE SET NULL,
    revoked_at          TIMESTAMPTZ  NULL,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    -- Same-account references only; deleting the profile, office or group clears the reference.
    FOREIGN KEY (account_id, profile_id) REFERENCES profiles (account_id, id) ON DELETE SET NULL (profile_id),
    FOREIGN KEY (account_id, office_id) REFERENCES company_offices (account_id, id) ON DELETE SET NULL (office_id),
    FOREIGN KEY (account_id, group_id) REFERENCES company_groups (account_id, id) ON DELETE SET NULL (group_id)
);
-- One open invite per address and account; accepted/revoked rows are history.
CREATE UNIQUE INDEX uq_account_invites_pending ON account_invites (account_id, email) WHERE status = 'pending';
CREATE INDEX idx_account_invites_account ON account_invites (account_id, created_at DESC);
CREATE UNIQUE INDEX uq_account_invites_profile ON account_invites (profile_id) WHERE status = 'pending' AND profile_id IS NOT NULL;
CREATE TRIGGER trg_account_invites_updated BEFORE UPDATE ON account_invites
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Only company accounts take invites.
CREATE OR REPLACE FUNCTION guard_account_invites_company()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM accounts WHERE id = NEW.account_id AND type = 'company') THEN
        RAISE EXCEPTION 'invites need a company account' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_account_invites_company BEFORE INSERT ON account_invites
    FOR EACH ROW EXECUTE FUNCTION guard_account_invites_company();

INSERT INTO permissions (name, slug) VALUES
    ('Read account invites', 'app.account_invites.read'),
    ('Manage account invites', 'app.account_invites.write')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug IN ('super_admin', 'subscriber')
  AND p.slug IN ('app.account_invites.read', 'app.account_invites.write')
ON CONFLICT DO NOTHING;

-- Sent to the invited address only (no user row may exist yet), so email channel only.
INSERT INTO notification_templates (code, channel, language, subject, body, active) VALUES
    ('accounts.invite', 'email', 'tr',
     '{{inviter_name}} sizi {{account_name}} hesabına davet etti',
     E'Merhaba,\n\n{{inviter_name}} sizi ComCode''da {{account_name}} hesabına {{role}} olarak davet etti.\n\nDaveti kabul etmek için bağlantıyı açın: {{invite_url}}\n\nHesabınız yoksa aynı bağlantıdan bu e-posta adresiyle kayıt olabilirsiniz. Bağlantı {{expires_days}} gün geçerlidir.',
     TRUE),
    ('accounts.invite', 'email', 'en',
     '{{inviter_name}} invited you to {{account_name}}',
     E'Hello,\n\n{{inviter_name}} invited you to join {{account_name}} on ComCode as {{role}}.\n\nOpen the link to accept: {{invite_url}}\n\nIf you do not have an account yet, sign up with this email address from the same link. The link is valid for {{expires_days}} days.',
     TRUE)
ON CONFLICT DO NOTHING;
