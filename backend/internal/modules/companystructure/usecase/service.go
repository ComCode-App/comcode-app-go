package usecase

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CompanyRepository interface {
	InCompany(context.Context, authctx.Principal, func(repository.Store, string) error) error
}
type Service struct{ repo CompanyRepository }

func New(repo CompanyRepository) *Service { return &Service{repo: repo} }
func (s *Service) run(ctx context.Context, p authctx.Principal, write bool, fn func(repository.Store, string) error) error {
	perm := rbac.PermAppCompanyStructureRead
	if write {
		perm = rbac.PermAppCompanyStructureWrite
	}
	if !p.HasPermission(perm) {
		return model.ErrForbidden
	}
	return s.repo.InCompany(ctx, p, fn)
}
func manager(role string) bool {
	return role == authctx.AccountRoleOwner || role == authctx.AccountRoleAdmin
}
func officeScope(ctx context.Context, q repository.Store, p authctx.Principal, role string, office *uuid.UUID) error {
	if manager(role) {
		return nil
	}
	if role != authctx.AccountRoleOfficeManager || office == nil {
		return model.ErrForbidden
	}
	ok, err := q.StructureHasOfficeScope(ctx, db.StructureHasOfficeScopeParams{
		AccountID: p.Account.ID,
		UserID:    p.UserInternal,
		Uuid:      *office,
	})
	if err != nil {
		return err
	}
	if !ok {
		return model.ErrForbidden
	}
	return nil
}

// viewer is what the actor may read (TEC-948). Owners/admins see the whole structure. Office
// managers see their granted offices with those offices' groups and members, plus the pool of
// unassigned plain members. Plain members see only their own office and group.
type viewer struct {
	all     bool
	self    uuid.UUID
	offices []uuid.UUID // offices listed by the office endpoints
	managed []uuid.UUID // offices whose groups and members are visible
	groups  []uuid.UUID // extra groups visible outside managed offices (the viewer's own)
	pool    bool        // unassigned plain members are visible
}

func loadViewer(ctx context.Context, q repository.Store, p authctx.Principal, role string) (viewer, error) {
	v := viewer{all: manager(role), self: p.UserID, offices: []uuid.UUID{}, managed: []uuid.UUID{}, groups: []uuid.UUID{}}
	if v.all {
		return v, nil
	}
	own, err := q.GetStructureAssignment(ctx, db.GetStructureAssignmentParams{AccountID: p.Account.ID, Uuid: p.UserID})
	if err != nil {
		return v, err
	}
	if role == authctx.AccountRoleOfficeManager {
		if v.managed, err = q.ListStructureScopes(ctx, db.ListStructureScopesParams{AccountID: p.Account.ID, Uuid: p.UserID}); err != nil {
			return v, err
		}
		v.offices = append(v.offices, v.managed...)
		v.pool = true
	}
	if id := publicID(own.OfficeUuid); id != nil && !v.manages(id) {
		v.offices = append(v.offices, *id)
	}
	if id := publicID(own.GroupUuid); id != nil {
		v.groups = append(v.groups, *id)
	}
	return v, nil
}
func contains(ids []uuid.UUID, id *uuid.UUID) bool {
	if id == nil {
		return false
	}
	for _, v := range ids {
		if v == *id {
			return true
		}
	}
	return false
}
func (v viewer) manages(office *uuid.UUID) bool { return v.all || contains(v.managed, office) }
func (v viewer) seesOffice(id uuid.UUID) bool   { return v.all || contains(v.offices, &id) }
func (v viewer) seesGroup(g model.Group) bool {
	return v.manages(g.OfficeUUID) || contains(v.groups, &g.UUID)
}

// seesMember mirrors ListStructureAssignments; role is the target's account role.
func (v viewer) seesMember(a model.Assignment, role string) bool {
	return v.all || a.UserUUID == v.self || v.manages(a.OfficeUUID) ||
		(v.pool && a.OfficeUUID == nil && role == authctx.AccountRoleMember)
}
func nullable(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}
func publicID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	v := uuid.UUID(id.Bytes)
	return &v
}
func validName(s string, max int) bool {
	return strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= max
}
func officeModel(v db.CompanyOffice) model.Office {
	return model.Office{
		UUID: v.Uuid, OfficeInput: model.OfficeInput{Name: v.Name, City: v.City, Address: v.Address, Phone: v.Phone, SortOrder: v.SortOrder},
		CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time,
	}
}
func assignmentModel(v db.GetStructureAssignmentRow) model.Assignment {
	return model.Assignment{UserUUID: v.UserUuid, OfficeUUID: publicID(v.OfficeUuid), GroupUUID: publicID(v.GroupUuid)}
}
func getGroup(ctx context.Context, q repository.Store, a int64, id uuid.UUID) (model.Group, error) {
	v, err := q.GetStructureGroup(ctx, db.GetStructureGroupParams{AccountID: a, Uuid: id})
	return model.Group{UUID: v.Uuid, GroupInput: model.GroupInput{Name: v.Name, Kind: v.Kind, OfficeUUID: publicID(v.OfficeUuid)}, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time}, err
}
func checkOffice(ctx context.Context, q repository.Store, a int64, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	_, err := q.GetStructureOffice(ctx, db.GetStructureOfficeParams{AccountID: a, Uuid: *id})
	return err
}
func (s *Service) Offices(ctx context.Context, p authctx.Principal, limit, offset int32) ([]model.Office, int64, error) {
	out := []model.Office{}
	var total int64
	err := s.run(ctx, p, false, func(q repository.Store, role string) error {
		vw, err := loadViewer(ctx, q, p, role)
		if err != nil {
			return err
		}
		rows, err := q.ListStructureOffices(ctx, db.ListStructureOfficesParams{
			AccountID:      p.Account.ID,
			Restricted:     !vw.all,
			VisibleOffices: vw.offices,
			LimitCount:     limit,
			OffsetCount:    offset,
		})
		if err != nil {
			return err
		}
		for _, v := range rows {
			out = append(out, officeModel(v))
		}
		total, err = q.CountStructureOffices(ctx, db.CountStructureOfficesParams{
			AccountID:      p.Account.ID,
			Restricted:     !vw.all,
			VisibleOffices: vw.offices,
		})
		return err
	})
	return out, total, err
}
func (s *Service) Office(ctx context.Context, p authctx.Principal, id uuid.UUID) (model.Office, error) {
	var out model.Office
	err := s.run(ctx, p, false, func(q repository.Store, role string) error {
		vw, err := loadViewer(ctx, q, p, role)
		if err != nil {
			return err
		}
		// Invisible offices read as missing so ids of other offices are not confirmed.
		if !vw.seesOffice(id) {
			return model.ErrNotFound
		}
		v, err := q.GetStructureOffice(ctx, db.GetStructureOfficeParams{AccountID: p.Account.ID, Uuid: id})
		out = officeModel(v)
		return err
	})
	return out, err
}

// SaveOffice uses replacement semantics for updates. Only company owners/admins manage offices.
func (s *Service) SaveOffice(ctx context.Context, p authctx.Principal, id uuid.UUID, in model.OfficeInput) (model.Office, error) {
	in.Name = strings.TrimSpace(in.Name)
	if !validName(in.Name, 150) || utf8.RuneCountInString(in.City) > 150 || utf8.RuneCountInString(in.Phone) > 50 || utf8.RuneCountInString(in.Address) > 10000 {
		return model.Office{}, model.ErrInvalid
	}
	var out model.Office
	err := s.run(ctx, p, true, func(q repository.Store, role string) error {
		if !manager(role) {
			return model.ErrForbidden
		}
		var v db.CompanyOffice
		var err error
		if id == uuid.Nil {
			v, err = q.CreateStructureOffice(ctx, db.CreateStructureOfficeParams{
				AccountID: p.Account.ID,
				Name:      in.Name,
				City:      in.City,
				Address:   in.Address,
				Phone:     in.Phone,
				SortOrder: in.SortOrder,
			})
		} else {
			v, err = q.UpdateStructureOffice(ctx, db.UpdateStructureOfficeParams{
				AccountID: p.Account.ID,
				Uuid:      id,
				Name:      in.Name,
				City:      in.City,
				Address:   in.Address,
				Phone:     in.Phone,
				SortOrder: in.SortOrder,
			})
		}
		out = officeModel(v)
		return err
	})
	return out, err
}
func (s *Service) DeleteOffice(ctx context.Context, p authctx.Principal, id uuid.UUID) error {
	return s.run(ctx, p, true, func(q repository.Store, role string) error {
		if !manager(role) {
			return model.ErrForbidden
		}
		n, err := q.DeleteStructureOffice(ctx, db.DeleteStructureOfficeParams{AccountID: p.Account.ID, Uuid: id})
		if err == nil && n == 0 {
			return model.ErrNotFound
		}
		return err
	})
}
func (s *Service) Groups(ctx context.Context, p authctx.Principal, f model.GroupFilter, limit, offset int32) ([]model.Group, int64, error) {
	out := []model.Group{}
	var total int64
	if f.OfficeUUID != nil && f.CompanyWide {
		return out, 0, model.ErrInvalid
	}
	err := s.run(ctx, p, false, func(q repository.Store, role string) error {
		vw, err := loadViewer(ctx, q, p, role)
		if err != nil {
			return err
		}
		rows, err := q.ListStructureGroups(ctx, db.ListStructureGroupsParams{
			AccountID:        p.Account.ID,
			FilterOfficeUuid: nullable(f.OfficeUUID),
			CompanyWide:      f.CompanyWide,
			Restricted:       !vw.all,
			VisibleOffices:   vw.managed,
			VisibleGroups:    vw.groups,
			LimitCount:       limit,
			OffsetCount:      offset,
		})
		if err != nil {
			return err
		}
		for _, v := range rows {
			out = append(out, model.Group{UUID: v.Uuid, GroupInput: model.GroupInput{Name: v.Name, Kind: v.Kind, OfficeUUID: publicID(v.OfficeUuid)}, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time})
		}
		total, err = q.CountStructureGroups(ctx, db.CountStructureGroupsParams{
			AccountID:        p.Account.ID,
			FilterOfficeUuid: nullable(f.OfficeUUID),
			CompanyWide:      f.CompanyWide,
			Restricted:       !vw.all,
			VisibleOffices:   vw.managed,
			VisibleGroups:    vw.groups,
		})
		return err
	})
	return out, total, err
}
func (s *Service) Group(ctx context.Context, p authctx.Principal, id uuid.UUID) (model.Group, error) {
	var out model.Group
	err := s.run(ctx, p, false, func(q repository.Store, role string) error {
		vw, err := loadViewer(ctx, q, p, role)
		if err != nil {
			return err
		}
		if out, err = getGroup(ctx, q, p.Account.ID, id); err != nil {
			return err
		}
		if !vw.seesGroup(out) {
			out = model.Group{}
			return model.ErrNotFound
		}
		return nil
	})
	return out, err
}
func (s *Service) SaveGroup(ctx context.Context, p authctx.Principal, id uuid.UUID, in model.GroupInput) (model.Group, error) {
	in.Name = strings.TrimSpace(in.Name)
	if !validName(in.Name, 150) || (in.Kind != "department" && in.Kind != "team") {
		return model.Group{}, model.ErrInvalid
	}
	var out model.Group
	err := s.run(ctx, p, true, func(q repository.Store, role string) error {
		if err := checkOffice(ctx, q, p.Account.ID, in.OfficeUUID); err != nil {
			return err
		}
		if id != uuid.Nil {
			old, err := getGroup(ctx, q, p.Account.ID, id)
			if err != nil {
				return err
			}
			if err = officeScope(ctx, q, p, role, old.OfficeUUID); err != nil {
				return err
			}
		}
		if err := officeScope(ctx, q, p, role, in.OfficeUUID); err != nil {
			return err
		}
		if id == uuid.Nil {
			v, err := q.CreateStructureGroup(ctx, db.CreateStructureGroupParams{
				AccountID:  p.Account.ID,
				Name:       in.Name,
				Kind:       in.Kind,
				OfficeUuid: nullable(in.OfficeUUID),
			})
			if err != nil {
				return err
			}
			id = v.Uuid
		} else {
			_, err := q.UpdateStructureGroup(ctx, db.UpdateStructureGroupParams{
				AccountID:  p.Account.ID,
				Uuid:       id,
				Name:       in.Name,
				Kind:       in.Kind,
				OfficeUuid: nullable(in.OfficeUUID),
			})
			if err != nil {
				return err
			}
		}
		var err error
		out, err = getGroup(ctx, q, p.Account.ID, id)
		return err
	})
	return out, err
}
func (s *Service) DeleteGroup(ctx context.Context, p authctx.Principal, id uuid.UUID) error {
	return s.run(ctx, p, true, func(q repository.Store, role string) error {
		old, err := getGroup(ctx, q, p.Account.ID, id)
		if err != nil {
			return err
		}
		if err = officeScope(ctx, q, p, role, old.OfficeUUID); err != nil {
			return err
		}
		n, err := q.DeleteStructureGroup(ctx, db.DeleteStructureGroupParams{AccountID: p.Account.ID, Uuid: id})
		if err == nil && n == 0 {
			return model.ErrNotFound
		}
		return err
	})
}
func (s *Service) Assignment(ctx context.Context, p authctx.Principal, user uuid.UUID) (model.Assignment, error) {
	var out model.Assignment
	err := s.run(ctx, p, false, func(q repository.Store, role string) error {
		vw, err := loadViewer(ctx, q, p, role)
		if err != nil {
			return err
		}
		target, err := q.GetAccountMemberByUserUUID(ctx, db.GetAccountMemberByUserUUIDParams{AccountID: p.Account.ID, UserUuid: user})
		if err != nil {
			return err
		}
		v, err := q.GetStructureAssignment(ctx, db.GetStructureAssignmentParams{AccountID: p.Account.ID, Uuid: user})
		if err != nil {
			return err
		}
		if a := assignmentModel(v); vw.seesMember(a, string(target.Role)) {
			out = a
			return nil
		}
		return model.ErrNotFound
	})
	return out, err
}

// Assignments lists the office/group placement of the members the actor may see (viewer) in
// one page. Read rules match Assignment.
func (s *Service) Assignments(ctx context.Context, p authctx.Principal, limit, offset int32) ([]model.Assignment, int64, error) {
	out := []model.Assignment{}
	var total int64
	err := s.run(ctx, p, false, func(q repository.Store, role string) error {
		vw, err := loadViewer(ctx, q, p, role)
		if err != nil {
			return err
		}
		rows, err := q.ListStructureAssignments(ctx, db.ListStructureAssignmentsParams{
			AccountID:         p.Account.ID,
			Restricted:        !vw.all,
			SelfUuid:          vw.self,
			VisibleOffices:    vw.managed,
			IncludeUnassigned: vw.pool,
			LimitCount:        limit,
			OffsetCount:       offset,
		})
		if err != nil {
			return err
		}
		for _, v := range rows {
			out = append(out, model.Assignment{UserUUID: v.UserUuid, OfficeUUID: publicID(v.OfficeUuid), GroupUUID: publicID(v.GroupUuid)})
		}
		total, err = q.CountStructureAssignments(ctx, db.CountStructureAssignmentsParams{
			AccountID:         p.Account.ID,
			Restricted:        !vw.all,
			SelfUuid:          vw.self,
			VisibleOffices:    vw.managed,
			IncludeUnassigned: vw.pool,
		})
		return err
	})
	return out, total, err
}

// SetAssignment places a member. Office managers move members between offices they manage and
// may claim unassigned plain members into them (TEC-948). Every change is written to the
// activity log in the same transaction.
func (s *Service) SetAssignment(ctx context.Context, p authctx.Principal, in model.Assignment) (model.Assignment, error) {
	var out model.Assignment
	err := s.run(ctx, p, true, func(q repository.Store, role string) error {
		target, err := q.GetAccountMemberByUserUUID(ctx, db.GetAccountMemberByUserUUIDParams{AccountID: p.Account.ID, UserUuid: in.UserUUID})
		if err != nil {
			return err
		}
		old, err := q.GetStructureAssignment(ctx, db.GetStructureAssignmentParams{AccountID: p.Account.ID, Uuid: in.UserUUID})
		if err != nil {
			return err
		}
		if err = checkOffice(ctx, q, p.Account.ID, in.OfficeUUID); err != nil {
			return err
		}
		if in.GroupUUID != nil {
			g, err := getGroup(ctx, q, p.Account.ID, *in.GroupUUID)
			if err != nil {
				return err
			}
			if g.OfficeUUID != nil && (in.OfficeUUID == nil || *in.OfficeUUID != *g.OfficeUUID) {
				return model.ErrInvalid
			}
			// Office managers cannot assign an account-wide group, which they do not manage.
			if !manager(role) && g.OfficeUUID == nil {
				return model.ErrForbidden
			}
		}
		if !manager(role) {
			// The unassigned pool holds plain members only; anyone else needs a managed office.
			unassigned := !old.OfficeUuid.Valid && target.Role == db.AccountRoleMember
			if !unassigned {
				if err = officeScope(ctx, q, p, role, publicID(old.OfficeUuid)); err != nil {
					return err
				}
			}
			if err = officeScope(ctx, q, p, role, in.OfficeUUID); err != nil {
				return err
			}
		}
		_, err = q.SetStructureAssignment(ctx, db.SetStructureAssignmentParams{
			AccountID:  p.Account.ID,
			UserUuid:   in.UserUUID,
			OfficeUuid: nullable(in.OfficeUUID),
			GroupUuid:  nullable(in.GroupUUID),
		})
		if err != nil {
			return err
		}
		v, err := q.GetStructureAssignment(ctx, db.GetStructureAssignmentParams{AccountID: p.Account.ID, Uuid: in.UserUUID})
		if err != nil {
			return err
		}
		out = assignmentModel(v)
		return recordAssignment(ctx, q, p, assignmentModel(old), out)
	})
	return out, err
}

// AssignmentActivityAction is the activity log action of a member placement change.
const AssignmentActivityAction = "company.assignment.updated"

func recordAssignment(ctx context.Context, q repository.Store, p authctx.Principal, from, to model.Assignment) error {
	body, err := json.Marshal(map[string]any{
		"account_uuid":     p.Account.UUID,
		"actor_role":       p.Account.Role,
		"from_office_uuid": from.OfficeUUID,
		"from_group_uuid":  from.GroupUUID,
		"office_uuid":      to.OfficeUUID,
		"group_uuid":       to.GroupUUID,
	})
	if err != nil {
		return err
	}
	_, err = q.InsertActivityEvent(ctx, db.InsertActivityEventParams{
		ActorUserID:  pgtype.Int8{Int64: p.UserInternal, Valid: true},
		Action:       AssignmentActivityAction,
		Resource:     "company_member",
		ResourceUuid: pgtype.UUID{Bytes: to.UserUUID, Valid: true},
		Payload:      body,
	})
	return err
}
func (s *Service) Scopes(ctx context.Context, p authctx.Principal, user uuid.UUID) ([]uuid.UUID, error) {
	out := []uuid.UUID{}
	err := s.run(ctx, p, false, func(q repository.Store, role string) error {
		// Office manager grants are visible to owners/admins and to the manager themselves.
		if !manager(role) && user != p.UserID {
			return model.ErrForbidden
		}
		if _, err := q.GetAccountMemberByUserUUID(ctx, db.GetAccountMemberByUserUUIDParams{AccountID: p.Account.ID, UserUuid: user}); err != nil {
			return err
		}
		var err error
		out, err = q.ListStructureScopes(ctx, db.ListStructureScopesParams{AccountID: p.Account.ID, Uuid: user})
		return err
	})
	return out, err
}

// SetScopes replaces an office manager's grants atomically. It never grants a role.
func (s *Service) SetScopes(ctx context.Context, p authctx.Principal, user uuid.UUID, offices []uuid.UUID) error {
	if len(offices) > 100 {
		return model.ErrInvalid
	}
	return s.run(ctx, p, true, func(q repository.Store, role string) error {
		if !manager(role) {
			return model.ErrForbidden
		}
		target, err := q.GetAccountMemberByUserUUID(ctx, db.GetAccountMemberByUserUUIDParams{AccountID: p.Account.ID, UserUuid: user})
		if err != nil {
			return err
		}
		if string(target.Role) != authctx.AccountRoleOfficeManager && len(offices) > 0 {
			return model.ErrInvalid
		}
		for _, id := range offices {
			if err = checkOffice(ctx, q, p.Account.ID, &id); err != nil {
				return err
			}
		}
		if err = q.ClearStructureScopes(ctx, db.ClearStructureScopesParams{AccountID: p.Account.ID, Uuid: user}); err != nil {
			return err
		}
		seen := map[uuid.UUID]bool{}
		for _, id := range offices {
			if seen[id] {
				continue
			}
			seen[id] = true
			n, err := q.AddStructureScope(ctx, db.AddStructureScopeParams{AccountID: p.Account.ID, Uuid: user, Uuid_2: id})
			if err != nil {
				return err
			}
			if n != 1 {
				return model.ErrNotFound
			}
		}
		return nil
	})
}
func (s *Service) Sector(ctx context.Context, p authctx.Principal) (*uuid.UUID, error) {
	var out *uuid.UUID
	err := s.run(ctx, p, false, func(q repository.Store, _ string) error {
		v, err := q.GetCompanySector(ctx, p.Account.ID)
		out = publicID(v)
		return err
	})
	return out, err
}
func (s *Service) SetSector(ctx context.Context, p authctx.Principal, id *uuid.UUID) error {
	return s.run(ctx, p, true, func(q repository.Store, role string) error {
		if !manager(role) {
			return model.ErrForbidden
		}
		n, err := q.SetCompanySector(ctx, db.SetCompanySectorParams{AccountID: p.Account.ID, SectorUuid: nullable(id)})
		if err == nil && n == 0 {
			return model.ErrNotFound
		}
		return err
	})
}

// Catalog uses dedicated platform methods; no company-scope bypass is available here.
type Catalog struct{ q repository.CatalogStore }

func NewCatalog(q repository.CatalogStore) *Catalog { return &Catalog{q: q} }
func sectorModel(v db.Sector) model.Sector {
	return model.Sector{UUID: v.Uuid, SectorInput: model.SectorInput{Slug: v.Slug, NameTR: v.NameTr, NameEN: v.NameEn}, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time}
}
func (s *Catalog) List(ctx context.Context, p authctx.Principal, query string, limit, offset int32) ([]model.Sector, int64, error) {
	if !p.HasPermission(rbac.PermAppCompanyStructureRead) && !p.HasPermission(rbac.PermPlatformSectorsRead) {
		return nil, 0, model.ErrForbidden
	}
	rows, err := s.q.ListSectors(ctx, db.ListSectorsParams{Q: query, LimitCount: limit, OffsetCount: offset})
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.Sector, 0, len(rows))
	for _, v := range rows {
		out = append(out, sectorModel(v))
	}
	total, err := s.q.CountSectors(ctx, query)
	return out, total, err
}
func (s *Catalog) GetAdmin(ctx context.Context, p authctx.Principal, id uuid.UUID) (model.Sector, error) {
	if !p.HasPermission(rbac.PermPlatformSectorsRead) {
		return model.Sector{}, model.ErrForbidden
	}
	v, err := s.q.GetSectorAdmin(ctx, id)
	return sectorModel(v), repository.MapError(err)
}

var sectorSlug = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func (s *Catalog) SaveAdmin(ctx context.Context, p authctx.Principal, id uuid.UUID, in model.SectorInput) (model.Sector, error) {
	if !p.HasPermission(rbac.PermPlatformSectorsWrite) {
		return model.Sector{}, model.ErrForbidden
	}
	in.Slug = strings.TrimSpace(in.Slug)
	in.NameTR = strings.TrimSpace(in.NameTR)
	in.NameEN = strings.TrimSpace(in.NameEN)
	if !sectorSlug.MatchString(in.Slug) || len(in.Slug) > 80 || !validName(in.NameTR, 150) || !validName(in.NameEN, 150) {
		return model.Sector{}, model.ErrInvalid
	}
	var v db.Sector
	var err error
	if id == uuid.Nil {
		v, err = s.q.CreateSectorAdmin(ctx, db.CreateSectorAdminParams{Slug: in.Slug, NameTr: in.NameTR, NameEn: in.NameEN})
	} else {
		v, err = s.q.UpdateSectorAdmin(ctx, db.UpdateSectorAdminParams{Uuid: id, Slug: in.Slug, NameTr: in.NameTR, NameEn: in.NameEN})
	}
	return sectorModel(v), repository.MapError(err)
}
func (s *Catalog) DeleteAdmin(ctx context.Context, p authctx.Principal, id uuid.UUID) error {
	if !p.HasPermission(rbac.PermPlatformSectorsWrite) {
		return model.ErrForbidden
	}
	n, err := s.q.DeleteSectorAdmin(ctx, id)
	if err == nil && n == 0 {
		return model.ErrNotFound
	}
	return repository.MapError(err)
}
