package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/password"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
)

type memRepo struct {
	users     map[int64]model.User
	byEmail   map[string]model.User
	byUUID    map[uuid.UUID]model.User
	userRoles map[int64][]string
	nextID    int64
	perms     map[string][]string
	roles     map[string]int64

	sessions    []*memSession
	accounts    map[int64]*memAccount
	nextAccount int64

	// otps maps email+"|"+type to the active code hash (nil: no active OTP).
	otps map[string]string
}

type memSession struct {
	row     model.RefreshSession
	hash    string
	revoked bool
	expires time.Time
}

type memAccount struct {
	membership model.AccountMembership
	personal   int64 // personal_user_id; 0 for company accounts
	members    map[int64]string
}

func newMemRepo() *memRepo {
	r := &memRepo{
		users:     map[int64]model.User{},
		byEmail:   map[string]model.User{},
		byUUID:    map[uuid.UUID]model.User{},
		userRoles: map[int64][]string{},
		perms: map[string][]string{
			rbac.RoleSuperAdmin: {
				rbac.PermPlatformUsersRead, rbac.PermPlatformUsersWrite,
				rbac.PermPlatformRolesRead, rbac.PermPlatformRolesWrite,
				rbac.PermAuthSession,
			},
			rbac.RoleSubscriber: {rbac.PermAuthSession, rbac.PermNotificationsRead},
		},
		roles:    map[string]int64{rbac.RoleSuperAdmin: 1, rbac.RoleSubscriber: 2},
		accounts: map[int64]*memAccount{},
	}
	return r
}

func (r *memRepo) CreateUser(_ context.Context, u model.User, emailVerified bool) (model.User, error) {
	r.nextID++
	u.ID = r.nextID
	u.UUID = uuid.New()
	u.PasswordSet = true // DB default
	if emailVerified {
		u.EmailVerified = true
	}
	r.users[u.ID] = u
	r.byEmail[u.Email] = u
	r.byUUID[u.UUID] = u
	return u, nil
}

func (r *memRepo) FindUserByEmail(_ context.Context, email string) (model.User, error) {
	u, ok := r.byEmail[email]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	return u, nil
}

func (r *memRepo) FindUserByUUID(_ context.Context, id uuid.UUID) (model.User, error) {
	u, ok := r.byUUID[id]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	return u, nil
}

func (r *memRepo) FindUserByID(_ context.Context, id int64) (model.User, error) {
	u, ok := r.users[id]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	return u, nil
}

func (r *memRepo) UpdateLastLogin(context.Context, int64) error { return nil }

func (r *memRepo) UpdatePassword(_ context.Context, userID int64, hash string) error {
	u, ok := r.users[userID]
	if !ok {
		return repository.ErrNotFound
	}
	u.PasswordHash = hash
	u.PasswordSet = true
	r.users[userID] = u
	r.byEmail[u.Email] = u
	r.byUUID[u.UUID] = u
	return nil
}

func (r *memRepo) MarkPasswordUnset(_ context.Context, userID int64) error {
	u, ok := r.users[userID]
	if !ok {
		return repository.ErrNotFound
	}
	u.PasswordSet = false
	r.users[userID] = u
	r.byEmail[u.Email] = u
	r.byUUID[u.UUID] = u
	return nil
}

func (r *memRepo) UpdateProfile(_ context.Context, userUUID uuid.UUID, name, surname, locale *string) (model.User, error) {
	u, ok := r.byUUID[userUUID]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	if name != nil {
		u.Name = *name
	}
	if surname != nil {
		u.Surname = *surname
	}
	if locale != nil {
		u.Locale = *locale
	}
	r.users[u.ID] = u
	r.byEmail[u.Email] = u
	r.byUUID[u.UUID] = u
	return u, nil
}

func (r *memRepo) UpdateUserPlatform(_ context.Context, id uuid.UUID, name, surname, status *string) (model.User, error) {
	u, ok := r.byUUID[id]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	if name != nil {
		u.Name = *name
	}
	if surname != nil {
		u.Surname = *surname
	}
	if status != nil {
		u.Status = *status
	}
	r.users[u.ID] = u
	r.byEmail[u.Email] = u
	r.byUUID[u.UUID] = u
	return u, nil
}

func (r *memRepo) ListUsersFiltered(_ context.Context, _, _ int32, _, _, _ string) ([]model.User, int64, error) {
	out := make([]model.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, u)
	}
	return out, int64(len(out)), nil
}

func (r *memRepo) CountUsersWithRole(_ context.Context, roleSlug string) (int64, error) {
	var n int64
	for userID, roles := range r.userRoles {
		for _, role := range roles {
			if role == roleSlug && r.users[userID].Status != "disabled" {
				n++
			}
		}
	}
	return n, nil
}

func (r *memRepo) UpsertSuperAdmin(_ context.Context, email, name, surname, hash string) (model.User, bool, error) {
	if u, err := r.FindUserByEmail(context.Background(), email); err == nil {
		u.PasswordHash = hash
		u.Name = name
		u.Surname = surname
		u.Status = "active"
		r.users[u.ID] = u
		r.byEmail[u.Email] = u
		r.byUUID[u.UUID] = u
		r.userRoles[u.ID] = []string{rbac.RoleSuperAdmin}
		return u, false, nil
	}
	u, err := r.CreateUser(context.Background(), model.User{
		Email: email, PasswordHash: hash, Name: name, Surname: surname, Status: "active",
	}, true)
	if err != nil {
		return model.User{}, false, err
	}
	r.userRoles[u.ID] = []string{rbac.RoleSuperAdmin}
	return u, true, nil
}

func (r *memRepo) GetRoleIDBySlug(_ context.Context, slug string) (int64, error) {
	id, ok := r.roles[slug]
	if !ok {
		return 0, repository.ErrNotFound
	}
	return id, nil
}

func (r *memRepo) ListPermissionsByRoleSlug(_ context.Context, slug string) ([]string, error) {
	return r.perms[slug], nil
}

func (r *memRepo) ListUserRoleSlugs(_ context.Context, userID int64) ([]string, error) {
	return r.userRoles[userID], nil
}

func (r *memRepo) UserHasRoleSlug(_ context.Context, userID int64, slug string) (bool, error) {
	for _, role := range r.userRoles[userID] {
		if role == slug {
			return true, nil
		}
	}
	return false, nil
}

func (r *memRepo) ListUserRolesByUserUUID(_ context.Context, userUUID uuid.UUID) ([]model.RoleSummary, error) {
	u, err := r.FindUserByUUID(context.Background(), userUUID)
	if err != nil {
		return nil, err
	}
	out := make([]model.RoleSummary, 0)
	for _, slug := range r.userRoles[u.ID] {
		out = append(out, model.RoleSummary{UUID: uuid.New(), Name: slug, Slug: slug, IsSystem: slug == rbac.RoleSuperAdmin})
	}
	return out, nil
}

func (r *memRepo) ListRolesForUserIDs(ctx context.Context, userIDs []int64) (map[int64][]model.RoleSummary, error) {
	out := make(map[int64][]model.RoleSummary, len(userIDs))
	for _, id := range userIDs {
		u, err := r.FindUserByID(ctx, id)
		if err != nil {
			continue
		}
		roles, err := r.ListUserRolesByUserUUID(ctx, u.UUID)
		if err != nil {
			return nil, err
		}
		out[id] = roles
	}
	return out, nil
}

func (r *memRepo) ListOAuthAccountsForUserIDs(ctx context.Context, userIDs []int64) (map[int64][]model.OAuthAccountRecord, error) {
	out := make(map[int64][]model.OAuthAccountRecord, len(userIDs))
	for _, id := range userIDs {
		rows, err := r.ListOAuthAccountsByUserID(ctx, id)
		if err != nil {
			return nil, err
		}
		out[id] = rows
	}
	return out, nil
}

func (r *memRepo) ListPasskeysForUserIDs(ctx context.Context, userIDs []int64) (map[int64][]model.PasskeyRecord, error) {
	out := make(map[int64][]model.PasskeyRecord, len(userIDs))
	for _, id := range userIDs {
		rows, err := r.ListPasskeysByUserID(ctx, id)
		if err != nil {
			return nil, err
		}
		out[id] = rows
	}
	return out, nil
}

func (r *memRepo) ReplaceUserRoles(_ context.Context, userID int64, roleIDs []int64) error {
	slugs := make([]string, 0, len(roleIDs))
	for slug, id := range r.roles {
		for _, rid := range roleIDs {
			if rid == id {
				slugs = append(slugs, slug)
			}
		}
	}
	r.userRoles[userID] = slugs
	return nil
}

func (r *memRepo) ResolveRoleIDsByUUIDs(_ context.Context, roleUUIDs []uuid.UUID) ([]int64, error) {
	return []int64{2}, nil
}

func (r *memRepo) SaveRefresh(_ context.Context, userID int64, hash string, exp time.Time, meta model.SessionMeta) (model.RefreshSession, error) {
	row := model.RefreshSession{
		UUID: uuid.New(), FamilyUUID: uuid.New(), UserID: userID,
		ImpersonatorUserID: meta.ImpersonatorUserID, ActiveAccountID: meta.ActiveAccountID,
	}
	r.sessions = append(r.sessions, &memSession{row: row, hash: hash, expires: exp})
	return row, nil
}

func (r *memRepo) liveSession(match func(*memSession) bool) *memSession {
	for _, s := range r.sessions {
		if !s.revoked && time.Now().Before(s.expires) && match(s) {
			return s
		}
	}
	return nil
}

func (r *memRepo) RotateRefresh(_ context.Context, oldHash, newHash string, exp time.Time, _ model.SessionMeta) (model.RefreshSession, error) {
	old := r.liveSession(func(s *memSession) bool { return s.hash == oldHash })
	if old == nil {
		return model.RefreshSession{}, repository.ErrNotFound
	}
	old.revoked = true
	row := old.row
	row.UUID = uuid.New()
	r.sessions = append(r.sessions, &memSession{row: row, hash: newHash, expires: exp})
	return row, nil
}

func (r *memRepo) RotateRefreshFamilyWithAccount(_ context.Context, family uuid.UUID, userID int64, newHash string, exp time.Time, accountID int64) (model.RefreshSession, error) {
	old := r.liveSession(func(s *memSession) bool { return s.row.FamilyUUID == family && s.row.UserID == userID })
	if old == nil {
		return model.RefreshSession{}, repository.ErrNotFound
	}
	old.revoked = true
	row := old.row
	row.UUID = uuid.New()
	row.ActiveAccountID = &accountID
	r.sessions = append(r.sessions, &memSession{row: row, hash: newHash, expires: exp})
	return row, nil
}

func (r *memRepo) GetActiveSessionFamily(_ context.Context, family uuid.UUID, userID int64) (model.RefreshSession, error) {
	s := r.liveSession(func(s *memSession) bool { return s.row.FamilyUUID == family && s.row.UserID == userID })
	if s == nil {
		return model.RefreshSession{}, repository.ErrNotFound
	}
	return s.row, nil
}

func (r *memRepo) ListActiveSessions(context.Context, int64) ([]model.DeviceSession, error) {
	return nil, nil
}

func (r *memRepo) revokeWhere(match func(*memSession) bool) int {
	n := 0
	for _, s := range r.sessions {
		if !s.revoked && match(s) {
			s.revoked = true
			n++
		}
	}
	return n
}

func (r *memRepo) RevokeSession(_ context.Context, userID int64, family uuid.UUID) error {
	if r.revokeWhere(func(s *memSession) bool { return s.row.UserID == userID && s.row.FamilyUUID == family }) == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *memRepo) RevokeOtherSessions(_ context.Context, userID int64, keep uuid.UUID) error {
	r.revokeWhere(func(s *memSession) bool { return s.row.UserID == userID && s.row.FamilyUUID != keep })
	return nil
}

func (r *memRepo) RevokeRefresh(_ context.Context, hash string) error {
	var family uuid.UUID
	for _, s := range r.sessions {
		if s.hash == hash {
			family = s.row.FamilyUUID
		}
	}
	r.revokeWhere(func(s *memSession) bool { return s.row.FamilyUUID == family })
	return nil
}

func (r *memRepo) RevokeAllRefresh(_ context.Context, userID int64) error {
	r.revokeWhere(func(s *memSession) bool { return s.row.UserID == userID })
	return nil
}

// personalAccount emulates the users insert trigger: every user has a personal account.
func (r *memRepo) personalAccount(userID int64) *memAccount {
	for _, a := range r.accounts {
		if a.personal == userID {
			return a
		}
	}
	r.nextAccount++
	a := &memAccount{
		membership: model.AccountMembership{
			ID: r.nextAccount, UUID: uuid.New(), Type: "personal", Name: "personal", Status: "active",
		},
		personal: userID,
		members:  map[int64]string{userID: "owner"},
	}
	r.accounts[a.membership.ID] = a
	return a
}

// addCompanyAccount creates a company account with the given members (test helper).
func (r *memRepo) addCompanyAccount(name string, members map[int64]string) model.AccountMembership {
	r.nextAccount++
	a := &memAccount{
		membership: model.AccountMembership{ID: r.nextAccount, UUID: uuid.New(), Type: "company", Name: name, Status: "active"},
		members:    members,
	}
	r.accounts[a.membership.ID] = a
	return a.membership
}

func (r *memRepo) membershipOf(a *memAccount, userID int64) (model.AccountMembership, error) {
	role, ok := a.members[userID]
	if !ok {
		return model.AccountMembership{}, repository.ErrNotFound
	}
	m := a.membership
	m.Role = role
	return m, nil
}

func (r *memRepo) GetPersonalAccount(_ context.Context, userID int64) (model.AccountMembership, error) {
	return r.membershipOf(r.personalAccount(userID), userID)
}

func (r *memRepo) GetAccountMembershipByID(_ context.Context, accountID, userID int64) (model.AccountMembership, error) {
	a, ok := r.accounts[accountID]
	if !ok {
		return model.AccountMembership{}, repository.ErrNotFound
	}
	return r.membershipOf(a, userID)
}

func (r *memRepo) GetAccountMembershipByUUID(_ context.Context, accountUUID uuid.UUID, userID int64) (model.AccountMembership, error) {
	for _, a := range r.accounts {
		if a.membership.UUID == accountUUID {
			return r.membershipOf(a, userID)
		}
	}
	return model.AccountMembership{}, repository.ErrNotFound
}

func (r *memRepo) ListAccountMemberships(_ context.Context, userID int64) ([]model.AccountMembership, error) {
	r.personalAccount(userID)
	var out []model.AccountMembership
	for _, a := range r.accounts {
		if m, err := r.membershipOf(a, userID); err == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *memRepo) CreateOTP(context.Context, *int64, string, string, string, time.Time) error {
	return nil
}
func (r *memRepo) InvalidateOTPs(context.Context, string, string) error { return nil }
func (r *memRepo) GetActiveOTP(_ context.Context, email, otpType string) (int64, string, int32, int32, error) {
	if hash, ok := r.otps[email+"|"+otpType]; ok {
		return 1, hash, 0, 5, nil
	}
	return 0, "", 0, 0, repository.ErrNotFound
}
func (r *memRepo) IncrementOTPAttempts(context.Context, int64) (int32, int32, error) {
	return 0, 0, nil
}
func (r *memRepo) ConsumeOTP(context.Context, int64) error { return nil }
func (r *memRepo) SetEmailVerified(_ context.Context, userID int64) (model.User, error) {
	u, ok := r.users[userID]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	u.EmailVerified = true
	r.users[userID] = u
	return u, nil
}

func (r *memRepo) GetPasskeyByCredentialID(context.Context, string) (model.PasskeyRecord, error) {
	return model.PasskeyRecord{}, repository.ErrNotFound
}
func (r *memRepo) ListPasskeysByUserID(context.Context, int64) ([]model.PasskeyRecord, error) {
	return nil, nil
}
func (r *memRepo) CreatePasskey(context.Context, int64, model.CreateAdapterAuthenticatorInput) (model.PasskeyRecord, error) {
	return model.PasskeyRecord{}, nil
}
func (r *memRepo) UpdatePasskeyCounter(context.Context, string, int64) error { return nil }
func (r *memRepo) UpdatePasskeyName(context.Context, uuid.UUID, uuid.UUID, *string) (model.PasskeyRecord, error) {
	return model.PasskeyRecord{}, repository.ErrNotFound
}
func (r *memRepo) DeletePasskeyByCredentialID(context.Context, string) error       { return nil }
func (r *memRepo) DeletePasskeyByUUID(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (r *memRepo) GetOAuthAccountByProviderAccount(context.Context, string, string) (model.OAuthAccountRecord, error) {
	return model.OAuthAccountRecord{}, repository.ErrNotFound
}
func (r *memRepo) GetOAuthAccountByUserProvider(context.Context, int64, string) (model.OAuthAccountRecord, error) {
	return model.OAuthAccountRecord{}, repository.ErrNotFound
}
func (r *memRepo) ListOAuthAccountsByUserID(context.Context, int64) ([]model.OAuthAccountRecord, error) {
	return nil, nil
}
func (r *memRepo) CreateOAuthAccount(context.Context, model.CreateOAuthAccountInput) (model.OAuthAccountRecord, error) {
	return model.OAuthAccountRecord{}, nil
}
func (r *memRepo) DeleteOAuthAccountByProviderAccount(context.Context, string, string) error {
	return nil
}
func (r *memRepo) DeleteOAuthAccountByUserProvider(context.Context, int64, string) error {
	return repository.ErrNotFound
}
func (r *memRepo) GetUserTOTP(context.Context, int64) (model.UserTOTP, error) {
	return model.UserTOTP{}, repository.ErrNotFound
}
func (r *memRepo) UpsertUserTOTPSetup(context.Context, int64, string) (model.UserTOTP, error) {
	return model.UserTOTP{}, nil
}
func (r *memRepo) ConfirmUserTOTP(context.Context, int64, []string) (model.UserTOTP, error) {
	return model.UserTOTP{}, nil
}
func (r *memRepo) UpdateUserTOTPRecoveryHashes(context.Context, int64, []string) error { return nil }
func (r *memRepo) DeleteUserTOTP(context.Context, int64) error                         { return nil }

func (r *memRepo) ListRolesFiltered(context.Context, int32, int32, string) ([]model.RoleSummary, int64, error) {
	return []model.RoleSummary{{UUID: uuid.New(), Name: "Subscriber", Slug: rbac.RoleSubscriber}}, 1, nil
}
func (r *memRepo) GetRoleByUUID(context.Context, uuid.UUID) (model.RoleSummary, error) {
	return model.RoleSummary{UUID: uuid.New(), Slug: rbac.RoleSubscriber}, nil
}
func (r *memRepo) ListRolePermissionSlugs(context.Context, uuid.UUID) ([]string, error) {
	return []string{rbac.PermAuthSession}, nil
}
func (r *memRepo) CreateRole(_ context.Context, name, slug string, description *string) (model.RoleSummary, error) {
	return model.RoleSummary{UUID: uuid.New(), Name: name, Slug: slug}, nil
}
func (r *memRepo) UpdateRole(context.Context, uuid.UUID, *string, *string) (model.RoleSummary, error) {
	return model.RoleSummary{}, nil
}
func (r *memRepo) DeleteRole(context.Context, uuid.UUID) error               { return nil }
func (r *memRepo) SetRolePermissions(context.Context, int64, []string) error { return nil }
func (r *memRepo) ListPermissionsFiltered(context.Context, int32, int32, string) ([]model.PermissionSummary, int64, error) {
	return []model.PermissionSummary{{Slug: rbac.PermAuthSession, Name: "Auth session"}}, 1, nil
}

func TestRegisterLoginMeWithoutRoles(t *testing.T) {
	repo := newMemRepo()
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uc := New(repo, tokens)
	uc.SetAuthSettings(testAuthGate{
		passwordRegister: true,
		passwordLogin:    true,
		passkeyLogin:     true,
		registration:     true,
	})

	user, err := uc.Register(context.Background(), model.RegisterInput{
		Email: "user@example.com", Password: "Password1!", Name: "Test", Surname: "User",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err = uc.Login(context.Background(), "user@example.com", "Password1!", "", model.SessionMeta{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	me, err := uc.Me(context.Background(), user.UUID, nil, 0)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if len(me.Roles) != 0 {
		t.Fatalf("roles = %v", me.Roles)
	}
}

type testAuthGate struct {
	passwordRegister bool
	passwordLogin    bool
	passkeyLogin     bool
	registration     bool
	defaultRoleID    *int64
}

func (g testAuthGate) CanPasswordRegister(context.Context) (bool, error) {
	return g.passwordRegister && g.registration, nil
}
func (g testAuthGate) CanPasswordLogin(context.Context) (bool, error) {
	return g.passwordLogin, nil
}
func (g testAuthGate) CanPasskeyLogin(context.Context) (bool, error) {
	return g.passkeyLogin, nil
}
func (g testAuthGate) RegistrationEnabled(context.Context) (bool, error) {
	return g.registration, nil
}
func (g testAuthGate) DefaultRoleID(context.Context) (*int64, error) {
	return g.defaultRoleID, nil
}

func TestSuperAdminLogin(t *testing.T) {
	repo := newMemRepo()
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uc := New(repo, tokens)
	hash, _ := password.Hash("Password1!")
	user, _, err := repo.UpsertSuperAdmin(context.Background(), "admin@example.com", "Admin", "User", hash)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := uc.Login(context.Background(), "admin@example.com", "Password1!", "", model.SessionMeta{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if tok.AccessToken == "" {
		t.Fatal("expected access token")
	}
	me, err := uc.Me(context.Background(), user.UUID, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !me.User.IsSuperAdmin {
		t.Fatal("expected super admin")
	}
}

func TestLastSuperAdminProtection(t *testing.T) {
	repo := newMemRepo()
	tokens, _ := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	uc := New(repo, tokens)
	hash, _ := password.Hash("Password1!")
	user, _, _ := repo.UpsertSuperAdmin(context.Background(), "admin@example.com", "Admin", "User", hash)
	empty := []uuid.UUID{}
	if _, err := uc.PatchPlatformUser(context.Background(), user.UUID, model.PatchPlatformUserInput{RoleUUIDs: &empty}); !errors.Is(err, ErrLastSuperAdmin) {
		t.Fatalf("expected last super admin error, got %v", err)
	}
}

type oneInvite struct{ token, email string }

func (i oneInvite) PendingFor(_ context.Context, token, email string) bool {
	return token == i.token && email == i.email
}

// TestRegisterWithInviteWhileClosed: an open company invite lets its address sign up when
// self-registration is closed (TEC-945), and only that address.
func TestRegisterWithInviteWhileClosed(t *testing.T) {
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uc := New(newMemRepo(), tokens)
	uc.SetAuthSettings(testAuthGate{passwordLogin: true})
	uc.SetInviteChecker(oneInvite{token: "tok", email: "invited@example.com"})
	ctx := context.Background()

	if _, err := uc.Register(ctx, model.RegisterInput{
		Email: "other@example.com", Password: "Password1!", Name: "O", Surname: "X", InviteToken: "tok",
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other address: %v", err)
	}
	if _, err := uc.Register(ctx, model.RegisterInput{
		Email: "invited@example.com", Password: "Password1!", Name: "I", Surname: "X",
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("no token: %v", err)
	}
	user, err := uc.Register(ctx, model.RegisterInput{
		Email: "Invited@Example.com", Password: "Password1!", Name: "I", Surname: "X", InviteToken: "tok",
	})
	if err != nil {
		t.Fatalf("invited: %v", err)
	}
	if user.Email != "invited@example.com" {
		t.Fatalf("user: %+v", user)
	}
}

func TestCanSubscribeChannelAllowlist(t *testing.T) {
	uc := New(newMemRepo(), nil)
	me, other := uuid.New(), uuid.New()
	ctx := context.Background()
	cases := []struct {
		ch   string
		sa   bool
		want bool
	}{
		{"user:" + me.String(), false, true},
		{"user:" + me.String(), true, true},
		{"user:" + other.String(), false, false},
		{"user:" + other.String(), true, false},
		{"user:" + strings.ToUpper(me.String()), false, false},
		{"conversation:" + other.String(), false, false},
		{"conversation:" + me.String(), true, false},
		{"workspace:" + other.String(), false, false},
		{"account:" + other.String(), false, false},
		{"tenant:" + other.String(), true, false},
		{"system.notifications", false, false},
		{"system.notifications", true, true},
		{"", true, false},
	}
	for _, c := range cases {
		got, err := uc.CanSubscribeChannel(ctx, me, c.sa, c.ch)
		if err != nil || got != c.want {
			t.Fatalf("%q (sa=%v): got %v,%v want %v", c.ch, c.sa, got, err, c.want)
		}
	}
	if ok, _ := uc.CanSubscribeChannel(ctx, uuid.Nil, false, "user:"+uuid.Nil.String()); ok {
		t.Fatal("nil user must not get a user channel")
	}
}
