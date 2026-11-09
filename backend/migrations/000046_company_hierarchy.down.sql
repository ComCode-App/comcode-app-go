-- Destructive rollback: structure, assignments, scope grants and sector selections are lost.
DROP TRIGGER trg_member_role_scopes ON account_members;
DROP FUNCTION clear_company_manager_scopes();
DROP TABLE account_member_scopes, company_member_assignments, company_groups, company_offices;
DROP FUNCTION guard_company_structure(), guard_company_assignment(), guard_company_group_move();
ALTER TABLE account_members DROP CONSTRAINT uq_account_members_account_id;
UPDATE account_members SET role='member' WHERE role::text='office_manager';
-- PostgreSQL cannot remove a single enum value safely; retain the unused office_manager value.
ALTER TABLE accounts DROP COLUMN sector_id;
DROP TABLE sectors;
DELETE FROM permissions WHERE slug IN ('app.company_structure.read','app.company_structure.write','platform.sectors.read','platform.sectors.write');
