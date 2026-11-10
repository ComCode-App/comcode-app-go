-- TEC-1184: QR sign-in. Sessions live in Redis only (no table); this seeds the notice sent
-- after a user approves a web sign-in on their phone: a security email (sent even when email
-- notifications are off) and an in-app notification. Variables: name, device, ip, location.

INSERT INTO notification_templates (code, channel, language, subject, body, active) VALUES
    ('auth.qr_login_approved', 'email', 'tr', 'Yeni bir web oturumu açıldı',
     E'Merhaba {{name}},\n\nTelefonunuzdaki uygulamadan bir web oturumunu QR kod ile onayladınız.\n\nTarayıcı: {{device}}\nIP adresi: {{ip}}\nYaklaşık konum: {{location}}\n\nBu onayı siz vermediyseniz hesabınızdaki oturumlar bölümünden bu oturumu hemen kapatın ve şifrenizi değiştirin.', TRUE),
    ('auth.qr_login_approved', 'email', 'en', 'A new web session was opened',
     E'Hello {{name}},\n\nYou approved a web sign-in with a QR code from the app on your phone.\n\nBrowser: {{device}}\nIP address: {{ip}}\nApproximate location: {{location}}\n\nIf you did not approve this, end that session from the sessions section of your account right away and change your password.', TRUE),
    ('auth.qr_login_approved', 'inapp', 'tr', 'Yeni bir web oturumu açıldı',
     'QR kod ile bir web oturumu onaylandı: {{device}}, {{ip}}. Siz değilseniz oturumu kapatın.', TRUE),
    ('auth.qr_login_approved', 'inapp', 'en', 'A new web session was opened',
     'A web sign-in was approved with a QR code: {{device}}, {{ip}}. If this was not you, end the session.', TRUE)
ON CONFLICT (code, channel, language) DO NOTHING;
