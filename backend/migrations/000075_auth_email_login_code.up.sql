-- TEC-1121: passwordless sign-in (and sign-up while registration is open) with a
-- one-time code sent by email. Codes live in otp_codes (hashed, single use,
-- attempt-limited) under the new type login_code; the email is a security email,
-- so the code is only ever stored sealed (TEC-1099).

ALTER TABLE otp_codes DROP CONSTRAINT chk_otp_codes_type;
ALTER TABLE otp_codes ADD CONSTRAINT chk_otp_codes_type CHECK (
    type IN ('password_reset', 'email_verification', 'login_code')
);

INSERT INTO notification_templates (code, channel, language, subject, body, active) VALUES
    ('auth.login_code', 'email', 'tr', 'Giriş kodunuz',
     E'Merhaba {{name}},\n\nGiriş kodunuz: {{code}}\nKod {{expires_minutes}} dakika geçerlidir ve yalnızca bir kez kullanılabilir.\n\nBu isteği siz yapmadıysanız bu e-postayı yok sayabilirsiniz.', TRUE),
    ('auth.login_code', 'email', 'en', 'Your sign-in code',
     E'Hello {{name}},\n\nYour sign-in code is: {{code}}\nThe code is valid for {{expires_minutes}} minutes and can be used once.\n\nIf you did not request this, you can ignore this email.', TRUE)
ON CONFLICT (code, channel, language) DO NOTHING;
