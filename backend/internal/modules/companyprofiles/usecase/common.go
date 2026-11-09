// Package usecase implements employee profiles of company accounts (TEC-573): the member-profile
// link, the company_profiles bulk adapter and the employee import adapter.
//
// Profile writes go through the profiles service (validation, completion rate, vCard and route
// cache refresh). That service accepts owner/admin writers only, so the adapters authorize every
// item themselves first (live role and, for office managers, office scope) and then write as an
// admin of the active account for that one profile.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jobscope"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Store is the sqlc surface the module uses (*db.Queries).
type Store interface {
	CompanyActorMembership(ctx context.Context, arg db.CompanyActorMembershipParams) (db.CompanyActorMembershipRow, error)
	CompanyMemberHasOfficeScope(ctx context.Context, arg db.CompanyMemberHasOfficeScopeParams) (bool, error)
	ListCompanyMemberProfiles(ctx context.Context, arg db.ListCompanyMemberProfilesParams) ([]db.ListCompanyMemberProfilesRow, error)
	CountCompanyMemberProfiles(ctx context.Context, arg db.CountCompanyMemberProfilesParams) (int64, error)
	ListCompanyBulkProfileUUIDs(ctx context.Context, arg db.ListCompanyBulkProfileUUIDsParams) ([]uuid.UUID, error)
	GetCompanyProfileTarget(ctx context.Context, arg db.GetCompanyProfileTargetParams) (db.GetCompanyProfileTargetRow, error)
	GetCompanyMemberByUserUUID(ctx context.Context, arg db.GetCompanyMemberByUserUUIDParams) (db.GetCompanyMemberByUserUUIDRow, error)
	GetCompanyMemberByUserID(ctx context.Context, arg db.GetCompanyMemberByUserIDParams) (db.GetCompanyMemberByUserIDRow, error)
	GetCompanyMemberLink(ctx context.Context, arg db.GetCompanyMemberLinkParams) (db.GetCompanyMemberLinkRow, error)
	SetCompanyMemberProfile(ctx context.Context, arg db.SetCompanyMemberProfileParams) (int64, error)
	DeleteCompanyMemberProfile(ctx context.Context, arg db.DeleteCompanyMemberProfileParams) (int64, error)
	FindCompanyOfficesByName(ctx context.Context, arg db.FindCompanyOfficesByNameParams) ([]uuid.UUID, error)
	FindCompanyGroupsByName(ctx context.Context, arg db.FindCompanyGroupsByNameParams) ([]db.FindCompanyGroupsByNameRow, error)
	FindActiveUserByEmail(ctx context.Context, email string) (db.FindActiveUserByEmailRow, error)
	GetPendingAccountInviteByEmail(ctx context.Context, arg db.GetPendingAccountInviteByEmailParams) (db.GetPendingAccountInviteByEmailRow, error)
	GetStructureAssignment(ctx context.Context, arg db.GetStructureAssignmentParams) (db.GetStructureAssignmentRow, error)
	SetStructureAssignment(ctx context.Context, arg db.SetStructureAssignmentParams) (int64, error)
	GetStructureOffice(ctx context.Context, arg db.GetStructureOfficeParams) (db.CompanyOffice, error)
	GetStructureGroup(ctx context.Context, arg db.GetStructureGroupParams) (db.GetStructureGroupRow, error)
	AddAccountMember(ctx context.Context, arg db.AddAccountMemberParams) error
	RemoveAccountMember(ctx context.Context, arg db.RemoveAccountMemberParams) (int64, error)
	GetAccountByID(ctx context.Context, id int64) (db.GetAccountByIDRow, error)
	GetSocialPlatformBySlug(ctx context.Context, slug string) (db.SocialPlatform, error)
	ClearCompanyProfileMedia(ctx context.Context, arg db.ClearCompanyProfileMediaParams) (int64, error)
}

// Profiles is the profiles service surface (*profiles/usecase.Service).
type Profiles interface {
	Get(ctx context.Context, p authctx.Principal, id uuid.UUID) (*pmodel.ProfileDetail, error)
	Create(ctx context.Context, p authctx.Principal, in pmodel.UpsertProfileInput) (*pmodel.ProfileDetail, error)
	Update(ctx context.Context, p authctx.Principal, id uuid.UUID, in pmodel.UpsertProfileInput) (*pmodel.ProfileDetail, error)
	Delete(ctx context.Context, p authctx.Principal, id uuid.UUID) error
	GetPublicPage(ctx context.Context, p authctx.Principal, id uuid.UUID) (*pmodel.PublicPageSettings, error)
	UpdatePublicPage(ctx context.Context, p authctx.Principal, id uuid.UUID, in pmodel.UpdatePublicPageInput) (*pmodel.PublicPageSettings, error)
}

// actor is the caller's live membership in the company.
type actor struct {
	accountID   int64
	accountUUID uuid.UUID
	userID    int64
	userUUID  uuid.UUID
	memberID  int64
	role      string
}

func (a actor) isAdmin() bool {
	return a.role == authctx.AccountRoleOwner || a.role == authctx.AccountRoleAdmin
}

func (a actor) isOfficeManager() bool { return a.role == authctx.AccountRoleOfficeManager }

// loadActor reloads the actor's role from the database. A missing membership, a personal
// account or a plain member role is ErrForbidden.
func loadActor(ctx context.Context, q Store, accountID, userID int64) (actor, error) {
	if accountID <= 0 || userID <= 0 {
		return actor{}, model.ErrForbidden
	}
	m, err := q.CompanyActorMembership(ctx, db.CompanyActorMembershipParams{AccountID: accountID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actor{}, model.ErrForbidden
		}
		return actor{}, err
	}
	a := actor{accountID: accountID, accountUUID: m.AccountUuid, userID: userID, userUUID: m.UserUuid, memberID: m.MemberID, role: string(m.Role)}
	if !a.isAdmin() && !a.isOfficeManager() {
		return actor{}, model.ErrForbidden
	}
	return a, nil
}

// scopedActor loads the actor of the job on ctx; no scope fails closed.
func scopedActor(ctx context.Context, q Store) (actor, jobscope.Scope, error) {
	scope, ok := jobscope.From(ctx)
	if !ok {
		return actor{}, jobscope.Scope{}, model.ErrForbidden
	}
	a, err := loadActor(ctx, q, scope.AccountID, scope.ActorID)
	return a, scope, err
}

// canTouchProfile reports whether a may change the given live profile of its company: any
// profile for owners/admins, for office managers only profiles linked to a member assigned to
// one of their offices.
func canTouchProfile(ctx context.Context, q Store, a actor, profile uuid.UUID) error {
	target, err := q.GetCompanyProfileTarget(ctx, db.GetCompanyProfileTargetParams{AccountID: a.accountID, ProfileUuid: profile})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ErrNotFound
		}
		return err
	}
	if a.isAdmin() {
		return nil
	}
	if !a.isOfficeManager() || !target.MemberID.Valid || !target.OfficeID.Valid {
		return model.ErrForbidden
	}
	ok, err := q.CompanyMemberHasOfficeScope(ctx, db.CompanyMemberHasOfficeScopeParams{
		AccountID: a.accountID, MemberID: a.memberID, OfficeID: target.OfficeID.Int64,
	})
	if err != nil {
		return err
	}
	if !ok {
		return model.ErrForbidden
	}
	return nil
}

// writer is the principal profile writes run under once the actor was authorized for the
// profile (see the package doc).
func writer(a actor) authctx.Principal {
	return authctx.Principal{
		UserID:       a.userUUID,
		UserInternal: a.userID,
		Permissions:  []string{rbac.PermAppProfilesRead, rbac.PermAppProfilesWrite},
		Account: authctx.Account{
			ID: a.accountID, UUID: a.accountUUID, Type: authctx.AccountTypeCompany, Role: authctx.AccountRoleAdmin,
		},
	}
}

// toUpsert turns a stored profile into the full-replacement update payload.
func toUpsert(d *pmodel.ProfileDetail) pmodel.UpsertProfileInput {
	in := pmodel.UpsertProfileInput{
		PrimaryLocale: d.PrimaryLocale, Birthday: d.Birthday, Gender: d.Gender,
		AvatarURL: d.AvatarURL, CoverURL: d.CoverURL, IsPrimary: d.IsPrimary,
		Translations:   make([]pmodel.UpsertTranslationInput, 0, len(d.Translations)),
		Addresses:      make([]pmodel.CreateAddressInput, 0, len(d.Addresses)),
		ContactDetails: make([]pmodel.CreateContactDetailInput, 0, len(d.ContactDetails)),
		SocialLinks:    make([]pmodel.CreateSocialLinkInput, 0, len(d.SocialLinks)),
	}
	for _, t := range d.Translations {
		in.Translations = append(in.Translations, pmodel.UpsertTranslationInput(t))
	}
	for _, a := range d.Addresses {
		in.Addresses = append(in.Addresses, pmodel.CreateAddressInput{
			Type: a.Type, Label: a.Label, Street: a.Street, Extended: a.Extended, City: a.City,
			Region: a.Region, PostalCode: a.PostalCode, Country: a.Country,
			IsPrimary: a.IsPrimary, DisplayOrder: a.DisplayOrder,
		})
	}
	for _, c := range d.ContactDetails {
		in.ContactDetails = append(in.ContactDetails, pmodel.CreateContactDetailInput{
			Category: c.Category, Type: c.Type, Label: c.Label, Value: c.Value,
			IsPrimary: c.IsPrimary, DisplayOrder: c.DisplayOrder,
		})
	}
	for _, l := range d.SocialLinks {
		in.SocialLinks = append(in.SocialLinks, pmodel.CreateSocialLinkInput{
			PlatformID: l.PlatformID, Platform: l.Platform, Label: l.Label, URL: l.URL,
			IconURL: l.IconURL, DisplayOrder: l.DisplayOrder, IsActive: l.IsActive,
		})
	}
	return in
}

func nullUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func publicUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	v := uuid.UUID(id.Bytes)
	return &v
}

// mapDBError turns constraint violations into model errors.
func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23503", "23505":
			return fmt.Errorf("%w: %s", model.ErrConflict, pg.Message)
		case "23514", "22001":
			return fmt.Errorf("%w: %s", model.ErrInvalid, pg.Message)
		}
	}
	return err
}

// itemError is the message of a failed bulk item or import row: a stable code (not_allowed,
// not_found, no_office, ...) the UI translates, else the underlying validation message.
func itemError(err error) string {
	switch {
	case errors.Is(err, model.ErrForbidden):
		return "not_allowed"
	case errors.Is(err, model.ErrNotFound):
		return "not_found"
	default:
		return strings.TrimPrefix(err.Error(), model.ErrInvalid.Error()+": ")
	}
}
