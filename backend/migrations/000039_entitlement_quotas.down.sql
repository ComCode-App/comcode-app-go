DELETE FROM feature_definitions WHERE key IN ('profiles.max', 'smart_links.max', 'analytics.history_days');

UPDATE feature_definitions
SET label = 'Maximum cards',
    description = 'Maximum number of digital business cards'
WHERE key = 'cards.max';
