package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GroupProfile is one profile of a person's swipe group (TEC-1326), in swipe order.
type GroupProfile struct {
	ProfileID   int64
	ProfileUUID uuid.UUID
	Username    string
	// Open is the effective switch: an employee profile the company keeps open is always open.
	Open        bool
	FirstName   string
	LastName    string
	CompanyName string
}

// OpenState is the stored switch of a profile with what decides who may change it.
type OpenState struct {
	IsOpen bool
	// MemberProfile: the profile is linked to a company member (company_member_profiles).
	MemberProfile bool
	// OwnMemberProfile: it is linked to the caller.
	OwnMemberProfile bool
	// AlwaysOpen: the company keeps employee profiles open (account_branding).
	AlwaysOpen bool
}

// ProfileGroupStore reads and writes the open switch and a person's profile group.
type ProfileGroupStore interface {
	// ProfileGroup lists the profiles of the person behind profileID that have an active public
	// page in an active account: their personal account's profiles, then their company profiles.
	// Empty when the profile belongs to no known person (a company profile without a member).
	ProfileGroup(ctx context.Context, profileID int64) ([]GroupProfile, error)
	OpenState(ctx context.Context, accountID, userID int64, profileUUID uuid.UUID) (*OpenState, error)
	SetOpen(ctx context.Context, accountID int64, profileUUID uuid.UUID, open bool) error
	// OwnMemberProfile is the caller's own employee profile in the account (uuid.Nil: none).
	OwnMemberProfile(ctx context.Context, accountID, userID int64) (uuid.UUID, error)
}

var _ ProfileGroupStore = (*Postgres)(nil)

func (r *Postgres) ProfileGroup(ctx context.Context, profileID int64) ([]GroupProfile, error) {
	rows, err := r.q.ListPersonProfileGroup(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("profile group: %w", err)
	}
	out := make([]GroupProfile, 0, len(rows))
	for _, row := range rows {
		out = append(out, GroupProfile{
			ProfileID: row.ProfileID, ProfileUUID: row.ProfileUuid, Username: row.Username, Open: row.IsOpen,
			FirstName: row.FirstName, LastName: row.LastName, CompanyName: row.CompanyName,
		})
	}
	return out, nil
}

func (r *Postgres) OpenState(ctx context.Context, accountID, userID int64, profileUUID uuid.UUID) (*OpenState, error) {
	row, err := r.q.GetProfileOpenState(ctx, db.GetProfileOpenStateParams{AccountID: accountID, UserID: userID, Uuid: profileUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("profile open state: %w", err)
	}
	return &OpenState{
		IsOpen: row.IsOpen, MemberProfile: row.IsMemberProfile,
		OwnMemberProfile: row.IsOwnMemberProfile, AlwaysOpen: row.AlwaysOpen,
	}, nil
}

func (r *Postgres) SetOpen(ctx context.Context, accountID int64, profileUUID uuid.UUID, open bool) error {
	n, err := r.q.SetProfileOpen(ctx, db.SetProfileOpenParams{IsOpen: open, AccountID: accountID, Uuid: profileUUID})
	if err != nil {
		return fmt.Errorf("set profile open: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Postgres) OwnMemberProfile(ctx context.Context, accountID, userID int64) (uuid.UUID, error) {
	id, err := r.q.GetOwnMemberProfileUUID(ctx, db.GetOwnMemberProfileUUIDParams{AccountID: accountID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("own member profile: %w", err)
	}
	return id, nil
}
