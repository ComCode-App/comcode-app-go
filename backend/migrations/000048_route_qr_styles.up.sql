ALTER TABLE routes ADD COLUMN qr_style jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(qr_style) = 'object');
