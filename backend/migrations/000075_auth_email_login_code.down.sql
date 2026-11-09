DELETE FROM notification_templates WHERE code = 'auth.login_code';

-- Data-loss note: pending sign-in codes are dropped (short-lived, safe to lose).
DELETE FROM otp_codes WHERE type = 'login_code';
ALTER TABLE otp_codes DROP CONSTRAINT chk_otp_codes_type;
ALTER TABLE otp_codes ADD CONSTRAINT chk_otp_codes_type CHECK (
    type IN ('password_reset', 'email_verification')
);
