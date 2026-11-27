-- Data-loss note: deletes leads that came from business page forms (the old CHECK cannot hold them).
DELETE FROM event_definitions WHERE key IN ('business.page.view', 'business.button.click');
DELETE FROM leads WHERE source_type = 'business_page';
ALTER TABLE leads DROP CONSTRAINT chk_leads_source_type;
ALTER TABLE leads ADD CONSTRAINT chk_leads_source_type CHECK (source_type IN ('profile', 'card', 'smart_link'));
