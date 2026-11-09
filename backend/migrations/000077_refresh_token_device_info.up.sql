-- TEC-1179 (mobile B2): device sessions and the mobile refresh policy.
--
-- client_type is fixed when a session family starts (login, email code, OAuth, passkey) from
-- the X-Client header: 'mobile' sessions get JWT_REFRESH_TTL_MOBILE, 'web' keeps
-- JWT_REFRESH_TTL. Rotation copies it, so a refresh can never change a family's lifetime.
-- device_name / app_version are display-only (sessions list) and come from client headers.
--
-- rotated_at marks a token that was replaced by its successor (refresh or account switch), as
-- opposed to one revoked by logout or session revocation. Presenting a rotated token again is
-- reuse: within the race tolerance it answers 409 REFRESH_RACE, after it the whole family is
-- revoked and the owner gets a security email (auth.refresh_reuse).

ALTER TABLE refresh_tokens
    ADD COLUMN client_type TEXT NOT NULL DEFAULT 'web',
    ADD COLUMN device_name TEXT NULL,
    ADD COLUMN app_version TEXT NULL,
    ADD COLUMN rotated_at  TIMESTAMPTZ NULL,
    ADD CONSTRAINT chk_refresh_tokens_client_type CHECK (client_type IN ('web', 'mobile')),
    ADD CONSTRAINT chk_refresh_tokens_device_name CHECK (char_length(device_name) <= 100),
    ADD CONSTRAINT chk_refresh_tokens_app_version CHECK (char_length(app_version) <= 32);

INSERT INTO notification_templates (code, channel, language, subject, body, active) VALUES
    ('auth.refresh_reuse', 'email', 'tr', 'Oturumunuz güvenlik nedeniyle kapatıldı',
     E'Merhaba {{name}},\n\nHesabınızdaki bir oturumun ({{device}}) daha önce kullanılmış bir oturum anahtarı ile yenilenmeye çalışıldığını gördük. Güvenliğiniz için bu oturumu kapattık; o cihazda yeniden giriş yapmanız gerekecek.\n\nBu siz değilseniz şifrenizi değiştirin ve hesap sayfanızdaki Aktif oturumlar bölümünden diğer oturumlarınızı kontrol edin.', TRUE),
    ('auth.refresh_reuse', 'email', 'en', 'A session was signed out for your security',
     E'Hello {{name}},\n\nA session on your account ({{device}}) was refreshed with a sign-in token that had already been used. To keep your account safe we signed that session out; you will need to sign in again on that device.\n\nIf this was not you, change your password and review your other sessions in the Active sessions section of your account page.', TRUE)
ON CONFLICT (code, channel, language) DO NOTHING;
