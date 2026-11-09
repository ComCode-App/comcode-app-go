-- TEC-571: company structure is owned by accounts; global roles remain unchanged.
ALTER TYPE account_role ADD VALUE IF NOT EXISTS 'office_manager';
CREATE TABLE sectors (
 id BIGSERIAL PRIMARY KEY, uuid UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
 slug VARCHAR(80) NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
 name_tr VARCHAR(150) NOT NULL CHECK (btrim(name_tr) <> ''),
 name_en VARCHAR(150) NOT NULL CHECK (btrim(name_en) <> ''),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_sectors_updated BEFORE UPDATE ON sectors FOR EACH ROW EXECUTE FUNCTION set_updated_at();
INSERT INTO sectors(slug,name_tr,name_en) VALUES
 ('technology','Teknoloji','Technology'), ('finance','Finans','Finance'),
 ('healthcare','Sağlık','Healthcare'), ('education','Eğitim','Education'),
 ('retail','Perakende','Retail'), ('manufacturing','Üretim','Manufacturing'),
 ('construction','İnşaat','Construction'), ('tourism','Turizm','Tourism'),
 ('logistics','Lojistik','Logistics'), ('professional-services','Profesyonel Hizmetler','Professional Services'),
 ('media','Medya','Media'), ('other','Diğer','Other');
ALTER TABLE accounts ADD COLUMN sector_id BIGINT REFERENCES sectors(id) ON DELETE NO ACTION;
ALTER TABLE accounts ADD CONSTRAINT chk_accounts_company_sector CHECK (sector_id IS NULL OR type = 'company');
CREATE INDEX idx_accounts_sector ON accounts(sector_id) WHERE sector_id IS NOT NULL;
CREATE TABLE company_offices (
 id BIGSERIAL PRIMARY KEY, uuid UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
 account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
 name VARCHAR(150) NOT NULL CHECK (btrim(name) <> ''), city VARCHAR(150) NOT NULL DEFAULT '',
 address TEXT NOT NULL DEFAULT '', phone VARCHAR(50) NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(account_id,id)
);
CREATE INDEX idx_company_offices_account_sort ON company_offices(account_id,sort_order,id);
CREATE TRIGGER trg_company_offices_updated BEFORE UPDATE ON company_offices FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TABLE company_groups (
 id BIGSERIAL PRIMARY KEY, uuid UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
 account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE, office_id BIGINT,
 name VARCHAR(150) NOT NULL CHECK (btrim(name) <> ''), kind TEXT NOT NULL CHECK (kind IN ('department','team')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(account_id,id), UNIQUE(account_id,id,office_id),
 FOREIGN KEY(account_id,office_id) REFERENCES company_offices(account_id,id) ON DELETE NO ACTION
);
CREATE INDEX idx_company_groups_account_office ON company_groups(account_id,office_id);
CREATE TRIGGER trg_company_groups_updated BEFORE UPDATE ON company_groups FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE account_members ADD CONSTRAINT uq_account_members_account_id UNIQUE(account_id,id);
CREATE TABLE company_member_assignments (
 account_id BIGINT NOT NULL, member_id BIGINT PRIMARY KEY, office_id BIGINT, group_id BIGINT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(account_id,member_id) REFERENCES account_members(account_id,id) ON DELETE CASCADE,
 FOREIGN KEY(account_id,office_id) REFERENCES company_offices(account_id,id) ON DELETE NO ACTION,
 FOREIGN KEY(account_id,group_id) REFERENCES company_groups(account_id,id) ON DELETE NO ACTION
);
CREATE INDEX idx_company_assignments_office ON company_member_assignments(account_id,office_id);
CREATE INDEX idx_company_assignments_group ON company_member_assignments(account_id,group_id);
CREATE TRIGGER trg_company_assignments_updated BEFORE UPDATE ON company_member_assignments FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TABLE account_member_scopes (
 account_id BIGINT NOT NULL, member_id BIGINT NOT NULL, office_id BIGINT NOT NULL,
 PRIMARY KEY(member_id,office_id),
 FOREIGN KEY(account_id,member_id) REFERENCES account_members(account_id,id) ON DELETE CASCADE,
 FOREIGN KEY(account_id,office_id) REFERENCES company_offices(account_id,id) ON DELETE CASCADE
);
CREATE INDEX idx_account_member_scopes_account_office ON account_member_scopes(account_id,office_id);
-- Lock the company account for structural writes, serializing scope revocation and changes.
CREATE FUNCTION guard_company_structure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM accounts WHERE id = NEW.account_id AND type = 'company' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'company account required' USING ERRCODE = '23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER trg_offices_company BEFORE INSERT OR UPDATE ON company_offices FOR EACH ROW EXECUTE FUNCTION guard_company_structure();
CREATE TRIGGER trg_groups_company BEFORE INSERT OR UPDATE ON company_groups FOR EACH ROW EXECUTE FUNCTION guard_company_structure();
CREATE TRIGGER trg_assignments_company BEFORE INSERT OR UPDATE ON company_member_assignments FOR EACH ROW EXECUTE FUNCTION guard_company_structure();
CREATE TRIGGER trg_scopes_company BEFORE INSERT OR UPDATE ON account_member_scopes FOR EACH ROW EXECUTE FUNCTION guard_company_structure();
-- An office-bound group and assignment must agree. Account-wide groups may be used in any office.
CREATE FUNCTION guard_company_assignment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM accounts WHERE id=NEW.account_id FOR UPDATE;
 IF NEW.group_id IS NOT NULL AND EXISTS (
  SELECT 1 FROM company_groups WHERE account_id=NEW.account_id AND id=NEW.group_id
  AND office_id IS NOT NULL AND office_id IS DISTINCT FROM NEW.office_id
 ) THEN RAISE EXCEPTION 'group and assignment offices differ' USING ERRCODE = '23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER trg_assignment_office BEFORE INSERT OR UPDATE ON company_member_assignments FOR EACH ROW EXECUTE FUNCTION guard_company_assignment();
CREATE FUNCTION guard_company_group_move() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM accounts WHERE id=NEW.account_id FOR UPDATE;
 IF NEW.office_id IS NOT NULL AND EXISTS (
 SELECT 1 FROM company_member_assignments WHERE account_id=NEW.account_id AND group_id=NEW.id
 AND office_id IS DISTINCT FROM NEW.office_id
 ) THEN RAISE EXCEPTION 'group has assignments in another office' USING ERRCODE = '23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER trg_group_move BEFORE UPDATE ON company_groups FOR EACH ROW EXECUTE FUNCTION guard_company_group_move();
INSERT INTO permissions(name,slug) VALUES
 ('Read company structure','app.company_structure.read'),('Write company structure','app.company_structure.write'),
 ('Read sectors','platform.sectors.read'),('Manage sectors','platform.sectors.write');
INSERT INTO role_permissions(role_id,permission_id)
 SELECT r.id,p.id FROM roles r CROSS JOIN permissions p WHERE
 (r.slug='super_admin' AND p.slug IN ('app.company_structure.read','app.company_structure.write','platform.sectors.read','platform.sectors.write')) OR
 (r.slug='subscriber' AND p.slug IN ('app.company_structure.read','app.company_structure.write')) ON CONFLICT DO NOTHING;

-- Grants do not survive demotion; promoting the member again requires explicit new grants.
CREATE FUNCTION clear_company_manager_scopes() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.role IS DISTINCT FROM OLD.role THEN
  DELETE FROM account_member_scopes WHERE account_id=NEW.account_id AND member_id=NEW.id;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER trg_member_role_scopes AFTER UPDATE OF role ON account_members FOR EACH ROW EXECUTE FUNCTION clear_company_manager_scopes();
