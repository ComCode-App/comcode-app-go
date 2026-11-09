-- Card routes added for app-created cards stay: they are valid routes on their own.
DELETE FROM routes WHERE route_type = 'canonical_profile';
DELETE FROM event_definitions WHERE key IN ('contact.whatsapp.click', 'contact.address.click', 'social.link.click', 'bank.iban.copy');
DROP TABLE IF EXISTS profile_bank_accounts;
DROP INDEX IF EXISTS uq_routes_canonical_profile_live;
