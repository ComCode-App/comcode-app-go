-- TEC-549: platform-approved "verified profile" badge.
--
-- One row per verification request; a profile's current status is its latest row:
--   pending -> pending, approved -> verified, rejected -> rejected, revoked/cancelled -> none.
-- At most one open (pending or approved) request per profile. When a verified profile changes
-- its display name or organization the approved row is revoked (revoke_reason records why) and
-- a pending re-review row (origin = 're_review') is opened in the same step.
CREATE TABLE profile_verification_requests (
    id                   BIGSERIAL    PRIMARY KEY,
    uuid                 UUID         NOT NULL DEFAULT gen_random_uuid(),
    account_id           BIGINT       NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id           BIGINT       NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    status               VARCHAR(16)  NOT NULL DEFAULT 'pending',
    origin               VARCHAR(16)  NOT NULL DEFAULT 'owner',
    note                 TEXT         NOT NULL DEFAULT '',
    requested_by_user_id BIGINT       NULL REFERENCES users(id) ON DELETE SET NULL,
    reviewed_by_user_id  BIGINT       NULL REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at          TIMESTAMPTZ  NULL,
    review_reason        TEXT         NOT NULL DEFAULT '',
    revoked_by_user_id   BIGINT       NULL REFERENCES users(id) ON DELETE SET NULL,
    revoked_at           TIMESTAMPTZ  NULL,
    revoke_reason        TEXT         NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_profile_verification_requests_uuid UNIQUE (uuid),
    CONSTRAINT chk_profile_verification_requests_status
        CHECK (status IN ('pending', 'approved', 'rejected', 'revoked', 'cancelled')),
    CONSTRAINT chk_profile_verification_requests_origin CHECK (origin IN ('owner', 're_review'))
);

CREATE UNIQUE INDEX uq_profile_verification_requests_open
    ON profile_verification_requests (profile_id)
    WHERE status IN ('pending', 'approved');
CREATE INDEX idx_profile_verification_requests_profile
    ON profile_verification_requests (profile_id, id DESC);
CREATE INDEX idx_profile_verification_requests_status
    ON profile_verification_requests (status, created_at);

CREATE TRIGGER trg_profile_verification_requests_set_updated_at
    BEFORE UPDATE ON profile_verification_requests
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

INSERT INTO permissions (name, slug) VALUES
    ('Manage profile verifications', 'platform.profile_verifications.manage')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.slug = 'super_admin'
  AND p.slug = 'platform.profile_verifications.manage'
ON CONFLICT DO NOTHING;

INSERT INTO notification_templates (code, channel, language, subject, body, active) VALUES
    ('profiles.verification.approved', 'inapp', 'tr', 'Profiliniz doğrulandı',
     '{{profile_name}} profili doğrulandı. Doğrulanmış profil rozeti artık herkese açık sayfanızda görünüyor.', TRUE),
    ('profiles.verification.approved', 'email', 'tr', 'Profiliniz doğrulandı',
     '{{profile_name}} profili doğrulandı. Doğrulanmış profil rozeti artık herkese açık sayfanızda görünüyor: {{profile_url}}', TRUE),
    ('profiles.verification.approved', 'inapp', 'en', 'Your profile is verified',
     'The {{profile_name}} profile was verified. The verified badge now shows on your public page.', TRUE),
    ('profiles.verification.approved', 'email', 'en', 'Your profile is verified',
     'The {{profile_name}} profile was verified. The verified badge now shows on your public page: {{profile_url}}', TRUE),
    ('profiles.verification.rejected', 'inapp', 'tr', 'Doğrulama talebi reddedildi',
     '{{profile_name}} profili için doğrulama talebi reddedildi. Neden: {{reason}}', TRUE),
    ('profiles.verification.rejected', 'email', 'tr', 'Doğrulama talebi reddedildi',
     '{{profile_name}} profili için doğrulama talebi reddedildi. Neden: {{reason}}. Bilgileri düzenleyip yeniden talep edebilirsiniz: {{profile_url}}', TRUE),
    ('profiles.verification.rejected', 'inapp', 'en', 'Verification request rejected',
     'The verification request for {{profile_name}} was rejected. Reason: {{reason}}', TRUE),
    ('profiles.verification.rejected', 'email', 'en', 'Verification request rejected',
     'The verification request for {{profile_name}} was rejected. Reason: {{reason}}. You can update the profile and request again: {{profile_url}}', TRUE),
    ('profiles.verification.revoked', 'inapp', 'tr', 'Doğrulama rozeti kaldırıldı',
     '{{profile_name}} profilinin doğrulama rozeti kaldırıldı. Neden: {{reason}}', TRUE),
    ('profiles.verification.revoked', 'inapp', 'en', 'Verified badge removed',
     'The verified badge of {{profile_name}} was removed. Reason: {{reason}}', TRUE),
    ('profiles.verification.re_review', 'inapp', 'tr', 'Profil yeniden incelemede',
     '{{profile_name}} profilinin adı veya kurumu değiştiği için doğrulama rozeti kaldırıldı; profil yeniden incelemeye alındı.', TRUE),
    ('profiles.verification.re_review', 'inapp', 'en', 'Profile back in review',
     'The name or organization of {{profile_name}} changed, so its verified badge was removed and the profile is back in review.', TRUE);
