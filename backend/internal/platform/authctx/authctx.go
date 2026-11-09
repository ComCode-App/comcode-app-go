package authctx

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey int

const (
	keyPrincipal ctxKey = iota + 1
)

// Account roles (account_members.role).
const (
	AccountRoleOwner         = "owner"
	AccountRoleAdmin         = "admin"
	AccountRoleMember        = "member"
	AccountRoleOfficeManager = "office_manager"
)

// Account types (accounts.type).
const (
	AccountTypePersonal = "personal"
	AccountTypeCompany  = "company"
)

// Account is the active account a request acts in, with the caller's role in it.
// Every authenticated request carries one (docs/TENANCY_AND_ACCESS.md).
type Account struct {
	ID   int64
	UUID uuid.UUID
	Type string
	Role string
}

// IsOwner reports whether the caller owns the active account.
func (a Account) IsOwner() bool { return a.Role == AccountRoleOwner }

// CanWrite reports whether the caller may create, edit or delete resources in the account.
func (a Account) CanWrite() bool { return a.Role == AccountRoleOwner || a.Role == AccountRoleAdmin }

// CanManageMembers reports whether the caller may manage non-owner members.
func (a Account) CanManageMembers() bool { return a.CanWrite() }

// IsPersonal reports whether the account is a personal account.
func (a Account) IsPersonal() bool { return a.Type == AccountTypePersonal }

// Principal is the authenticated identity attached to a request context.
type Principal struct {
	UserID             uuid.UUID
	UserInternal       int64
	Email              string
	Roles              []string
	Permissions        []string
	IsSuperAdmin       bool
	ImpersonatorUserID *uuid.UUID
	// SessionID is the refresh token family of the current device session.
	SessionID uuid.UUID
	Account   Account
}

// WithPrincipal stores the principal on the context.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, keyPrincipal, p)
}

// PrincipalFrom returns the principal if present.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(keyPrincipal).(Principal)
	return p, ok
}

// MustPrincipal returns the principal or panics (for handlers behind Authenticate).
func MustPrincipal(ctx context.Context) Principal {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		panic("authctx: principal missing from context")
	}
	return p
}

// HasPermission reports whether the principal includes the given permission slug.
func (p Principal) HasPermission(slug string) bool {
	if p.IsSuperAdmin {
		return true
	}
	for _, perm := range p.Permissions {
		if perm == slug {
			return true
		}
	}
	return false
}

// HasRole reports whether the principal includes the given role slug.
func (p Principal) HasRole(slug string) bool {
	for _, role := range p.Roles {
		if role == slug {
			return true
		}
	}
	return false
}
