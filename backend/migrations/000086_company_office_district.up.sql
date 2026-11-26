-- TEC-1316: district (ilçe) of a company office, picked together with the city (il).
ALTER TABLE company_offices ADD COLUMN district VARCHAR(150) NOT NULL DEFAULT '';
