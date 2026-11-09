package usecase

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/google/uuid"
)

// signInMethodRemoval names the one sign-in method a request wants to remove:
// an OAuth provider or a passkey (by uuid).
type signInMethodRemoval struct {
	provider    string
	passkeyUUID uuid.UUID
}

// ensureSignInMethodLeft refuses a removal that would leave the user with no
// way to sign in: no password they chose, no other linked OAuth identity and
// no other passkey (TEC-1105). It returns ErrNotFound when the method to
// remove is not the user's.
func ensureSignInMethodLeft(ctx context.Context, repo Repository, user model.User, rm signInMethodRemoval) error {
	accounts, err := repo.ListOAuthAccountsByUserID(ctx, user.ID)
	if err != nil {
		return err
	}
	passkeys, err := repo.ListPasskeysByUserID(ctx, user.ID)
	if err != nil {
		return err
	}
	found, remaining := false, 0
	for _, a := range accounts {
		if rm.provider != "" && a.Provider == rm.provider {
			found = true
			continue
		}
		remaining++
	}
	for _, p := range passkeys {
		if rm.passkeyUUID != uuid.Nil && p.UUID == rm.passkeyUUID {
			found = true
			continue
		}
		remaining++
	}
	if !found {
		return ErrNotFound
	}
	if user.PasswordSet || remaining > 0 {
		return nil
	}
	return ErrLastSignInMethod
}
