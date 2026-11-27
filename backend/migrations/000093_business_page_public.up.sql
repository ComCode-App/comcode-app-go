-- TEC-1327 (part C): the public business page. Adds the lead source of a business page form and
-- the page's visitor events. New allowed values and new rows only; no existing row changes.
ALTER TABLE leads DROP CONSTRAINT chk_leads_source_type;
ALTER TABLE leads ADD CONSTRAINT chk_leads_source_type CHECK (source_type IN ('profile', 'card', 'smart_link', 'business_page'));

INSERT INTO event_definitions (key, name, description, category, schema_version, enabled, is_public, aggregate_enabled, retention_days, required_properties)
VALUES
    ('business.page.view', 'Business Page View', 'Visitor opened a company business page', 'card', 1, true, true, true, 365, '{}'),
    ('business.button.click', 'Business Button Click', 'Visitor clicked an action button of a business page (property: target)', 'contact', 1, true, true, true, 365, '{}')
ON CONFLICT (key) DO NOTHING;
