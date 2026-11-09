-- name: LockStructureAccount :one
SELECT id FROM accounts WHERE id=$1 AND type='company' FOR UPDATE;

-- name: StructureActorRole :one
SELECT role FROM account_members WHERE account_id=$1 AND user_id=$2 FOR UPDATE;

-- name: StructureHasOfficeScope :one
SELECT EXISTS(SELECT 1 FROM account_member_scopes s JOIN account_members m ON m.id=s.member_id
 JOIN company_offices o ON o.id=s.office_id AND o.account_id=s.account_id
 WHERE s.account_id=$1 AND m.user_id=$2 AND o.uuid=$3);

-- name: ListStructureScopes :many
SELECT o.uuid FROM account_member_scopes s JOIN account_members m ON m.id=s.member_id
 JOIN users u ON u.id=m.user_id JOIN company_offices o ON o.id=s.office_id
 WHERE s.account_id=$1 AND u.uuid=$2 ORDER BY o.uuid;

-- name: ClearStructureScopes :exec
DELETE FROM account_member_scopes s WHERE s.account_id=$1 AND s.member_id=(SELECT m.id FROM account_members m JOIN users u ON u.id=m.user_id WHERE m.account_id=$1 AND u.uuid=$2);

-- name: AddStructureScope :execrows
INSERT INTO account_member_scopes(account_id,member_id,office_id)
SELECT $1,m.id,o.id FROM account_members m JOIN users u ON u.id=m.user_id
JOIN company_offices o ON o.account_id=m.account_id
WHERE m.account_id=$1 AND u.uuid=$2 AND o.uuid=$3 ON CONFLICT DO NOTHING;

-- name: SetCompanySector :execrows
UPDATE accounts SET sector_id=(SELECT id FROM sectors WHERE uuid=sqlc.narg(sector_uuid)::uuid)
WHERE accounts.id=sqlc.arg(account_id) AND accounts.type='company'
AND (sqlc.narg(sector_uuid)::uuid IS NULL OR EXISTS(SELECT 1 FROM sectors WHERE uuid=sqlc.narg(sector_uuid)::uuid));

-- name: GetCompanySector :one
SELECT s.uuid AS sector_uuid FROM accounts a LEFT JOIN sectors s ON s.id=a.sector_id WHERE a.id=$1 AND a.type='company';

-- name: ListSectors :many
SELECT * FROM sectors WHERE name_tr ILIKE '%' || sqlc.arg(q)::text || '%' OR name_en ILIKE '%' || sqlc.arg(q)::text || '%' OR slug ILIKE '%' || sqlc.arg(q)::text || '%'
ORDER BY slug LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountSectors :one
SELECT count(*) FROM sectors WHERE name_tr ILIKE '%' || sqlc.arg(q)::text || '%' OR name_en ILIKE '%' || sqlc.arg(q)::text || '%' OR slug ILIKE '%' || sqlc.arg(q)::text || '%';

-- name: GetSectorAdmin :one
SELECT * FROM sectors WHERE uuid=$1;

-- name: CreateSectorAdmin :one
INSERT INTO sectors(slug,name_tr,name_en) VALUES($1,$2,$3) RETURNING *;

-- name: UpdateSectorAdmin :one
UPDATE sectors SET slug=$2,name_tr=$3,name_en=$4 WHERE uuid=$1 RETURNING *;

-- name: DeleteSectorAdmin :execrows
DELETE FROM sectors WHERE uuid=$1;

-- name: ListStructureOffices :many
-- restricted limits the list to visible_offices (TEC-948 role visibility); false lists all.
SELECT t.* FROM company_offices t WHERE t.account_id=sqlc.arg(account_id)
AND (NOT sqlc.arg(restricted)::bool OR t.uuid=ANY(sqlc.arg(visible_offices)::uuid[]))
ORDER BY t.sort_order,t.name,t.id LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountStructureOffices :one
SELECT count(*) FROM company_offices t WHERE t.account_id=sqlc.arg(account_id)
AND (NOT sqlc.arg(restricted)::bool OR t.uuid=ANY(sqlc.arg(visible_offices)::uuid[]));

-- name: GetStructureOffice :one
SELECT t.* FROM company_offices t WHERE t.account_id=$1 AND t.uuid=$2;

-- name: CreateStructureOffice :one
INSERT INTO company_offices (account_id,name,city,address,phone,sort_order) VALUES (sqlc.arg(account_id),sqlc.arg(name),sqlc.arg(city),sqlc.arg(address),sqlc.arg(phone),sqlc.arg(sort_order)) RETURNING *;

-- name: UpdateStructureOffice :one
UPDATE company_offices SET name=sqlc.arg(name),city=sqlc.arg(city),address=sqlc.arg(address),phone=sqlc.arg(phone),sort_order=sqlc.arg(sort_order) WHERE company_offices.account_id=sqlc.arg(account_id) AND company_offices.uuid=sqlc.arg(uuid) RETURNING *;

-- name: DeleteStructureOffice :execrows
DELETE FROM company_offices WHERE account_id=$1 AND uuid=$2;

-- name: ListStructureGroups :many
-- office_uuid narrows to one office's groups; company_wide narrows to groups without an office.
-- restricted keeps groups of visible_offices plus the visible_groups themselves.
SELECT t.*, o.uuid AS office_uuid FROM company_groups t LEFT JOIN company_offices o ON o.id=t.office_id AND o.account_id=t.account_id WHERE t.account_id=sqlc.arg(account_id)
AND (sqlc.narg(filter_office_uuid)::uuid IS NULL OR o.uuid=sqlc.narg(filter_office_uuid)::uuid)
AND (NOT sqlc.arg(company_wide)::bool OR t.office_id IS NULL)
AND (NOT sqlc.arg(restricted)::bool OR o.uuid=ANY(sqlc.arg(visible_offices)::uuid[]) OR t.uuid=ANY(sqlc.arg(visible_groups)::uuid[]))
ORDER BY t.name,t.id LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountStructureGroups :one
SELECT count(*) FROM company_groups t LEFT JOIN company_offices o ON o.id=t.office_id AND o.account_id=t.account_id WHERE t.account_id=sqlc.arg(account_id)
AND (sqlc.narg(filter_office_uuid)::uuid IS NULL OR o.uuid=sqlc.narg(filter_office_uuid)::uuid)
AND (NOT sqlc.arg(company_wide)::bool OR t.office_id IS NULL)
AND (NOT sqlc.arg(restricted)::bool OR o.uuid=ANY(sqlc.arg(visible_offices)::uuid[]) OR t.uuid=ANY(sqlc.arg(visible_groups)::uuid[]));

-- name: GetStructureGroup :one
SELECT t.*, o.uuid AS office_uuid FROM company_groups t LEFT JOIN company_offices o ON o.id=t.office_id AND o.account_id=t.account_id WHERE t.account_id=$1 AND t.uuid=$2;

-- name: CreateStructureGroup :one
INSERT INTO company_groups (account_id,name,kind,office_id) SELECT sqlc.arg(account_id),sqlc.arg(name),sqlc.arg(kind),(SELECT o.id FROM company_offices o WHERE o.account_id=sqlc.arg(account_id) AND o.uuid=sqlc.narg(office_uuid)::uuid) WHERE (sqlc.narg(office_uuid)::uuid IS NULL OR EXISTS (SELECT 1 FROM company_offices o WHERE o.account_id=sqlc.arg(account_id) AND o.uuid=sqlc.narg(office_uuid)::uuid)) RETURNING *;

-- name: UpdateStructureGroup :one
UPDATE company_groups SET name=sqlc.arg(name),kind=sqlc.arg(kind), office_id=(SELECT o.id FROM company_offices o WHERE o.account_id=sqlc.arg(account_id) AND o.uuid=sqlc.narg(office_uuid)::uuid) WHERE company_groups.account_id=sqlc.arg(account_id) AND company_groups.uuid=sqlc.arg(uuid) AND (sqlc.narg(office_uuid)::uuid IS NULL OR EXISTS(SELECT 1 FROM company_offices o WHERE o.account_id=sqlc.arg(account_id) AND o.uuid=sqlc.narg(office_uuid)::uuid)) RETURNING *;

-- name: DeleteStructureGroup :execrows
DELETE FROM company_groups WHERE account_id=$1 AND uuid=$2;

-- name: GetStructureAssignment :one
SELECT u.uuid AS user_uuid,o.uuid AS office_uuid,g.uuid AS group_uuid
FROM account_members m JOIN users u ON u.id=m.user_id
LEFT JOIN company_member_assignments a ON a.member_id=m.id AND a.account_id=m.account_id
LEFT JOIN company_offices o ON o.id=a.office_id AND o.account_id=m.account_id
LEFT JOIN company_groups g ON g.id=a.group_id AND g.account_id=m.account_id
WHERE m.account_id=$1 AND u.uuid=$2;

-- name: ListStructureAssignments :many
-- Every member of the account with its (possibly empty) office/group placement. restricted keeps
-- the viewer, members of visible_offices and (include_unassigned) plain members without an office.
SELECT u.uuid AS user_uuid,o.uuid AS office_uuid,g.uuid AS group_uuid
FROM account_members m JOIN users u ON u.id=m.user_id
LEFT JOIN company_member_assignments a ON a.member_id=m.id AND a.account_id=m.account_id
LEFT JOIN company_offices o ON o.id=a.office_id AND o.account_id=m.account_id
LEFT JOIN company_groups g ON g.id=a.group_id AND g.account_id=m.account_id
WHERE m.account_id=sqlc.arg(account_id)
AND (NOT sqlc.arg(restricted)::bool OR u.uuid=sqlc.arg(self_uuid)::uuid OR o.uuid=ANY(sqlc.arg(visible_offices)::uuid[])
 OR (sqlc.arg(include_unassigned)::bool AND o.id IS NULL AND m.role='member'))
ORDER BY m.id LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountStructureAssignments :one
SELECT count(*)
FROM account_members m JOIN users u ON u.id=m.user_id
LEFT JOIN company_member_assignments a ON a.member_id=m.id AND a.account_id=m.account_id
LEFT JOIN company_offices o ON o.id=a.office_id AND o.account_id=m.account_id
WHERE m.account_id=sqlc.arg(account_id)
AND (NOT sqlc.arg(restricted)::bool OR u.uuid=sqlc.arg(self_uuid)::uuid OR o.uuid=ANY(sqlc.arg(visible_offices)::uuid[])
 OR (sqlc.arg(include_unassigned)::bool AND o.id IS NULL AND m.role='member'));

-- name: SetStructureAssignment :execrows
INSERT INTO company_member_assignments(account_id,member_id,office_id,group_id)
SELECT sqlc.arg(account_id),m.id,
 (SELECT o.id FROM company_offices o WHERE o.account_id=sqlc.arg(account_id) AND o.uuid=sqlc.narg(office_uuid)::uuid),
 (SELECT id FROM company_groups WHERE account_id=sqlc.arg(account_id) AND uuid=sqlc.narg(group_uuid)::uuid)
FROM account_members m JOIN users u ON u.id=m.user_id WHERE m.account_id=sqlc.arg(account_id) AND u.uuid=sqlc.arg(user_uuid)
AND (sqlc.narg(office_uuid)::uuid IS NULL OR EXISTS(SELECT 1 FROM company_offices o WHERE o.account_id=sqlc.arg(account_id) AND o.uuid=sqlc.narg(office_uuid)::uuid))
AND (sqlc.narg(group_uuid)::uuid IS NULL OR EXISTS(SELECT 1 FROM company_groups g WHERE g.account_id=sqlc.arg(account_id) AND g.uuid=sqlc.narg(group_uuid)::uuid))
ON CONFLICT(member_id) DO UPDATE SET office_id=excluded.office_id,group_id=excluded.group_id;

-- name: GetStructureAccountAdmin :one
SELECT id FROM accounts WHERE uuid=$1 AND type='company';
