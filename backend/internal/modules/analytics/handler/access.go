package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
)

// Subject types an account can query its own analytics for.
const (
	SubjectRoute   = "route"
	SubjectCard    = "card"
	SubjectProfile = "profile"
)

// HistoryHeader tells the client how many days back its plan may query (absent = unlimited).
const HistoryHeader = "X-Analytics-History-Days"

var (
	errSubjectRequired = errors.New("subject_type and subject_id are required")
	errSubjectNotFound = errors.New("analytics subject not found")
)

// SubjectOwnership reports whether an analytics subject belongs to an account.
type SubjectOwnership interface {
	AccountOwnsSubject(ctx context.Context, accountID int64, subjectType string, subjectID uuid.UUID) (bool, error)
}

// HistoryPolicy is the plan's analytics history window (entitlement.Service implements it).
type HistoryPolicy interface {
	HistoryWindow(ctx context.Context, accountID int64, isSuperAdmin bool) (days int32, unlimited bool, err error)
}

// Access holds the dependencies that scope account analytics queries.
type Access struct {
	Ownership SubjectOwnership
	History   HistoryPolicy
	Now       func() time.Time
}

// authorize scopes an app analytics query: platform analytics readers see every subject;
// everyone else only subjects of the active account (404 otherwise), and no further back
// than the plan's analytics.history_days.
func (a Access) authorize(w http.ResponseWriter, r *http.Request, f *domain.AnalyticsFilter) error {
	p := authctx.MustPrincipal(r.Context())
	if p.HasPermission(rbac.PermPlatformAnalyticsRead) {
		return nil
	}
	if a.Ownership == nil || a.History == nil {
		return errors.New("analytics access is not configured")
	}
	if f.SubjectType == "" || f.SubjectID == "" {
		return errSubjectRequired
	}
	switch f.SubjectType {
	case SubjectRoute, SubjectCard, SubjectProfile:
	default:
		return errSubjectNotFound
	}
	id, err := uuid.Parse(f.SubjectID)
	if err != nil {
		return errSubjectNotFound
	}
	owned, err := a.Ownership.AccountOwnsSubject(r.Context(), p.Account.ID, f.SubjectType, id)
	if err != nil {
		return err
	}
	if !owned {
		return errSubjectNotFound
	}

	days, unlimited, err := a.History.HistoryWindow(r.Context(), p.Account.ID, p.IsSuperAdmin)
	if err != nil {
		return err
	}
	if unlimited {
		return nil
	}
	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	today := now().UTC().Truncate(24 * time.Hour)
	earliest := today.AddDate(0, 0, -int(days))
	if f.DateFrom.Before(earliest) {
		f.DateFrom = earliest
	}
	if f.DateTo.Before(f.DateFrom) {
		f.DateTo = f.DateFrom
	}
	w.Header().Set(HistoryHeader, strconv.Itoa(int(days)))
	return nil
}

// dbOwnership checks subject ownership with sqlc queries.
type dbOwnership struct{ q *db.Queries }

// NewSubjectOwnership returns the Postgres-backed ownership check.
func NewSubjectOwnership(q *db.Queries) SubjectOwnership { return dbOwnership{q: q} }

func (o dbOwnership) AccountOwnsSubject(ctx context.Context, accountID int64, subjectType string, id uuid.UUID) (bool, error) {
	switch subjectType {
	case SubjectRoute:
		return o.q.AccountOwnsRoute(ctx, db.AccountOwnsRouteParams{Uuid: id, AccountID: accountID})
	case SubjectCard:
		return o.q.AccountOwnsCard(ctx, db.AccountOwnsCardParams{Uuid: id, AccountID: accountID})
	case SubjectProfile:
		return o.q.AccountOwnsProfile(ctx, db.AccountOwnsProfileParams{Uuid: id, AccountID: accountID})
	default:
		return false, fmt.Errorf("unknown analytics subject type %q", subjectType)
	}
}
