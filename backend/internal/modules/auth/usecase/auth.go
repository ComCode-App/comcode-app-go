package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/password"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserDisabled         = errors.New("user is disabled")
	ErrForbidden            = errors.New("forbidden")
	ErrNotFound             = errors.New("not found")
	ErrConflict             = errors.New("conflict")
	ErrInvalidRequest       = errors.New("invalid request")
	ErrLastSuperAdmin       = errors.New("cannot demote the last super admin")
	ErrSystemRole           = errors.New("system role cannot be modified")
	ErrAlreadyImpersonating = errors.New("already impersonating another user")
	ErrSessionRevoked       = errors.New("session revoked or expired")
	ErrAccountSuspended     = errors.New("account is suspended")
	// ErrLastSignInMethod: the change would leave the user without any way to sign in.
	ErrLastSignInMethod = errors.New("cannot remove the last sign-in method")
)

// ErrSuperAdminRequired: only a super admin may grant or remove the super_admin role or change a
// super admin account (roles, status, password, impersonation). It wraps ErrForbidden (403).
var ErrSuperAdminRequired = fmt.Errorf("%w: only a super admin can change super admin accounts or the super_admin role", ErrForbidden)

// Repository is the persistence port for auth use cases.
type Repository interface {
	CreateUser(ctx context.Context, u model.User, emailVerified bool) (model.User, error)
	FindUserByEmail(ctx context.Context, email string) (model.User, error)
	FindUserByUUID(ctx context.Context, id uuid.UUID) (model.User, error)
	FindUserByID(ctx context.Context, id int64) (model.User, error)
	UpdateLastLogin(ctx context.Context, userID int64) error
	UpdatePassword(ctx context.Context, userID int64, hash string) error
	MarkPasswordUnset(ctx context.Context, userID int64) error
	UpdateProfile(ctx context.Context, userUUID uuid.UUID, name, surname, locale *string) (model.User, error)
	SetEmailVerified(ctx context.Context, userID int64) (model.User, error)
	UpdateUserPlatform(ctx context.Context, id uuid.UUID, name, surname, status *string) (model.User, error)
	ListUsersFiltered(ctx context.Context, limit, offset int32, q, status, roleSlug string) ([]model.User, int64, error)
	CountUsersWithRole(ctx context.Context, roleSlug string) (int64, error)
	UpsertSuperAdmin(ctx context.Context, email, name, surname, hash string) (model.User, bool, error)
	GetRoleIDBySlug(ctx context.Context, slug string) (int64, error)
	ListPermissionsByRoleSlug(ctx context.Context, slug string) ([]string, error)
	ListUserRoleSlugs(ctx context.Context, userID int64) ([]string, error)
	UserHasRoleSlug(ctx context.Context, userID int64, slug string) (bool, error)
	ListUserRolesByUserUUID(ctx context.Context, userUUID uuid.UUID) ([]model.RoleSummary, error)
	ListRolesForUserIDs(ctx context.Context, userIDs []int64) (map[int64][]model.RoleSummary, error)
	ListOAuthAccountsForUserIDs(ctx context.Context, userIDs []int64) (map[int64][]model.OAuthAccountRecord, error)
	ListPasskeysForUserIDs(ctx context.Context, userIDs []int64) (map[int64][]model.PasskeyRecord, error)
	ReplaceUserRoles(ctx context.Context, userID int64, roleIDs []int64) error
	ResolveRoleIDsByUUIDs(ctx context.Context, roleUUIDs []uuid.UUID) ([]int64, error)
	SaveRefresh(ctx context.Context, userID int64, hash string, expiresAt time.Time, meta model.SessionMeta) (model.RefreshSession, error)
	RotateRefresh(ctx context.Context, oldHash, newHash string, expiresAt time.Time, meta model.SessionMeta) (model.RefreshSession, error)
	RotateRefreshFamilyWithAccount(ctx context.Context, familyUUID uuid.UUID, userID int64, newHash string, expiresAt time.Time, accountID int64) (model.RefreshSession, error)
	GetActiveSessionFamily(ctx context.Context, familyUUID uuid.UUID, userID int64) (model.RefreshSession, error)
	RevokeRefresh(ctx context.Context, hash string) error
	RevokeAllRefresh(ctx context.Context, userID int64) error
	ListActiveSessions(ctx context.Context, userID int64) ([]model.DeviceSession, error)
	RevokeSession(ctx context.Context, userID int64, sessionUUID uuid.UUID) error
	RevokeOtherSessions(ctx context.Context, userID int64, keep uuid.UUID) error
	CreateOTP(ctx context.Context, userID *int64, email, codeHash, otpType string, expiresAt time.Time) error
	InvalidateOTPs(ctx context.Context, email, otpType string) error
	GetActiveOTP(ctx context.Context, email, otpType string) (id int64, codeHash string, attempts, maxAttempts int32, err error)
	IncrementOTPAttempts(ctx context.Context, id int64) (attempts, maxAttempts int32, err error)
	ConsumeOTP(ctx context.Context, id int64) error
	GetPasskeyByCredentialID(ctx context.Context, credentialID string) (model.PasskeyRecord, error)
	ListPasskeysByUserID(ctx context.Context, userID int64) ([]model.PasskeyRecord, error)
	CreatePasskey(ctx context.Context, userID int64, in model.CreateAdapterAuthenticatorInput) (model.PasskeyRecord, error)
	UpdatePasskeyCounter(ctx context.Context, credentialID string, counter int64) error
	UpdatePasskeyName(ctx context.Context, userUUID, passkeyUUID uuid.UUID, name *string) (model.PasskeyRecord, error)
	DeletePasskeyByCredentialID(ctx context.Context, credentialID string) error
	DeletePasskeyByUUID(ctx context.Context, userUUID, passkeyUUID uuid.UUID) error
	GetOAuthAccountByProviderAccount(ctx context.Context, provider, providerAccountID string) (model.OAuthAccountRecord, error)
	GetOAuthAccountByUserProvider(ctx context.Context, userID int64, provider string) (model.OAuthAccountRecord, error)
	ListOAuthAccountsByUserID(ctx context.Context, userID int64) ([]model.OAuthAccountRecord, error)
	CreateOAuthAccount(ctx context.Context, in model.CreateOAuthAccountInput) (model.OAuthAccountRecord, error)
	DeleteOAuthAccountByProviderAccount(ctx context.Context, provider, providerAccountID string) error
	DeleteOAuthAccountByUserProvider(ctx context.Context, userID int64, provider string) error
	GetUserTOTP(ctx context.Context, userID int64) (model.UserTOTP, error)
	UpsertUserTOTPSetup(ctx context.Context, userID int64, secretEnc string) (model.UserTOTP, error)
	ConfirmUserTOTP(ctx context.Context, userID int64, recoveryHashes []string) (model.UserTOTP, error)
	UpdateUserTOTPRecoveryHashes(ctx context.Context, userID int64, recoveryHashes []string) error
	DeleteUserTOTP(ctx context.Context, userID int64) error
	ListRolesFiltered(ctx context.Context, limit, offset int32, q string) ([]model.RoleSummary, int64, error)
	GetRoleByUUID(ctx context.Context, id uuid.UUID) (model.RoleSummary, error)
	ListRolePermissionSlugs(ctx context.Context, roleUUID uuid.UUID) ([]string, error)
	CreateRole(ctx context.Context, name, slug string, description *string) (model.RoleSummary, error)
	UpdateRole(ctx context.Context, roleUUID uuid.UUID, name, description *string) (model.RoleSummary, error)
	DeleteRole(ctx context.Context, roleUUID uuid.UUID) error
	SetRolePermissions(ctx context.Context, roleID int64, permissionSlugs []string) error
	ListPermissionsFiltered(ctx context.Context, limit, offset int32, q string) ([]model.PermissionSummary, int64, error)
	GetPersonalAccount(ctx context.Context, userID int64) (model.AccountMembership, error)
	GetAccountMembershipByID(ctx context.Context, accountID, userID int64) (model.AccountMembership, error)
	GetAccountMembershipByUUID(ctx context.Context, accountUUID uuid.UUID, userID int64) (model.AccountMembership, error)
	ListAccountMemberships(ctx context.Context, userID int64) ([]model.AccountMembership, error)
}

// EntitlementGate resolves plan limits for session hydration.
type EntitlementGate interface {
	SummaryForAccount(ctx context.Context, accountID int64) (MeSubscriptionSummary, error)
}

// MeSubscriptionSummary mirrors entitlement.Summary without importing entitlement in handlers.
type MeSubscriptionSummary struct {
	PlanSlug string
	PlanName string
	Status   string
	Source   string
	Limits   []MeSubscriptionLimitEntry
}

// MeSubscriptionLimitEntry is one limit row for /me.
type MeSubscriptionLimitEntry struct {
	Key       string
	ValueType string
	IntValue  *int32
	BoolValue *bool
}

// AuthUseCase coordinates identity and session flows.
type AuthUseCase struct {
	repo          Repository
	tokens        *jwt.Manager
	now           func() time.Time
	notifier      Notifier
	log           *slog.Logger
	realtime      RealtimeHints
	searchIndexer SearchIndexer
	entitlement   EntitlementGate
	// Optional registration / method policy (nil = allow password login, deny register).
	authSettings AuthSettingsGate
	accessPolicy AccessPolicyGate
	box          SecretBox
	totpIssuer   string
	invites      InviteChecker
}

// InviteChecker reports open company invites (accounts usecase.Invites, TEC-945).
type InviteChecker interface {
	PendingFor(ctx context.Context, token, email string) bool
}

// SetInviteChecker lets invited addresses sign up while self-registration is closed.
func (u *AuthUseCase) SetInviteChecker(c InviteChecker) { u.invites = c }

// SecretBox encrypts at-rest secrets (TOTP).
type SecretBox interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(encoded string) (string, error)
}

// AuthSettingsGate is implemented by authsettings usecase.Service.
type AuthSettingsGate interface {
	CanPasswordRegister(ctx context.Context) (bool, error)
	CanPasswordLogin(ctx context.Context) (bool, error)
	CanPasskeyLogin(ctx context.Context) (bool, error)
	RegistrationEnabled(ctx context.Context) (bool, error)
	DefaultRoleID(ctx context.Context) (*int64, error)
}

// AccessPolicyGate reads admin access / verification policy.
type AccessPolicyGate interface {
	PasswordLoginTOTPRequired(ctx context.Context) (bool, error)
}

// SearchIndexer enqueues search index updates (optional, fail-soft).
type SearchIndexer interface {
	EnqueueUpsert(ctx context.Context, spec, id string)
	EnqueueDelete(ctx context.Context, spec, id string)
}

// RealtimeHints carries non-secret Centrifugo metadata for /me.
type RealtimeHints struct {
	Enabled bool
	WSURL   string
}

// New creates an AuthUseCase.
func New(repo Repository, tokens *jwt.Manager) *AuthUseCase {
	return &AuthUseCase{repo: repo, tokens: tokens, now: time.Now}
}

// SetEntitlement attaches plan limit resolution for /me.
func (u *AuthUseCase) SetEntitlement(gate EntitlementGate) {
	u.entitlement = gate
}

// SetAuthSettings attaches registration / login method policy.
func (u *AuthUseCase) SetAuthSettings(gate AuthSettingsGate) {
	u.authSettings = gate
}

// SetAccessPolicy attaches step-up / login verification policy.
func (u *AuthUseCase) SetAccessPolicy(gate AccessPolicyGate) {
	u.accessPolicy = gate
}

// SetSecretBox attaches encryption used for TOTP secrets.
func (u *AuthUseCase) SetSecretBox(box SecretBox, issuer string) {
	u.box = box
	u.totpIssuer = strings.TrimSpace(issuer)
}

// SetRealtimeHints attaches Centrifugo metadata for session hydration.
func (u *AuthUseCase) SetRealtimeHints(h RealtimeHints) {
	u.realtime = h
}

// Register creates a user-only account (no roles/tokens unless default role configured).
func (u *AuthUseCase) Register(ctx context.Context, in model.RegisterInput) (model.User, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	invited := in.InviteToken != "" && u.invites != nil && u.invites.PendingFor(ctx, in.InviteToken, in.Email)
	if u.authSettings != nil && !invited {
		ok, err := u.authSettings.CanPasswordRegister(ctx)
		if err != nil {
			return model.User{}, err
		}
		if !ok {
			return model.User{}, fmt.Errorf("%w: registration is disabled", ErrForbidden)
		}
	} else if !invited {
		return model.User{}, fmt.Errorf("%w: registration is disabled", ErrForbidden)
	}

	in.Name = strings.TrimSpace(in.Name)
	in.Surname = strings.TrimSpace(in.Surname)
	if in.Email == "" || in.Name == "" || in.Surname == "" {
		return model.User{}, fmt.Errorf("%w: email, name, surname are required", ErrInvalidRequest)
	}
	if strings.TrimSpace(in.Password) == "" {
		return model.User{}, fmt.Errorf("%w: password is required", ErrInvalidRequest)
	}
	if _, err := u.repo.FindUserByEmail(ctx, in.Email); err == nil {
		return model.User{}, fmt.Errorf("%w: email already registered", ErrConflict)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return model.User{}, err
	}

	hash, err := password.Hash(in.Password)
	if err != nil {
		return model.User{}, err
	}

	// The invite link reached this address, so it counts as verified.
	user, err := u.repo.CreateUser(ctx, model.User{
		Email: in.Email, PasswordHash: hash, Name: in.Name, Surname: in.Surname, Status: "active",
	}, invited)
	if err != nil {
		return model.User{}, err
	}
	if err := u.assignDefaultRole(ctx, user.ID); err != nil {
		return model.User{}, err
	}
	u.notifyWelcome(ctx, user)
	return user, nil
}

func (u *AuthUseCase) assignDefaultRole(ctx context.Context, userID int64) error {
	if u.authSettings == nil {
		return nil
	}
	roleID, err := u.authSettings.DefaultRoleID(ctx)
	if err != nil {
		return err
	}
	if roleID == nil {
		return nil
	}
	return u.repo.ReplaceUserRoles(ctx, userID, []int64{*roleID})
}

// RegisterOAuthUser creates a user for OAuth self-registration (no password login).
func (u *AuthUseCase) RegisterOAuthUser(ctx context.Context, email, name, surname string, emailVerified bool) (model.AdapterUser, error) {
	if u.authSettings == nil {
		return model.AdapterUser{}, fmt.Errorf("%w: registration is disabled", ErrForbidden)
	}
	enabled, err := u.authSettings.RegistrationEnabled(ctx)
	if err != nil {
		return model.AdapterUser{}, err
	}
	if !enabled {
		return model.AdapterUser{}, fmt.Errorf("%w: registration is disabled", ErrForbidden)
	}

	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	surname = strings.TrimSpace(surname)
	if email == "" {
		return model.AdapterUser{}, fmt.Errorf("%w: email is required", ErrInvalidRequest)
	}
	if name == "" {
		name = "User"
	}
	if surname == "" {
		surname = "Account"
	}
	if _, err := u.repo.FindUserByEmail(ctx, email); err == nil {
		return model.AdapterUser{}, fmt.Errorf("%w: email already registered", ErrConflict)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return model.AdapterUser{}, err
	}

	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return model.AdapterUser{}, err
	}
	hash, err := password.Hash(base64.RawURLEncoding.EncodeToString(random))
	if err != nil {
		return model.AdapterUser{}, err
	}

	user, err := u.repo.CreateUser(ctx, model.User{
		Email: email, PasswordHash: hash, Name: name, Surname: surname, Status: "active",
	}, emailVerified)
	if err != nil {
		return model.AdapterUser{}, err
	}
	// The random hash above is not a password the user can sign in with.
	if err := u.repo.MarkPasswordUnset(ctx, user.ID); err != nil {
		return model.AdapterUser{}, err
	}
	user.PasswordSet = false
	if err := u.assignDefaultRole(ctx, user.ID); err != nil {
		return model.AdapterUser{}, err
	}
	u.notifyWelcome(ctx, user)
	return toAdapterUser(user), nil
}

// Login authenticates and issues tokens for any active user.
// totpCode is required when the user has authenticator 2FA enabled, or when
// admin policy requires 2FA for password login.
func (u *AuthUseCase) Login(ctx context.Context, email, rawPassword, totpCode string, meta model.SessionMeta) (model.Tokens, error) {
	if u.authSettings != nil {
		ok, err := u.authSettings.CanPasswordLogin(ctx)
		if err != nil {
			return model.Tokens{}, err
		}
		if !ok {
			return model.Tokens{}, fmt.Errorf("%w: password login is disabled", ErrForbidden)
		}
	}

	email = strings.ToLower(strings.TrimSpace(email))
	user, err := u.repo.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Spend the same Argon2 work as a real check so response timing does not
			// reveal which emails are registered (TEC-1101).
			burnPasswordVerify(rawPassword)
			return model.Tokens{}, ErrInvalidCredentials
		}
		return model.Tokens{}, err
	}
	// Verify the password before revealing account state (disabled, suspended), so
	// the state cannot be probed without the credential.
	ok, err := password.Verify(user.PasswordHash, rawPassword)
	if err != nil {
		// Unusable stored hash: still spend the work, then answer like a wrong password.
		burnPasswordVerify(rawPassword)
		return model.Tokens{}, ErrInvalidCredentials
	}
	if !ok {
		return model.Tokens{}, ErrInvalidCredentials
	}
	if user.Status != "active" {
		return model.Tokens{}, ErrUserDisabled
	}

	hasTOTP, err := u.UserHasEnabledTOTP(ctx, user.ID)
	if err != nil {
		return model.Tokens{}, err
	}
	requirePolicy := false
	if u.accessPolicy != nil {
		requirePolicy, err = u.accessPolicy.PasswordLoginTOTPRequired(ctx)
		if err != nil {
			return model.Tokens{}, err
		}
	}
	if requirePolicy && !hasTOTP {
		return model.Tokens{}, ErrMFANotEnrolled
	}
	if hasTOTP {
		code := strings.TrimSpace(totpCode)
		if code == "" {
			return model.Tokens{}, ErrMFARequired
		}
		valid, _, err := u.verifyUserTOTPCode(ctx, user.ID, code, true)
		if err != nil {
			return model.Tokens{}, err
		}
		if !valid {
			return model.Tokens{}, ErrInvalidMFACode
		}
	}

	if err := u.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		return model.Tokens{}, err
	}
	return u.issueTokensForUser(ctx, user, meta)
}

// Refresh rotates an opaque refresh token.
// Refresh rotates the refresh token within its session family and keeps the session's active account.
func (u *AuthUseCase) Refresh(ctx context.Context, rawToken string, meta model.SessionMeta) (model.Tokens, error) {
	rawRefresh, err := newOpaqueToken()
	if err != nil {
		return model.Tokens{}, err
	}
	refreshExp := u.now().UTC().Add(u.tokens.RefreshTTL())
	session, err := u.repo.RotateRefresh(ctx, tokenHash(rawToken), tokenHash(rawRefresh), refreshExp, meta)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Tokens{}, ErrInvalidCredentials
		}
		return model.Tokens{}, err
	}
	user, err := u.repo.FindUserByID(ctx, session.UserID)
	if err != nil {
		return model.Tokens{}, ErrInvalidCredentials
	}
	if user.Status == "disabled" {
		_ = u.repo.RevokeRefresh(ctx, tokenHash(rawRefresh))
		return model.Tokens{}, ErrUserDisabled
	}
	return u.issueAccessForSession(ctx, user, session, rawRefresh, refreshExp)
}

// Logout revokes a refresh token. An empty token is a no-op so the BFF can
// still clear the browser session when the refresh cookie is already gone.
func (u *AuthUseCase) Logout(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	return u.repo.RevokeRefresh(ctx, tokenHash(rawToken))
}

// Me returns session hydration.
func (u *AuthUseCase) Me(ctx context.Context, userUUID uuid.UUID, impersonatorUUID *uuid.UUID, activeAccountID int64) (model.Me, error) {
	user, err := u.repo.FindUserByUUID(ctx, userUUID)
	if err != nil {
		return model.Me{}, ErrNotFound
	}
	roles, isSuperAdmin, perms, err := u.resolveUserAccess(ctx, user.ID)
	if err != nil {
		return model.Me{}, err
	}
	out := model.Me{
		User:          model.ToPublicUser(user, isSuperAdmin),
		Roles:         roles,
		Permissions:   perms,
		Links:         model.DefaultMeLinks(),
		Channels:      model.MeChannels{User: "user:" + user.UUID.String()},
		Realtime:      model.MeRealtime{Enabled: u.realtime.Enabled, WSURL: u.realtime.WSURL, UserChannel: "user:" + user.UUID.String()},
	}
	accounts, active, err := u.listAccountsWithActive(ctx, user.ID, activeAccountID)
	if err != nil {
		return model.Me{}, err
	}
	out.Accounts = accounts
	out.Account = active
	if u.entitlement != nil {
		if summary, err := u.entitlement.SummaryForAccount(ctx, activeAccountID); err == nil {
			limits := make([]model.MeSubscriptionLimit, 0, len(summary.Limits))
			for _, lim := range summary.Limits {
				limits = append(limits, model.MeSubscriptionLimit{
					Key: lim.Key, ValueType: lim.ValueType,
					IntValue: lim.IntValue, BoolValue: lim.BoolValue,
				})
			}
			out.Subscription = &model.MeSubscription{
				PlanSlug: summary.PlanSlug, PlanName: summary.PlanName,
				Status: summary.Status, Source: summary.Source, Limits: limits,
			}
		}
	}
	if impersonatorUUID != nil {
		impUser, err := u.repo.FindUserByUUID(ctx, *impersonatorUUID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return model.Me{}, ErrNotFound
			}
			return model.Me{}, err
		}
		isSA, err := u.repo.UserHasRoleSlug(ctx, impUser.ID, rbac.RoleSuperAdmin)
		if err != nil {
			return model.Me{}, err
		}
		out.Impersonation = &model.MeImpersonation{
			User: model.ToPublicUser(impUser, isSA),
		}
	}
	return out, nil
}

// ResolvePermissions returns permission slugs for roles (+ super admin expansion).
func (u *AuthUseCase) ResolvePermissions(ctx context.Context, roles []string, isSuperAdmin bool) ([]string, error) {
	set := map[string]struct{}{}
	if isSuperAdmin {
		all, err := u.repo.ListPermissionsByRoleSlug(ctx, rbac.RoleSuperAdmin)
		if err != nil {
			return nil, err
		}
		for _, p := range all {
			set[p] = struct{}{}
		}
	}
	for _, role := range roles {
		if role == rbac.RoleSuperAdmin {
			continue
		}
		ps, err := u.repo.ListPermissionsByRoleSlug(ctx, role)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			set[p] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	return out, nil
}

// LoadUserByUUID is used by middleware identity hydration.
func (u *AuthUseCase) LoadUserByUUID(ctx context.Context, id uuid.UUID) (model.User, error) {
	user, err := u.repo.FindUserByUUID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, err
	}
	return user, nil
}

func (u *AuthUseCase) resolveUserAccess(ctx context.Context, userID int64) ([]string, bool, []string, error) {
	roles, err := u.repo.ListUserRoleSlugs(ctx, userID)
	if err != nil {
		return nil, false, nil, err
	}
	isSuperAdmin := false
	for _, role := range roles {
		if role == rbac.RoleSuperAdmin {
			isSuperAdmin = true
			break
		}
	}
	perms, err := u.ResolvePermissions(ctx, roles, isSuperAdmin)
	if err != nil {
		return nil, false, nil, err
	}
	return uniqueStrings(roles), isSuperAdmin, perms, nil
}

// issueTokensForUser starts a new session family on the user's personal account.
func (u *AuthUseCase) issueTokensForUser(ctx context.Context, user model.User, meta model.SessionMeta) (model.Tokens, error) {
	personal, err := u.repo.GetPersonalAccount(ctx, user.ID)
	if err != nil {
		return model.Tokens{}, fmt.Errorf("personal account: %w", err)
	}
	meta.ActiveAccountID = &personal.ID
	rawRefresh, err := newOpaqueToken()
	if err != nil {
		return model.Tokens{}, err
	}
	refreshExp := u.now().UTC().Add(u.tokens.RefreshTTL())
	session, err := u.repo.SaveRefresh(ctx, user.ID, tokenHash(rawRefresh), refreshExp, meta)
	if err != nil {
		return model.Tokens{}, err
	}
	return u.issueAccessForSession(ctx, user, session, rawRefresh, refreshExp)
}

// issueAccessForSession signs an access token bound to a session family and its active account.
func (u *AuthUseCase) issueAccessForSession(ctx context.Context, user model.User, session model.RefreshSession, rawRefresh string, refreshExp time.Time) (model.Tokens, error) {
	roles, isSuperAdmin, _, err := u.resolveUserAccess(ctx, user.ID)
	if err != nil {
		return model.Tokens{}, err
	}
	var impersonatorUUID *uuid.UUID
	if session.ImpersonatorUserID != nil {
		impUser, err := u.repo.FindUserByID(ctx, *session.ImpersonatorUserID)
		if err != nil {
			return model.Tokens{}, err
		}
		impersonatorUUID = &impUser.UUID
	}
	account, err := u.resolveActiveAccount(ctx, user.ID, session.ActiveAccountID)
	if err != nil {
		return model.Tokens{}, err
	}
	access, accessExp, err := u.tokens.IssueAccess(jwt.AccessInput{
		UserID: user.UUID, Roles: roles, IsSuperAdmin: isSuperAdmin,
		ImpersonatorID: impersonatorUUID, SessionID: session.FamilyUUID,
		AccountID: account.UUID,
	})
	if err != nil {
		return model.Tokens{}, err
	}
	return model.Tokens{
		AccessToken: access, RefreshToken: rawRefresh,
		TokenType: "Bearer", ExpiresIn: int64(accessExp.Sub(u.now().UTC()).Seconds()),
		RefreshExpiresAt: refreshExp,
	}, nil
}

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

// burnPasswordVerify runs an Argon2 verification (same parameters as real hashes)
// against a throwaway hash computed once, so unknown emails cost as much as wrong passwords.
func burnPasswordVerify(raw string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = password.HashSecret("timing-equalizer")
	})
	if dummyHash != "" {
		_, _ = password.Verify(dummyHash, raw)
	}
}

func newOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
