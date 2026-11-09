// Package usecase manages accounts and their memberships (docs/TENANCY_AND_ACCESS.md).
//
// App methods act on the caller's active account and enforce account roles. Platform methods act
// on any account for platform admins; they skip role rules but keep the data invariants (a
// personal account only holds its user; every account keeps an owner), which the database also
// enforces.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/slug"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("account role does not allow this")
	ErrInvalidRequest  = errors.New("invalid request")
	ErrConflict        = errors.New("conflict")
	ErrLastOwner       = errors.New("account must keep at least one owner")
	ErrPersonalAccount = errors.New("personal accounts cannot have other members")
)

type Service struct {
	q       *db.Queries
	search  searchengine.Enqueuer
	invites *Invites
}

// SetInvites lets AddMember send an email invite instead of adding an address that is not
// verified (TEC-1104). Without it such addresses are rejected.
func (s *Service) SetInvites(i *Invites) { s.invites = i }

// SetSearchIndexer keeps company accounts in the Cmd+K index (TEC-595).
func (s *Service) SetSearchIndexer(e searchengine.Enqueuer) { s.search = e }

func (s *Service) indexCompany(ctx context.Context, id uuid.UUID) {
	if s.search != nil {
		s.search.EnqueueUpsert(ctx, "companies", id.String())
	}
}

func New(q *db.Queries) *Service {
	return &Service{q: q}
}

// AccountDetail is an account as seen by a member (Role) or the platform (Role empty).
type AccountDetail struct {
	UUID        uuid.UUID `json:"uuid"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Status      string    `json:"status"`
	Role        string    `json:"role,omitempty"`
	MemberCount int       `json:"member_count"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// PlatformAccountDetail adds the member list for the platform panel.
type PlatformAccountDetail struct {
	AccountDetail
	Members []Member `json:"members"`
}

type Member struct {
	UserUUID  uuid.UUID `json:"user_uuid"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Surname   string    `json:"surname"`
	Role      string    `json:"role"`
	CreatedAt string    `json:"created_at"`
}

// ---- App (active account) ----

// Current returns the caller's active account.
func (s *Service) Current(ctx context.Context, p authctx.Principal) (AccountDetail, error) {
	row, err := s.q.GetAccountByID(ctx, p.Account.ID)
	if err != nil {
		return AccountDetail{}, mapNotFound(err)
	}
	d := mapAccount(row.ID, row.Uuid, row.Type, row.Name, row.Slug, row.Status, row.MemberCount, row.CreatedAt, row.UpdatedAt)
	d.Role = p.Account.Role
	return d, nil
}

// Rename renames the active company account (owner/admin).
func (s *Service) Rename(ctx context.Context, p authctx.Principal, name string) (AccountDetail, error) {
	if p.Account.IsPersonal() {
		return AccountDetail{}, fmt.Errorf("%w: personal accounts follow the user's name", ErrInvalidRequest)
	}
	if !p.Account.CanManageMembers() {
		return AccountDetail{}, ErrForbidden
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return AccountDetail{}, fmt.Errorf("%w: name is required", ErrInvalidRequest)
	}
	if _, err := s.q.UpdateAccount(ctx, db.UpdateAccountParams{ID: p.Account.ID, Name: pgtype.Text{String: name, Valid: true}}); err != nil {
		return AccountDetail{}, err
	}
	s.indexCompany(ctx, p.Account.UUID)
	return s.Current(ctx, p)
}

// Members lists the active account's members (any member).
func (s *Service) Members(ctx context.Context, p authctx.Principal) ([]Member, error) {
	return s.listMembers(ctx, p.Account.ID)
}

// AddMemberResult is either the added member or, for an address that may not be added
// directly, the invite that was sent instead (TEC-1104).
type AddMemberResult struct {
	Member *Member
	Invite *Invite
}

// AddMember adds a user by email to the active company account. Owners may add any role; admins
// may add admins and members.
//
// Only a user whose email is verified is added directly. While self-registration is open anyone
// can register someone else's address, so an unverified user (or an address without a user) gets
// an email invite instead: only the mailbox owner can accept it (TEC-1104). Sending the invite
// needs the app.account_invites.write permission, like POST /v1/account/invites.
func (s *Service) AddMember(ctx context.Context, p authctx.Principal, email, role string) (AddMemberResult, error) {
	if p.Account.IsPersonal() {
		return AddMemberResult{}, ErrPersonalAccount
	}
	if !p.Account.CanManageMembers() {
		return AddMemberResult{}, ErrForbidden
	}
	role, err := parseRole(role, authctx.AccountRoleMember)
	if err != nil {
		return AddMemberResult{}, err
	}
	if role == authctx.AccountRoleOwner && !p.Account.IsOwner() {
		return AddMemberResult{}, ErrForbidden
	}
	user, err := s.q.GetUserForMembershipByEmail(ctx, email)
	switch {
	case err == nil && user.EmailVerified:
		if err := s.q.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: p.Account.ID, UserID: user.ID, Role: db.AccountRole(role)}); err != nil {
			return AddMemberResult{}, mapWriteErr(err)
		}
		m, err := s.memberByUserID(ctx, p.Account.ID, user.ID)
		if err != nil {
			return AddMemberResult{}, err
		}
		return AddMemberResult{Member: &m}, nil
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return AddMemberResult{}, err
	}
	if s.invites == nil {
		return AddMemberResult{}, fmt.Errorf("%w: no user with a verified email address", ErrNotFound)
	}
	if !p.HasPermission(rbac.PermAppAccountInvitesWrite) {
		return AddMemberResult{}, ErrForbidden
	}
	inv, err := s.invites.Create(ctx, p, email, role)
	if err != nil {
		return AddMemberResult{}, err
	}
	return AddMemberResult{Invite: &inv}, nil
}

// ChangeRole changes a member's role in the active company account. Only owners may grant or
// revoke the owner role; admins manage admins and members.
func (s *Service) ChangeRole(ctx context.Context, p authctx.Principal, userUUID uuid.UUID, role string) (Member, error) {
	if p.Account.IsPersonal() {
		return Member{}, ErrPersonalAccount
	}
	if !p.Account.CanManageMembers() {
		return Member{}, ErrForbidden
	}
	role, err := parseRole(role, "")
	if err != nil {
		return Member{}, err
	}
	target, err := s.q.GetAccountMemberByUserUUID(ctx, db.GetAccountMemberByUserUUIDParams{AccountID: p.Account.ID, UserUuid: userUUID})
	if err != nil {
		return Member{}, mapNotFound(err)
	}
	if (string(target.Role) == authctx.AccountRoleOwner || role == authctx.AccountRoleOwner) && !p.Account.IsOwner() {
		return Member{}, ErrForbidden
	}
	return s.setRole(ctx, p.Account.ID, target.UserID, role)
}

// RemoveMember removes a member from the active company account. Any member may leave; removing
// someone else needs owner/admin, and removing an owner needs an owner.
func (s *Service) RemoveMember(ctx context.Context, p authctx.Principal, userUUID uuid.UUID) error {
	if p.Account.IsPersonal() {
		return ErrPersonalAccount
	}
	target, err := s.q.GetAccountMemberByUserUUID(ctx, db.GetAccountMemberByUserUUIDParams{AccountID: p.Account.ID, UserUuid: userUUID})
	if err != nil {
		return mapNotFound(err)
	}
	leaving := target.UserID == p.UserInternal
	if !leaving {
		if !p.Account.CanManageMembers() {
			return ErrForbidden
		}
		if string(target.Role) == authctx.AccountRoleOwner && !p.Account.IsOwner() {
			return ErrForbidden
		}
	}
	return s.removeMember(ctx, p.Account.ID, target.UserID)
}

// ---- Platform ----

// List lists accounts for the platform panel.
func (s *Service) List(ctx context.Context, q, typ, status string, limit, offset int32) ([]AccountDetail, int64, error) {
	params := db.ListAccountsFilteredParams{LimitCount: limit, OffsetCount: offset}
	countParams := db.CountAccountsFilteredParams{}
	if v := strings.TrimSpace(q); v != "" {
		params.Q = pgtype.Text{String: v, Valid: true}
		countParams.Q = params.Q
	}
	if typ != "" {
		t, err := parseType(typ)
		if err != nil {
			return nil, 0, err
		}
		params.Type = db.NullAccountType{AccountType: t, Valid: true}
		countParams.Type = params.Type
	}
	if status != "" {
		st, err := parseStatus(status)
		if err != nil {
			return nil, 0, err
		}
		params.Status = db.NullAccountStatus{AccountStatus: st, Valid: true}
		countParams.Status = params.Status
	}
	rows, err := s.q.ListAccountsFiltered(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountAccountsFiltered(ctx, countParams)
	if err != nil {
		return nil, 0, err
	}
	out := make([]AccountDetail, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAccount(row.ID, row.Uuid, row.Type, row.Name, row.Slug, row.Status, row.MemberCount, row.CreatedAt, row.UpdatedAt))
	}
	return out, total, nil
}

// Get returns an account with its members.
func (s *Service) Get(ctx context.Context, accountUUID uuid.UUID) (PlatformAccountDetail, error) {
	row, err := s.q.GetAccountByUUID(ctx, accountUUID)
	if err != nil {
		return PlatformAccountDetail{}, mapNotFound(err)
	}
	members, err := s.listMembers(ctx, row.ID)
	if err != nil {
		return PlatformAccountDetail{}, err
	}
	return PlatformAccountDetail{
		AccountDetail: mapAccount(row.ID, row.Uuid, row.Type, row.Name, row.Slug, row.Status, row.MemberCount, row.CreatedAt, row.UpdatedAt),
		Members:       members,
	}, nil
}

// CreateCompany creates a company account owned by ownerUserUUID.
func (s *Service) CreateCompany(ctx context.Context, name, rawSlug string, ownerUserUUID uuid.UUID) (PlatformAccountDetail, error) {
	name = strings.TrimSpace(name)
	slugSource := strings.TrimSpace(rawSlug)
	if slugSource == "" {
		slugSource = name
	}
	if name == "" {
		return PlatformAccountDetail{}, fmt.Errorf("%w: name is required", ErrInvalidRequest)
	}
	if ownerUserUUID == uuid.Nil {
		return PlatformAccountDetail{}, fmt.Errorf("%w: owner_user_uuid is required", ErrInvalidRequest)
	}
	ownerID, err := s.q.GetUserIDByUUID(ctx, ownerUserUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlatformAccountDetail{}, fmt.Errorf("%w: owner user not found", ErrNotFound)
		}
		return PlatformAccountDetail{}, err
	}
	account, err := s.q.CreateCompanyAccount(ctx, db.CreateCompanyAccountParams{Name: name, Slug: slug.FromName(slugSource)})
	if err != nil {
		return PlatformAccountDetail{}, mapWriteErr(err)
	}
	if err := s.q.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: account.ID, UserID: ownerID, Role: db.AccountRoleOwner}); err != nil {
		return PlatformAccountDetail{}, mapWriteErr(err)
	}
	s.indexCompany(ctx, account.Uuid)
	return s.Get(ctx, account.Uuid)
}

// Patch updates an account's name and/or status.
func (s *Service) Patch(ctx context.Context, accountUUID uuid.UUID, name, status *string) (PlatformAccountDetail, error) {
	row, err := s.q.GetAccountByUUID(ctx, accountUUID)
	if err != nil {
		return PlatformAccountDetail{}, mapNotFound(err)
	}
	params := db.UpdateAccountParams{ID: row.ID}
	if name != nil {
		v := strings.TrimSpace(*name)
		if v == "" {
			return PlatformAccountDetail{}, fmt.Errorf("%w: name cannot be empty", ErrInvalidRequest)
		}
		params.Name = pgtype.Text{String: v, Valid: true}
	}
	if status != nil {
		st, err := parseStatus(*status)
		if err != nil {
			return PlatformAccountDetail{}, err
		}
		params.Status = db.NullAccountStatus{AccountStatus: st, Valid: true}
	}
	if _, err := s.q.UpdateAccount(ctx, params); err != nil {
		return PlatformAccountDetail{}, err
	}
	s.indexCompany(ctx, accountUUID)
	return s.Get(ctx, accountUUID)
}

// PlatformAddMember adds a user to a company account with any role.
func (s *Service) PlatformAddMember(ctx context.Context, accountUUID, userUUID uuid.UUID, role string) (Member, error) {
	row, err := s.q.GetAccountByUUID(ctx, accountUUID)
	if err != nil {
		return Member{}, mapNotFound(err)
	}
	if row.Type == db.AccountTypePersonal {
		return Member{}, ErrPersonalAccount
	}
	role, err = parseRole(role, authctx.AccountRoleMember)
	if err != nil {
		return Member{}, err
	}
	userID, err := s.q.GetUserIDByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Member{}, fmt.Errorf("%w: user not found", ErrNotFound)
		}
		return Member{}, err
	}
	if err := s.q.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: row.ID, UserID: userID, Role: db.AccountRole(role)}); err != nil {
		return Member{}, mapWriteErr(err)
	}
	return s.memberByUserID(ctx, row.ID, userID)
}

// PlatformChangeRole changes a member's role in any company account.
func (s *Service) PlatformChangeRole(ctx context.Context, accountUUID, userUUID uuid.UUID, role string) (Member, error) {
	row, target, err := s.platformTarget(ctx, accountUUID, userUUID)
	if err != nil {
		return Member{}, err
	}
	role, err = parseRole(role, "")
	if err != nil {
		return Member{}, err
	}
	return s.setRole(ctx, row.ID, target.UserID, role)
}

// PlatformRemoveMember removes a member from any company account.
func (s *Service) PlatformRemoveMember(ctx context.Context, accountUUID, userUUID uuid.UUID) error {
	row, target, err := s.platformTarget(ctx, accountUUID, userUUID)
	if err != nil {
		return err
	}
	return s.removeMember(ctx, row.ID, target.UserID)
}

// ---- helpers ----

func (s *Service) platformTarget(ctx context.Context, accountUUID, userUUID uuid.UUID) (db.GetAccountByUUIDRow, db.GetAccountMemberByUserUUIDRow, error) {
	row, err := s.q.GetAccountByUUID(ctx, accountUUID)
	if err != nil {
		return row, db.GetAccountMemberByUserUUIDRow{}, mapNotFound(err)
	}
	if row.Type == db.AccountTypePersonal {
		return row, db.GetAccountMemberByUserUUIDRow{}, ErrPersonalAccount
	}
	target, err := s.q.GetAccountMemberByUserUUID(ctx, db.GetAccountMemberByUserUUIDParams{AccountID: row.ID, UserUuid: userUUID})
	if err != nil {
		return row, target, mapNotFound(err)
	}
	return row, target, nil
}

func (s *Service) setRole(ctx context.Context, accountID, userID int64, role string) (Member, error) {
	n, err := s.q.UpdateAccountMemberRole(ctx, db.UpdateAccountMemberRoleParams{AccountID: accountID, UserID: userID, Role: db.AccountRole(role)})
	if err != nil {
		return Member{}, mapWriteErr(err)
	}
	if n == 0 {
		return Member{}, ErrNotFound
	}
	return s.memberByUserID(ctx, accountID, userID)
}

// removeMember removes the membership and revokes the person's open invites to the account, so
// an old link cannot bring a removed member back (TEC-1104).
func (s *Service) removeMember(ctx context.Context, accountID, userID int64) error {
	n, err := s.q.RemoveAccountMember(ctx, db.RemoveAccountMemberParams{AccountID: accountID, UserID: userID})
	if err != nil {
		return mapWriteErr(err)
	}
	if n == 0 {
		return ErrNotFound
	}
	if _, err := s.q.RevokeOpenAccountInvitesForUser(ctx, db.RevokeOpenAccountInvitesForUserParams{AccountID: accountID, UserID: userID}); err != nil {
		return fmt.Errorf("revoke open invites: %w", err)
	}
	return nil
}

func (s *Service) listMembers(ctx context.Context, accountID int64) ([]Member, error) {
	rows, err := s.q.ListAccountMembers(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, Member{
			UserUUID: row.UserUuid, Email: row.Email, Name: row.Name, Surname: row.Surname,
			Role: string(row.Role), CreatedAt: formatTime(row.CreatedAt),
		})
	}
	return out, nil
}

func (s *Service) memberByUserID(ctx context.Context, accountID, userID int64) (Member, error) {
	members, err := s.listMembers(ctx, accountID)
	if err != nil {
		return Member{}, err
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return Member{}, err
	}
	for _, m := range members {
		if m.UserUUID == user.Uuid {
			return m, nil
		}
	}
	return Member{}, ErrNotFound
}

func mapAccount(_ int64, id uuid.UUID, typ db.AccountType, name, slug string, status db.AccountStatus, members int32, created, updated pgtype.Timestamptz) AccountDetail {
	return AccountDetail{
		UUID: id, Type: string(typ), Name: name, Slug: slug, Status: string(status),
		MemberCount: int(members), CreatedAt: formatTime(created), UpdatedAt: formatTime(updated),
	}
}

func formatTime(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

func parseRole(v, fallback string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		v = fallback
	}
	switch v {
	case authctx.AccountRoleOwner, authctx.AccountRoleAdmin, authctx.AccountRoleMember, authctx.AccountRoleOfficeManager:
		return v, nil
	default:
		return "", fmt.Errorf("%w: role must be owner, admin, office_manager or member", ErrInvalidRequest)
	}
}

func parseType(v string) (db.AccountType, error) {
	switch db.AccountType(v) {
	case db.AccountTypePersonal, db.AccountTypeCompany:
		return db.AccountType(v), nil
	default:
		return "", fmt.Errorf("%w: type must be personal or company", ErrInvalidRequest)
	}
}

func parseStatus(v string) (db.AccountStatus, error) {
	switch db.AccountStatus(v) {
	case db.AccountStatusActive, db.AccountStatusSuspended:
		return db.AccountStatus(v), nil
	default:
		return "", fmt.Errorf("%w: status must be active or suspended", ErrInvalidRequest)
	}
}

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// mapWriteErr translates the database invariants into domain errors.
func mapWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505":
		return fmt.Errorf("%w: already exists", ErrConflict)
	case "23514":
		if strings.Contains(pgErr.Message, "personal account") {
			return ErrPersonalAccount
		}
		if strings.Contains(pgErr.Message, "at least one owner") {
			return ErrLastOwner
		}
	}
	return err
}
