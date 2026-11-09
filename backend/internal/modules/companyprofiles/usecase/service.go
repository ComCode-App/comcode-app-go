package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service lists company members with their profiles and links profiles to members.
type Service struct {
	q Store
}

func NewService(q Store) *Service { return &Service{q: q} }

func companyAccount(p authctx.Principal) error {
	if p.Account.ID <= 0 || p.Account.Type != authctx.AccountTypeCompany {
		return model.ErrForbidden
	}
	return nil
}

// MemberProfiles lists the members of the active company account with office, group and the
// linked profile. Any member of the company may read it.
func (s *Service) MemberProfiles(ctx context.Context, p authctx.Principal, f model.MemberProfileFilter) ([]model.MemberProfile, int64, error) {
	if err := companyAccount(p); err != nil {
		return nil, 0, err
	}
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	hasProfile := pgtype.Bool{}
	if f.HasProfile != nil {
		hasProfile = pgtype.Bool{Bool: *f.HasProfile, Valid: true}
	}
	q := strings.TrimSpace(f.Q)
	rows, err := s.q.ListCompanyMemberProfiles(ctx, db.ListCompanyMemberProfilesParams{
		AccountID: p.Account.ID, OfficeUuid: nullUUID(f.OfficeUUID), GroupUuid: nullUUID(f.GroupUUID),
		HasProfile: hasProfile, Q: q, LimitCount: f.Limit, OffsetCount: f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountCompanyMemberProfiles(ctx, db.CountCompanyMemberProfilesParams{
		AccountID: p.Account.ID, OfficeUuid: nullUUID(f.OfficeUUID), GroupUuid: nullUUID(f.GroupUUID),
		HasProfile: hasProfile, Q: q,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.MemberProfile, 0, len(rows))
	for _, r := range rows {
		m := model.MemberProfile{
			UserUUID: r.UserUuid, Email: r.Email, Name: r.Name, Surname: r.Surname, Role: string(r.Role),
			OfficeUUID: publicUUID(r.OfficeUuid), GroupUUID: publicUUID(r.GroupUuid),
		}
		if r.ProfileUuid.Valid {
			m.Profile = &model.LinkedProfileInfo{
				UUID: uuid.UUID(r.ProfileUuid.Bytes), FirstName: r.FirstName, LastName: r.LastName,
				Title: r.Title, AvatarURL: r.AvatarUrl,
			}
		}
		out = append(out, m)
	}
	return out, total, nil
}

// LinkProfile makes profile the member's employee profile; nil removes the link. Owners and
// admins only. A profile already linked to another member is a conflict.
func (s *Service) LinkProfile(ctx context.Context, p authctx.Principal, userUUID uuid.UUID, profile *uuid.UUID) error {
	if err := companyAccount(p); err != nil {
		return err
	}
	a, err := loadActor(ctx, s.q, p.Account.ID, p.UserInternal)
	if err != nil {
		return err
	}
	if !a.isAdmin() {
		return model.ErrForbidden
	}
	member, err := s.q.GetCompanyMemberByUserUUID(ctx, db.GetCompanyMemberByUserUUIDParams{AccountID: p.Account.ID, UserUuid: userUUID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ErrNotFound
		}
		return err
	}
	if profile == nil {
		_, err = s.q.DeleteCompanyMemberProfile(ctx, db.DeleteCompanyMemberProfileParams{AccountID: p.Account.ID, MemberID: member.MemberID})
		return err
	}
	n, err := s.q.SetCompanyMemberProfile(ctx, db.SetCompanyMemberProfileParams{
		AccountID: p.Account.ID, MemberID: member.MemberID, ProfileUuid: *profile,
	})
	if err != nil {
		return mapDBError(err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}
