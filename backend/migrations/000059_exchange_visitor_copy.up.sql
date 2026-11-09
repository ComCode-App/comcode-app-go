-- TEC-940: after a contact exchange the visitor gets a copy by email: what they shared, the
-- profile owner's vCard and a KVKK "delete my data" link. The email row carries
-- source_event = 'leads.exchange_copy:{lead uuid}' and is deleted together with the exchange.
CREATE INDEX idx_notifications_exchange_copy ON notifications (source_event)
    WHERE source_event LIKE 'leads.exchange_copy:%';

INSERT INTO notification_templates (code, channel, language, subject, body, active) VALUES
    ('leads.exchange_copy', 'email', 'tr', '{{owner}} ile paylaştığınız iletişim bilgileri',
     'Merhaba {{name}},

{{owner}} ile kartvizit değişimi yaptınız. Paylaştığınız bilgiler:

{{summary}}

{{owner}} kişisinin kartviziti (vCard) bu e-postanın ekindedir; rehberinize ekleyebilirsiniz.

Paylaştığınız bilgilerin silinmesini KVKK kapsamında istediğiniz zaman talep edebilirsiniz. Aşağıdaki bağlantı kaydınızı kalıcı olarak siler ve yalnızca bir kez kullanılabilir:
{{delete_url}}

Bu e-postayı, {{owner}} kişisinin profil sayfasındaki kartvizit değişimi formunu doldurduğunuz için aldınız.', TRUE),
    ('leads.exchange_copy', 'email', 'en', 'The contact details you shared with {{owner}}',
     'Hello {{name}},

You exchanged contact details with {{owner}}. This is what you shared:

{{summary}}

{{owner}}''s contact card (vCard) is attached; you can add it to your contacts.

You can ask for your details to be deleted at any time. The link below deletes your record for good and works only once:
{{delete_url}}

You received this email because you filled in the contact exchange form on {{owner}}''s profile page.', TRUE);
