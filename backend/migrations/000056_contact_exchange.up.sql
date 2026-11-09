-- TEC-567: contact exchange. A visitor of a public profile shares their own details back with the
-- profile owner, either by filling in a form or, when signed in, by sharing one of their ComCode
-- profiles. Exchanges are leads of kind 'exchange': same inbox, consent evidence and hard delete.
ALTER TABLE leads
    ADD COLUMN kind              VARCHAR(16)  NOT NULL DEFAULT 'form',
    ADD COLUMN company           VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN job_title         VARCHAR(150) NOT NULL DEFAULT '',
    -- Set when a signed-in visitor shared a ComCode profile; the details above are a snapshot.
    ADD COLUMN sender_user_id    BIGINT       NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN sender_profile_id BIGINT       NULL REFERENCES profiles(id) ON DELETE SET NULL,
    ADD CONSTRAINT chk_leads_kind CHECK (kind IN ('form', 'exchange'));

INSERT INTO notification_templates (code, channel, language, subject, body, active) VALUES
    ('leads.exchange_received', 'inapp', 'en', 'New contact exchange',
     '{{name}} shared their contact details with you via {{source}}.', TRUE),
    ('leads.exchange_received', 'realtime', 'en', 'New contact exchange',
     '{{name}} shared their contact details with you.', TRUE),
    ('leads.exchange_received', 'inapp', 'tr', 'Yeni kartvizit değişimi',
     '{{name}}, {{source}} üzerinden iletişim bilgilerini sizinle paylaştı.', TRUE),
    ('leads.exchange_received', 'realtime', 'tr', 'Yeni kartvizit değişimi',
     '{{name}} iletişim bilgilerini sizinle paylaştı.', TRUE);
