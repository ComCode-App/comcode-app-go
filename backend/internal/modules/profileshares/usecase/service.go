// Package usecase shares a member's personal profiles with the companies they belong to
// (TEC-1315). Sharing is per profile and per company and off by default. Company owners and
// admins see shared profiles read-only; the share ends with the membership or the profile.
package usecase

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

// Store is the sqlc subset the service needs.
type Store interface {
	GetShareableProfileID(ctx context.Context, arg db.GetShareableProfileIDParams) (int64, error)
	ListProfileShareTargets(ctx context.Context, arg db.ListProfileShareTargetsParams) ([]db.ListProfileShareTargetsRow, error)
	GetCompanyMembershipForUser(ctx context.Context, arg db.GetCompanyMembershipForUserParams) (db.GetCompanyMembershipForUserRow, error)
	InsertProfileCompanyShare(ctx context.Context, arg db.InsertProfileCompanyShareParams) (int64, error)
	DeleteProfileCompanyShare(ctx context.Context, arg db.DeleteProfileCompanyShareParams) (int64, error)
	ListCompanySharedProfiles(ctx context.Context, arg db.ListCompanySharedProfilesParams) ([]db.ListCompanySharedProfilesRow, error)
	CountCompanySharedProfiles(ctx context.Context, companyAccountID int64) (int64, error)
}

// ShareTarget is a company the caller belongs to, and whether the profile is shared with it.
type ShareTarget struct {
	CompanyUUID uuid.UUID `json:"company_uuid"`
	CompanyName string    `json:"company_name"`
	Role        string    `json:"role"`
	Shared      bool      `json:"shared"`
}

// SharedProfile is a member's personal profile as the company sees it (read-only).
type SharedProfile struct {
	UUID           uuid.UUID    `json:"uuid"`
	FirstName      string       `json:"first_name"`
	LastName       string       `json:"last_name"`
	Title          string       `json:"title"`
	CompanyName    string       `json:"company_name"`
	AvatarURL      string       `json:"avatar_url"`
	CompletionRate int          `json:"completion_rate"`
	Username       string       `json:"username"`
	SharedAt       time.Time    `json:"shared_at"`
	Member         SharedMember `json:"member"`
}

// SharedMember is the company member who shared the profile.
type SharedMember struct {
	UserUUID uuid.UUID `json:"user_uuid"`
	Email    string    `json:"email"`
	Name     string    `json:"name"`
	Surname  string    `json:"surname"`
}

type Service struct {
	q        Store
	activity *activity.Recorder
}

func NewService(q Store, rec *activity.Recorder) *Service { return &Service{q: q, activity: rec} }

// profileID resolves a profile of the caller's active personal account.
func (s *Service) profileID(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) (int64, error) {
	if p.Account.ID <= 0 || !p.Account.IsPersonal() {
		return 0, ErrForbidden
	}
	id, err := s.q.GetShareableProfileID(ctx, db.GetShareableProfileIDParams{AccountID: p.Account.ID, ProfileUuid: profileUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// Targets lists the companies the caller belongs to with the share state of the profile.
// Profiles of a company account are not shareable: the list is empty there.
func (s *Service) Targets(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) ([]ShareTarget, error) {
	if !p.Account.IsPersonal() {
		return []ShareTarget{}, nil
	}
	id, err := s.profileID(ctx, p, profileUUID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListProfileShareTargets(ctx, db.ListProfileShareTargetsParams{
		AccountID: p.Account.ID, ProfileID: id, UserID: p.UserInternal,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ShareTarget, 0, len(rows))
	for _, r := range rows {
		out = append(out, ShareTarget{CompanyUUID: r.CompanyUuid, CompanyName: r.CompanyName, Role: string(r.Role), Shared: r.Shared})
	}
	return out, nil
}

// SetShared shares the profile with the company (shared=true) or stops sharing it.
func (s *Service) SetShared(ctx context.Context, p authctx.Principal, profileUUID, companyUUID uuid.UUID, shared bool, r *http.Request) error {
	if !p.Account.CanWrite() {
		return ErrForbidden
	}
	id, err := s.profileID(ctx, p, profileUUID)
	if err != nil {
		return err
	}
	m, err := s.q.GetCompanyMembershipForUser(ctx, db.GetCompanyMembershipForUserParams{CompanyUuid: companyUUID, UserID: p.UserInternal})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var n int64
	action := "profile.company_share_removed"
	if shared {
		action = "profile.company_shared"
		n, err = s.q.InsertProfileCompanyShare(ctx, db.InsertProfileCompanyShareParams{
			AccountID: p.Account.ID, ProfileID: id, CompanyAccountID: m.CompanyAccountID,
			MemberID: m.MemberID, SharedByUserID: p.UserInternal,
		})
	} else {
		n, err = s.q.DeleteProfileCompanyShare(ctx, db.DeleteProfileCompanyShareParams{
			AccountID: p.Account.ID, ProfileID: id, CompanyAccountID: m.CompanyAccountID,
		})
	}
	if err != nil {
		return err
	}
	if n > 0 {
		s.activity.Record(ctx, &p.UserInternal, action, "profiles", &profileUUID, map[string]any{
			"company_uuid": m.CompanyUuid.String(), "company_name": m.CompanyName,
		}, r)
	}
	return nil
}

// CompanyProfiles lists the profiles members shared with the active company account. Owners
// and admins only; other members are forbidden.
func (s *Service) CompanyProfiles(ctx context.Context, p authctx.Principal, limit, offset int32) ([]SharedProfile, int64, error) {
	if p.Account.ID <= 0 || p.Account.Type != authctx.AccountTypeCompany || !p.Account.CanWrite() {
		return nil, 0, ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.ListCompanySharedProfiles(ctx, db.ListCompanySharedProfilesParams{
		CompanyAccountID: p.Account.ID, LimitCount: limit, OffsetCount: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountCompanySharedProfiles(ctx, p.Account.ID)
	if err != nil {
		return nil, 0, err
	}
	out := make([]SharedProfile, 0, len(rows))
	for _, r := range rows {
		out = append(out, SharedProfile{
			UUID: r.ProfileUuid, FirstName: r.FirstName, LastName: r.LastName, Title: r.Title,
			CompanyName: r.CompanyName, AvatarURL: r.AvatarUrl, CompletionRate: int(r.CompletionRate),
			Username: r.Username, SharedAt: r.SharedAt.Time,
			Member: SharedMember{UserUUID: r.MemberUserUuid, Email: r.MemberEmail, Name: r.MemberName, Surname: r.MemberSurname},
		})
	}
	return out, total, nil
}
