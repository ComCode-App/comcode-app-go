DELETE FROM notification_templates WHERE code IN (
    'profiles.verification.approved', 'profiles.verification.rejected', 'profiles.verification.revoked',
    'profiles.verification.re_review'
);

DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE slug = 'platform.profile_verifications.manage');

DELETE FROM permissions WHERE slug = 'platform.profile_verifications.manage';

DROP TABLE IF EXISTS profile_verification_requests;
