-- Data-loss note: drops the stored office districts.
ALTER TABLE company_offices DROP COLUMN district;
