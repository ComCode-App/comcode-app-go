package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ImportAdapter imports employees into the active company account (model.ResourceCompanyEmployees).
// A row names a person by email. A registered user becomes a member (role member) if needed, is
// placed in the named office/group, and gets a linked employee profile that is created or
// updated (name, surname, title, work email and phone). An email without a user (TEC-945), or
// whose user has not verified it and is not a member yet (TEC-1104), gets
// the profile at once in the company account and an email invite carrying it and the office/group;
// accepting the invite links the profile to the new member. Owners/admins only. Import-only.
type ImportAdapter struct {
	q        Store
	profiles Profiles
	invites  Inviter
}

// Inviter opens account invites for employees without a user (accounts usecase.Invites).
type Inviter interface {
	// InviteEmployee opens or reuses the open invite of email and attaches profile, office and
	// group; created reports a new invite.
	InviteEmployee(ctx context.Context, accountID, invitedBy int64, email string, profile uuid.UUID, office, group *uuid.UUID) (invite uuid.UUID, created bool, err error)
	// UndoEmployeeInvite deletes a created invite or restores a reused one's attachments.
	UndoEmployeeInvite(ctx context.Context, accountID int64, invite uuid.UUID, created bool, profile, office, group *uuid.UUID) error
}

func NewImportAdapter(q Store, profiles Profiles) *ImportAdapter {
	return &ImportAdapter{q: q, profiles: profiles}
}

// SetInviter enables invites for rows without a user; without it those rows fail with
// user_not_found.
func (a *ImportAdapter) SetInviter(i Inviter) { a.invites = i }

var (
	_ ioengine.AccountScoped  = (*ImportAdapter)(nil)
	_ ioengine.RowPreviewer   = (*ImportAdapter)(nil)
	_ ioengine.SampleProvider = (*ImportAdapter)(nil)
)

func (a *ImportAdapter) Resource() string { return model.ResourceCompanyEmployees }

func (a *ImportAdapter) RequiresAccount() {}

func (a *ImportAdapter) ExportColumns() []ioengine.Column { return nil }

func (a *ImportAdapter) Export(context.Context, ioengine.ExportQuery, i18n.Locale) (ioengine.Dataset, error) {
	return ioengine.Dataset{}, fmt.Errorf("export not supported")
}

func (a *ImportAdapter) ImportSchema() []ioengine.ImportField {
	f := func(key string, required bool) ioengine.ImportField {
		return ioengine.ImportField{Key: key, LabelKey: "company_employees." + key, Type: ioengine.ColumnTypeString, Required: required}
	}
	return []ioengine.ImportField{
		f("name", true), f("surname", true), f("email", true), f("title", false),
		f("office", false), f("group", false), f("phone", false),
	}
}

func (a *ImportAdapter) SampleRows() []map[string]any {
	return []map[string]any{
		{"name": "Ayşe", "surname": "Yılmaz", "email": "ayse@example.com", "title": "Satış Müdürü", "office": "İstanbul", "group": "Satış", "phone": "+90 555 000 00 00"},
	}
}

// Row errors are stable codes (required, too_long, invalid_email, user_not_found and
// user_not_verified (only without an inviter),
// office_not_found, office_ambiguous, group_not_found, group_ambiguous, group_other_office,
// not_allowed) that the UI translates.

// employeeRow is a parsed row and what applying it needs.
type employeeRow struct {
	email, name, surname, title, phone string
	userID                             int64
	userUUID                           uuid.UUID
	memberID                           int64 // 0: not a member yet
	assign                             bool
	office, group                      *uuid.UUID
	profile                            *uuid.UUID // linked live profile (or the open invite's)
	// invite: the email has no user; the row creates or reuses an open invite.
	invite                      bool
	inviteProfile, inviteOffice *uuid.UUID // the open invite's attachments before the row
	inviteGroup                 *uuid.UUID
}

func cell(row map[string]any, key string) string {
	v, ok := row[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// plan validates a row against the account without writing.
func (a *ImportAdapter) plan(ctx context.Context, ac actor, row map[string]any) (employeeRow, []ioengine.RowError) {
	r := employeeRow{
		email: strings.ToLower(cell(row, "email")), name: cell(row, "name"), surname: cell(row, "surname"),
		title: cell(row, "title"), phone: cell(row, "phone"),
	}
	var errs []ioengine.RowError
	bad := func(field, msg string) { errs = append(errs, ioengine.RowError{Field: field, Error: msg}) }
	for _, req := range []struct{ field, v string }{{"name", r.name}, {"surname", r.surname}, {"email", r.email}} {
		if req.v == "" {
			bad(req.field, "required")
		}
	}
	if tooLong(r.name, 100) {
		bad("name", "too_long")
	}
	if tooLong(r.surname, 100) {
		bad("surname", "too_long")
	}
	if tooLong(r.title, 255) {
		bad("title", "too_long")
	}
	if tooLong(r.phone, 50) {
		bad("phone", "too_long")
	}
	if r.email != "" && (!strings.Contains(r.email, "@") || strings.ContainsAny(r.email, " \t") || tooLong(r.email, 255)) {
		bad("email", "invalid_email")
	}
	if len(errs) > 0 {
		return r, errs
	}
	var current db.GetStructureAssignmentRow
	user, err := a.q.FindActiveUserByEmail(ctx, r.email)
	if err == nil && !user.EmailVerified {
		// TEC-1104: an unverified address may belong to someone who registered another person's
		// email, so it is not added directly. It is invited like an unknown address; only the
		// mailbox owner can accept. A user who already is a member stays a member row.
		if _, merr := a.q.GetCompanyMemberByUserID(ctx, db.GetCompanyMemberByUserIDParams{AccountID: ac.accountID, UserID: user.ID}); errors.Is(merr, pgx.ErrNoRows) {
			if a.invites == nil {
				bad("email", "user_not_verified")
				return r, errs
			}
			err = pgx.ErrNoRows
		} else if merr != nil {
			bad("email", merr.Error())
			return r, errs
		}
	}
	switch {
	case errors.Is(err, pgx.ErrNoRows) && a.invites != nil:
		r.invite = true
		inv, err := a.q.GetPendingAccountInviteByEmail(ctx, db.GetPendingAccountInviteByEmailParams{AccountID: ac.accountID, Email: r.email})
		if err == nil {
			r.inviteProfile, r.inviteOffice, r.inviteGroup = publicUUID(inv.ProfileUuid), publicUUID(inv.OfficeUuid), publicUUID(inv.GroupUuid)
			r.profile = r.inviteProfile
			current.OfficeUuid, current.GroupUuid = inv.OfficeUuid, inv.GroupUuid
		} else if !errors.Is(err, pgx.ErrNoRows) {
			bad("email", err.Error())
			return r, errs
		}
	case errors.Is(err, pgx.ErrNoRows):
		bad("email", "user_not_found")
		return r, errs
	case err != nil:
		bad("email", err.Error())
		return r, errs
	}
	if r.invite {
		// No member yet: the office/group go onto the invite.
	} else if m, err := a.q.GetCompanyMemberByUserID(ctx, db.GetCompanyMemberByUserIDParams{AccountID: ac.accountID, UserID: user.ID}); err == nil {
		r.userID, r.userUUID = user.ID, user.Uuid
		r.memberID = m.MemberID
		if link, err := a.q.GetCompanyMemberLink(ctx, db.GetCompanyMemberLinkParams{AccountID: ac.accountID, MemberID: m.MemberID}); err == nil && !link.Deleted {
			id := link.ProfileUuid
			r.profile = &id
		}
		current, _ = a.q.GetStructureAssignment(ctx, db.GetStructureAssignmentParams{AccountID: ac.accountID, Uuid: user.Uuid})
	} else if errors.Is(err, pgx.ErrNoRows) {
		r.userID, r.userUUID = user.ID, user.Uuid
	} else {
		bad("email", err.Error())
		return r, errs
	}
	officeName, groupName := cell(row, "office"), cell(row, "group")
	r.office, r.group = publicUUID(current.OfficeUuid), publicUUID(current.GroupUuid)
	if officeName != "" {
		r.assign = true
		offices, err := a.q.FindCompanyOfficesByName(ctx, db.FindCompanyOfficesByNameParams{AccountID: ac.accountID, Name: officeName})
		switch {
		case err != nil:
			bad("office", err.Error())
		case len(offices) == 0:
			bad("office", "office_not_found")
		case len(offices) > 1:
			bad("office", "office_ambiguous")
		default:
			id := offices[0]
			r.office = &id
		}
	}
	if groupName != "" {
		r.assign = true
		groups, err := a.q.FindCompanyGroupsByName(ctx, db.FindCompanyGroupsByNameParams{AccountID: ac.accountID, Name: groupName})
		if err != nil {
			bad("group", err.Error())
		} else if id, msg := pickGroup(groups, r.office); msg != "" {
			bad("group", msg)
		} else {
			r.group = &id
		}
	} else if r.assign && r.group != nil {
		// A new office keeps the current group only when the group may be used there.
		if g, err := a.q.GetStructureGroup(ctx, db.GetStructureGroupParams{AccountID: ac.accountID, Uuid: *r.group}); err == nil &&
			g.OfficeUuid.Valid && (r.office == nil || uuid.UUID(g.OfficeUuid.Bytes) != *r.office) {
			r.group = nil
		}
	}
	return r, errs
}

// pickGroup chooses the group named in a row: one bound to the row's office wins over a
// company-wide one; groups of other offices never match.
func pickGroup(groups []db.FindCompanyGroupsByNameRow, office *uuid.UUID) (uuid.UUID, string) {
	var inOffice, global []uuid.UUID
	for _, g := range groups {
		switch {
		case !g.OfficeUuid.Valid:
			global = append(global, g.Uuid)
		case office != nil && uuid.UUID(g.OfficeUuid.Bytes) == *office:
			inOffice = append(inOffice, g.Uuid)
		}
	}
	switch {
	case len(inOffice) == 1:
		return inOffice[0], ""
	case len(inOffice) > 1:
		return uuid.Nil, "group_ambiguous"
	case len(global) == 1:
		return global[0], ""
	case len(global) > 1:
		return uuid.Nil, "group_ambiguous"
	case len(groups) > 0:
		return uuid.Nil, "group_other_office"
	default:
		return uuid.Nil, "group_not_found"
	}
}

func joinErrors(errs []ioengine.RowError) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		if e.Field != "" {
			parts = append(parts, e.Field+": "+e.Error)
		} else {
			parts = append(parts, e.Error)
		}
	}
	return strings.Join(parts, "; ")
}

func (a *ImportAdapter) importer(ctx context.Context) (actor, error) {
	ac, _, err := scopedActor(ctx, a.q)
	if err != nil {
		return actor{}, err
	}
	if !ac.isAdmin() {
		return actor{}, model.ErrForbidden
	}
	return ac, nil
}

// PreviewRow is the dry run of ApplyRow.
func (a *ImportAdapter) PreviewRow(ctx context.Context, row map[string]any, _ map[string]any) (string, []ioengine.RowError) {
	ac, err := a.importer(ctx)
	if err != nil {
		return "", []ioengine.RowError{{Error: "not_allowed"}}
	}
	r, errs := a.plan(ctx, ac, row)
	if len(errs) > 0 {
		return "", errs
	}
	if r.profile != nil {
		return "update", nil
	}
	return "create", nil
}

// employeeChange is the before-state of one applied row.
type employeeChange struct {
	UserUUID       uuid.UUID                          `json:"user_uuid"`
	MemberAdded    bool                               `json:"member_added,omitempty"`
	AssignmentSet  bool                               `json:"assignment_set,omitempty"`
	PrevOffice     *uuid.UUID                         `json:"prev_office_uuid,omitempty"`
	PrevGroup      *uuid.UUID                         `json:"prev_group_uuid,omitempty"`
	ProfileCreated bool                               `json:"profile_created,omitempty"`
	ProfileUUID    *uuid.UUID                         `json:"profile_uuid,omitempty"`
	Translations   *[]pmodel.UpsertTranslationInput   `json:"translations,omitempty"`
	ContactDetails *[]pmodel.CreateContactDetailInput `json:"contact_details,omitempty"`
	// Invite rows (no user): the invite the row created or reused and, when reused, its
	// attachments before the row.
	InviteUUID        *uuid.UUID `json:"invite_uuid,omitempty"`
	InviteCreated     bool       `json:"invite_created,omitempty"`
	PrevInviteProfile *uuid.UUID `json:"prev_invite_profile_uuid,omitempty"`
	PrevInviteOffice  *uuid.UUID `json:"prev_invite_office_uuid,omitempty"`
	PrevInviteGroup   *uuid.UUID `json:"prev_invite_group_uuid,omitempty"`
}

func (c employeeChange) toMap() map[string]any {
	b, _ := json.Marshal(c)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

// ApplyRow adds the member, sets the assignment and creates or updates the linked profile. A
// failure part-way undoes the row's earlier steps, so a failed row leaves nothing behind.
func (a *ImportAdapter) ApplyRow(ctx context.Context, row map[string]any, _ map[string]any) (ioengine.RowResult, error) {
	ac, err := a.importer(ctx)
	if err != nil {
		return ioengine.RowResult{OK: false, Error: "not_allowed"}, nil
	}
	r, errs := a.plan(ctx, ac, row)
	if len(errs) > 0 {
		return ioengine.RowResult{OK: false, Error: joinErrors(errs)}, nil
	}
	ch := employeeChange{UserUUID: r.userUUID}
	fail := func(err error) (ioengine.RowResult, error) {
		if undoErr := a.revert(ctx, ac, ch); undoErr != nil {
			return ioengine.RowResult{OK: false, Error: itemError(err) + " (undo failed: " + undoErr.Error() + ")"}, nil
		}
		return ioengine.RowResult{OK: false, Error: itemError(mapDBError(err))}, nil
	}
	if r.invite {
		return a.applyInvite(ctx, ac, r)
	}
	if r.memberID == 0 {
		if err := a.q.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: ac.accountID, UserID: r.userID, Role: db.AccountRoleMember}); err != nil {
			return fail(err)
		}
		ch.MemberAdded = true
		m, err := a.q.GetCompanyMemberByUserID(ctx, db.GetCompanyMemberByUserIDParams{AccountID: ac.accountID, UserID: r.userID})
		if err != nil {
			return fail(err)
		}
		r.memberID = m.MemberID
	}
	if r.assign {
		cur, err := a.q.GetStructureAssignment(ctx, db.GetStructureAssignmentParams{AccountID: ac.accountID, Uuid: r.userUUID})
		if err != nil {
			return fail(err)
		}
		ch.AssignmentSet, ch.PrevOffice, ch.PrevGroup = true, publicUUID(cur.OfficeUuid), publicUUID(cur.GroupUuid)
		if _, err := a.q.SetStructureAssignment(ctx, db.SetStructureAssignmentParams{
			AccountID: ac.accountID, UserUuid: r.userUUID, OfficeUuid: nullUUID(r.office), GroupUuid: nullUUID(r.group),
		}); err != nil {
			return fail(err)
		}
	}
	w := writer(ac)
	op := "update"
	var profileID uuid.UUID
	if r.profile != nil {
		profileID = *r.profile
		detail, err := a.profiles.Get(ctx, w, profileID)
		if err != nil {
			return fail(err)
		}
		in := toUpsert(detail)
		before := toUpsertCopy(in)
		ch.ProfileUUID, ch.Translations, ch.ContactDetails = &profileID, &before.Translations, &before.ContactDetails
		applyEmployee(&in, r)
		if _, err := a.profiles.Update(ctx, w, profileID, in); err != nil {
			return fail(err)
		}
	} else {
		op = "create"
		created, err := a.createProfile(ctx, ac, r)
		if err != nil {
			return fail(err)
		}
		profileID = created
		ch.ProfileCreated, ch.ProfileUUID = true, &profileID
		if _, err := a.q.SetCompanyMemberProfile(ctx, db.SetCompanyMemberProfileParams{
			AccountID: ac.accountID, MemberID: r.memberID, ProfileUuid: profileID,
		}); err != nil {
			return fail(err)
		}
	}
	return ioengine.RowResult{
		OK: true, EntityType: "company_employee", EntityUUID: profileID.String(), Op: op, Previous: ch.toMap(),
	}, nil
}

// applyInvite handles a row whose email has no user: the employee profile is created (or the
// open invite's profile updated) right away, so the card works before the person signs up, and
// the invite carries it with the office/group. One invite email goes out for a new invite.
func (a *ImportAdapter) applyInvite(ctx context.Context, ac actor, r employeeRow) (ioengine.RowResult, error) {
	ch := employeeChange{}
	fail := func(err error) (ioengine.RowResult, error) {
		if undoErr := a.revert(ctx, ac, ch); undoErr != nil {
			return ioengine.RowResult{OK: false, Error: itemError(err) + " (undo failed: " + undoErr.Error() + ")"}, nil
		}
		return ioengine.RowResult{OK: false, Error: itemError(mapDBError(err))}, nil
	}
	w := writer(ac)
	op := "update"
	var profileID uuid.UUID
	if r.profile != nil {
		profileID = *r.profile
		detail, err := a.profiles.Get(ctx, w, profileID)
		if err != nil {
			return fail(err)
		}
		in := toUpsert(detail)
		before := toUpsertCopy(in)
		ch.ProfileUUID, ch.Translations, ch.ContactDetails = &profileID, &before.Translations, &before.ContactDetails
		applyEmployee(&in, r)
		if _, err := a.profiles.Update(ctx, w, profileID, in); err != nil {
			return fail(err)
		}
	} else {
		op = "create"
		created, err := a.createProfile(ctx, ac, r)
		if err != nil {
			return fail(err)
		}
		profileID = created
		ch.ProfileCreated, ch.ProfileUUID = true, &profileID
	}
	invite, created, err := a.invites.InviteEmployee(ctx, ac.accountID, ac.userID, r.email, profileID, r.office, r.group)
	if err != nil {
		return fail(err)
	}
	ch.InviteUUID, ch.InviteCreated = &invite, created
	if !created {
		ch.PrevInviteProfile, ch.PrevInviteOffice, ch.PrevInviteGroup = r.inviteProfile, r.inviteOffice, r.inviteGroup
	}
	return ioengine.RowResult{
		OK: true, EntityType: "company_employee", EntityUUID: profileID.String(), Op: op, Previous: ch.toMap(),
	}, nil
}

// createProfile creates the row's employee profile (company name = account name, locale tr).
func (a *ImportAdapter) createProfile(ctx context.Context, ac actor, r employeeRow) (uuid.UUID, error) {
	in := pmodel.UpsertProfileInput{PrimaryLocale: "tr"}
	if acc, err := a.q.GetAccountByID(ctx, ac.accountID); err == nil {
		in.Translations = []pmodel.UpsertTranslationInput{{Locale: "tr", IsDefault: true, CompanyName: acc.Name}}
	}
	applyEmployee(&in, r)
	created, err := a.profiles.Create(ctx, writer(ac), in)
	if err != nil {
		return uuid.Nil, err
	}
	return created.UUID, nil
}

// applyEmployee writes the row's fields into the profile's primary translation and work contacts.
func applyEmployee(in *pmodel.UpsertProfileInput, r employeeRow) {
	if len(in.Translations) == 0 {
		in.Translations = []pmodel.UpsertTranslationInput{{Locale: "tr", IsDefault: true}}
	}
	idx := 0
	for i, t := range in.Translations {
		if t.IsDefault {
			idx = i
			break
		}
		if t.Locale == in.PrimaryLocale {
			idx = i
		}
	}
	t := &in.Translations[idx]
	t.FirstName, t.LastName = r.name, r.surname
	if r.title != "" {
		t.Title = r.title
	}
	in.ContactDetails = setWorkContact(in.ContactDetails, "email", r.email)
	if r.phone != "" {
		in.ContactDetails = setWorkContact(in.ContactDetails, "phone", r.phone)
	}
}

// RevertRow undoes one applied row.
func (a *ImportAdapter) RevertRow(ctx context.Context, _, _ string, previous map[string]any) error {
	ac, err := a.importer(ctx)
	if err != nil {
		return err
	}
	b, err := json.Marshal(previous)
	if err != nil {
		return err
	}
	var ch employeeChange
	if err := json.Unmarshal(b, &ch); err != nil {
		return err
	}
	return a.revert(ctx, ac, ch)
}

// revert restores the profile, the assignment and the membership recorded in ch, in reverse
// order of ApplyRow.
func (a *ImportAdapter) revert(ctx context.Context, ac actor, ch employeeChange) error {
	w := writer(ac)
	if ch.InviteUUID != nil && a.invites != nil {
		if err := a.invites.UndoEmployeeInvite(ctx, ac.accountID, *ch.InviteUUID, ch.InviteCreated,
			ch.PrevInviteProfile, ch.PrevInviteOffice, ch.PrevInviteGroup); err != nil {
			return err
		}
	}
	if ch.ProfileUUID != nil {
		if ch.ProfileCreated {
			if m, err := a.q.GetCompanyMemberByUserUUID(ctx, db.GetCompanyMemberByUserUUIDParams{AccountID: ac.accountID, UserUuid: ch.UserUUID}); err == nil {
				if _, err := a.q.DeleteCompanyMemberProfile(ctx, db.DeleteCompanyMemberProfileParams{AccountID: ac.accountID, MemberID: m.MemberID}); err != nil {
					return err
				}
			}
			if err := a.profiles.Delete(ctx, w, *ch.ProfileUUID); err != nil {
				return err
			}
		} else if ch.Translations != nil || ch.ContactDetails != nil {
			detail, err := a.profiles.Get(ctx, w, *ch.ProfileUUID)
			if err != nil {
				return err
			}
			in := toUpsert(detail)
			if ch.Translations != nil {
				in.Translations = *ch.Translations
			}
			if ch.ContactDetails != nil {
				in.ContactDetails = *ch.ContactDetails
			}
			if _, err := a.profiles.Update(ctx, w, *ch.ProfileUUID, in); err != nil {
				return err
			}
		}
	}
	if ch.AssignmentSet {
		if _, err := a.q.SetStructureAssignment(ctx, db.SetStructureAssignmentParams{
			AccountID: ac.accountID, UserUuid: ch.UserUUID, OfficeUuid: nullUUID(ch.PrevOffice), GroupUuid: nullUUID(ch.PrevGroup),
		}); err != nil {
			return mapDBError(err)
		}
	}
	if ch.MemberAdded {
		m, err := a.q.GetCompanyMemberByUserUUID(ctx, db.GetCompanyMemberByUserUUIDParams{AccountID: ac.accountID, UserUuid: ch.UserUUID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		// Only a member the import added, and still a plain member, is removed again.
		if string(m.Role) == "member" {
			if _, err := a.q.RemoveAccountMember(ctx, db.RemoveAccountMemberParams{AccountID: ac.accountID, UserID: m.UserID}); err != nil {
				return err
			}
		}
	}
	return nil
}
