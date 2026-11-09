// Package repository persists leads. App methods take the account id as a required scope:
// rows of other accounts behave as missing. Reads also take the viewer (TEC-940): owners and
// admins see the whole account, other members only leads of their own profile or card.
package repository

import (
	"context"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/google/uuid"
)

// CaptureRoute is the public route a visitor submits through.
type CaptureRoute struct {
	ID                 int64
	RouteType          string
	Status             string
	AccountID          int64
	PresentationConfig []byte
	CardID             *int64
	ProfileID          *int64
	ProfileUUID        *uuid.UUID
	CardLeadCapture    bool
	CardIsActive       bool
	CardIsLocked       bool
}

// NewLead is one row to insert.
type NewLead struct {
	AccountID  int64
	RouteID    int64
	ProfileID  *int64
	CardID     *int64
	SourceType string
	RoutePath  string
	Kind       string
	Name       string
	Email      string
	Phone      string
	Message    string
	Company    string
	JobTitle   string
	// SenderUserID and SenderProfileID: the signed-in visitor and the profile they shared.
	SenderUserID    *int64
	SenderProfileID *int64
	Locale          string
	ConsentAt       time.Time
	ConsentVersion  string
	IP              string
	UserAgent       string
	// Locked: over the account's monthly quota (TEC-941).
	Locked bool
}

// SenderProfile is the profile a signed-in visitor shares in an exchange.
type SenderProfile struct {
	ID        int64
	UserID    *int64
	FirstName string
	LastName  string
	Company   string
	Title     string
	Email     string
	Phone     string
}

// Scope is who reads: the active account and the signed-in user. Both are required.
type Scope struct {
	AccountID    int64
	ViewerUserID int64
}

// Repository is the leads persistence port.
type Repository interface {
	// CaptureRoute returns nil when no live route of an active account has the path.
	CaptureRoute(ctx context.Context, path string) (*CaptureRoute, error)
	Create(ctx context.Context, in NewLead) (model.SubmitResult, error)
	List(ctx context.Context, scope Scope, f model.ListFilter) ([]model.Lead, int64, error)
	// Get returns nil when the lead is not in the account or not visible to the viewer.
	Get(ctx context.Context, scope Scope, id uuid.UUID) (*model.Lead, error)
	// MarkRead and Delete report false when the lead is not in the account. Delete also drops
	// the visitor copy email of an exchange.
	MarkRead(ctx context.Context, accountID int64, id uuid.UUID) (bool, error)
	Delete(ctx context.Context, accountID int64, id uuid.UUID) (bool, error)
	// DeleteVisitorExchange deletes an exchange and its visitor copy email for the KVKK link of
	// that email; false when it is already gone.
	DeleteVisitorExchange(ctx context.Context, id uuid.UUID) (bool, error)
	// NotifyUserIDs lists the account's owners and admins.
	NotifyUserIDs(ctx context.Context, accountID int64) ([]int64, error)
	// ProfileName is the display name of a profile in the locale (falls back to its primary
	// locale); "" when the profile is gone.
	ProfileName(ctx context.Context, profileID int64, locale string) (string, error)
	// SenderProfile returns nil when the profile is not in the account.
	SenderProfile(ctx context.Context, accountID int64, id uuid.UUID) (*SenderProfile, error)
	// HasLocked reports whether the account has any locked lead (TEC-941).
	HasLocked(ctx context.Context, accountID int64) (bool, error)
	// UnlockBefore unlocks every lead of the account locked before monthStart.
	UnlockBefore(ctx context.Context, accountID int64, monthStart time.Time) (int64, error)
	// UnlockMonth unlocks up to maxRows of the oldest leads of a kind locked since monthStart.
	UnlockMonth(ctx context.Context, accountID int64, kind string, monthStart time.Time, maxRows int32) (int64, error)
}
