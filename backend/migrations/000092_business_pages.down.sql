-- Data-loss note: drops every business page, its blocks and their username routes.
DELETE FROM plan_limits WHERE feature_key = 'business_page.enabled';
DELETE FROM feature_definitions WHERE key = 'business_page.enabled';
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE slug IN ('app.business_page.read', 'app.business_page.write'));
DELETE FROM permissions WHERE slug IN ('app.business_page.read', 'app.business_page.write');

DROP INDEX IF EXISTS uq_routes_business_page_live;
DELETE FROM routes WHERE route_type = 'business_page';
ALTER TABLE routes DROP CONSTRAINT chk_routes_target_type;
ALTER TABLE routes ADD CONSTRAINT chk_routes_target_type CHECK (target_type IN (
    'profile', 'direct_url', 'vcard_download', 'social_hub', 'custom_modules'));
ALTER TABLE routes DROP CONSTRAINT chk_routes_route_type;
ALTER TABLE routes ADD CONSTRAINT chk_routes_route_type CHECK (route_type IN (
    'canonical_profile', 'custom_smart_link', 'card_route', 'legacy_route', 'system_reserved'));

DROP TABLE IF EXISTS business_page_blocks;
DROP TABLE IF EXISTS business_pages;
