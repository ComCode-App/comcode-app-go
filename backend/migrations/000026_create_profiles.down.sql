DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE slug IN (
        'app.profiles.read',
        'app.profiles.write',
        'platform.social_platforms.read',
        'platform.social_platforms.write'
    )
);

DELETE FROM permissions
WHERE slug IN (
    'app.profiles.read',
    'app.profiles.write',
    'platform.social_platforms.read',
    'platform.social_platforms.write'
);

DROP TABLE IF EXISTS profile_social_links;
DROP TABLE IF EXISTS profile_contact_details;
DROP TABLE IF EXISTS profile_addresses;
DROP TABLE IF EXISTS profile_translations;
DROP TABLE IF EXISTS profiles;
DROP TABLE IF EXISTS social_platforms;
