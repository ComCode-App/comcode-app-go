package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/google/uuid"
)

const accountStatusActive = "active"

// resolveActiveAccount returns the preferred account when the user is still an active member of it,
// otherwise the user's personal account.
func (u *AuthUseCase) resolveActiveAccount(ctx context.Context, userID int64, preferredID *int64) (model.AccountMembership, error) {
	if preferredID != nil {
		m, err := u.repo.GetAccountMembershipByID(ctx, *preferredID, userID)
		switch {
		case err == nil && m.Status == accountStatusActive:
			return m, nil
		case err != nil && !errors.Is(err, repository.ErrNotFound):
			return model.AccountMembership{}, err
		}
	}
	m, err := u.repo.GetPersonalAccount(ctx, userID)
	if err != nil {
		return model.AccountMembership{}, fmt.Errorf("personal account: %w", err)
	}
	return m, nil
}

// resolveAccountByUUID is resolveActiveAccount for an access-token acc claim.
func (u *AuthUseCase) resolveAccountByUUID(ctx context.Context, userID int64, accountUUID uuid.UUID) (model.AccountMembership, error) {
	if accountUUID != uuid.Nil {
		m, err := u.repo.GetAccountMembershipByUUID(ctx, accountUUID, userID)
		switch {
		case err == nil && m.Status == accountStatusActive:
			return m, nil
		case err != nil && !errors.Is(err, repository.ErrNotFound):
			return model.AccountMembership{}, err
		}
	}
	return u.resolveActiveAccount(ctx, userID, nil)
}

func (u *AuthUseCase) listAccountsWithActive(ctx context.Context, userID, activeAccountID int64) ([]model.MeAccount, *model.MeAccount, error) {
	memberships, err := u.repo.ListAccountMemberships(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	out := make([]model.MeAccount, 0, len(memberships))
	var active *model.MeAccount
	for _, m := range memberships {
		item := model.MeAccount{AccountMembership: m, Active: m.ID == activeAccountID}
		out = append(out, item)
		if item.Active {
			a := item
			active = &a
		}
	}
	return out, active, nil
}

// ListAccounts returns the user's accounts, flagging the active one.
func (u *AuthUseCase) ListAccounts(ctx context.Context, userID, activeAccountID int64) ([]model.MeAccount, error) {
	accounts, _, err := u.listAccountsWithActive(ctx, userID, activeAccountID)
	return accounts, err
}

// SwitchAccount makes accountUUID the active account of the caller's session and returns a fresh token pair.
func (u *AuthUseCase) SwitchAccount(ctx context.Context, p authctx.Principal, accountUUID uuid.UUID) (model.AccountSwitchResult, error) {
	if accountUUID == uuid.Nil {
		return model.AccountSwitchResult{}, fmt.Errorf("%w: account_uuid is required", ErrInvalidRequest)
	}
	if p.SessionID == uuid.Nil {
		return model.AccountSwitchResult{}, ErrSessionRevoked
	}
	membership, err := u.repo.GetAccountMembershipByUUID(ctx, accountUUID, p.UserInternal)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.AccountSwitchResult{}, ErrNotFound
		}
		return model.AccountSwitchResult{}, err
	}
	if membership.Status != accountStatusActive {
		return model.AccountSwitchResult{}, ErrAccountSuspended
	}
	user, err := u.repo.FindUserByID(ctx, p.UserInternal)
	if err != nil {
		return model.AccountSwitchResult{}, err
	}
	rawRefresh, err := newOpaqueToken()
	if err != nil {
		return model.AccountSwitchResult{}, err
	}
	session, err := u.repo.RotateRefreshFamilyWithAccount(ctx, p.SessionID, p.UserInternal, tokenHash(rawRefresh), u.refreshExpiry(), membership.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.AccountSwitchResult{}, ErrSessionRevoked
		}
		return model.AccountSwitchResult{}, err
	}
	tokens, err := u.issueAccessForSession(ctx, user, session, rawRefresh, session.ExpiresAt)
	if err != nil {
		return model.AccountSwitchResult{}, err
	}
	return model.AccountSwitchResult{
		Tokens:  tokens,
		Session: model.SessionSwitch{UserUUID: user.UUID, Email: user.Email, ImpersonatorUUID: p.ImpersonatorUserID},
		Account: membership,
	}, nil
}

// LoadPrincipal hydrates a request principal from a verified access token. Authorization data is
// reloaded from the database on every request: roles and super-admin status (revocations apply
// immediately), the device session (a revoked or expired session is rejected) and the active
// account membership (a lost membership falls back to the personal account).
func (u *AuthUseCase) LoadPrincipal(ctx context.Context, claims jwt.Claims) (authctx.Principal, error) {
	userUUID, err := claims.UserUUID()
	if err != nil {
		return authctx.Principal{}, err
	}
	user, err := u.LoadUserByUUID(ctx, userUUID)
	if err != nil {
		return authctx.Principal{}, err
	}
	if user.Status == "disabled" {
		return authctx.Principal{}, ErrUserDisabled
	}
	sessionID := claims.SessionUUID()
	if sessionID == uuid.Nil {
		return authctx.Principal{}, ErrSessionRevoked
	}
	if _, err := u.repo.GetActiveSessionFamily(ctx, sessionID, user.ID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return authctx.Principal{}, ErrSessionRevoked
		}
		return authctx.Principal{}, err
	}
	roles, isSuperAdmin, perms, err := u.resolveUserAccess(ctx, user.ID)
	if err != nil {
		return authctx.Principal{}, err
	}
	impersonatorUUID, err := claims.ImpersonatorUUID()
	if err != nil {
		return authctx.Principal{}, err
	}
	account, err := u.resolveAccountByUUID(ctx, user.ID, claims.AccountUUID())
	if err != nil {
		return authctx.Principal{}, err
	}
	return authctx.Principal{
		UserID:             user.UUID,
		UserInternal:       user.ID,
		Email:              user.Email,
		Roles:              roles,
		Permissions:        perms,
		IsSuperAdmin:       isSuperAdmin,
		ImpersonatorUserID: impersonatorUUID,
		SessionID:          sessionID,
		Account: authctx.Account{
			ID:   account.ID,
			UUID: account.UUID,
			Type: account.Type,
			Role: account.Role,
		},
	}, nil
}
