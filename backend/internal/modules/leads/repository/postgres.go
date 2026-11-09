package repository

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Postgres implements Repository with sqlc.
type Postgres struct {
	q *db.Queries
}

// NewPostgres creates the sqlc-backed repository.
func NewPostgres(q *db.Queries) *Postgres { return &Postgres{q: q} }

func (r *Postgres) CaptureRoute(ctx context.Context, path string) (*CaptureRoute, error) {
	row, err := r.q.GetLeadCaptureRouteByPath(ctx, path)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := &CaptureRoute{
		ID: row.ID, RouteType: row.RouteType, Status: row.Status, AccountID: row.AccountID,
		PresentationConfig: row.PresentationConfig,
		CardID:             int8Ptr(row.CardID), ProfileID: int8Ptr(row.ProfileID),
		ProfileUUID:     uuidPtr(row.ProfileUuid),
		CardLeadCapture: row.CardLeadCaptureEnabled, CardIsActive: row.CardIsActive, CardIsLocked: row.CardIsLocked,
	}
	return out, nil
}

func (r *Postgres) Create(ctx context.Context, in NewLead) (model.SubmitResult, error) {
	var ip *netip.Addr
	if a, err := netip.ParseAddr(in.IP); err == nil {
		ip = &a
	}
	row, err := r.q.CreateLead(ctx, db.CreateLeadParams{
		AccountID: in.AccountID, RouteID: pgtype.Int8{Int64: in.RouteID, Valid: true},
		ProfileID: int8Arg(in.ProfileID), CardID: int8Arg(in.CardID),
		SourceType: in.SourceType, RoutePath: in.RoutePath, Kind: in.Kind,
		Name: in.Name, Email: in.Email, Phone: in.Phone, Message: in.Message,
		Company: in.Company, JobTitle: in.JobTitle,
		SenderUserID: int8Arg(in.SenderUserID), SenderProfileID: int8Arg(in.SenderProfileID),
		Locale:         in.Locale,
		ConsentAt:      pgtype.Timestamptz{Time: in.ConsentAt, Valid: true},
		ConsentVersion: in.ConsentVersion, IpAddress: ip, UserAgent: in.UserAgent, Locked: in.Locked,
	})
	if err != nil {
		return model.SubmitResult{}, err
	}
	return model.SubmitResult{UUID: row.Uuid, CreatedAt: row.CreatedAt.Time}, nil
}

func (r *Postgres) List(ctx context.Context, scope Scope, f model.ListFilter) ([]model.Lead, int64, error) {
	q, source, kind, unread := textArg(f.Q), textArg(f.SourceType), textArg(f.Kind), boolArg(f.Unread)
	rows, err := r.q.ListAccountLeads(ctx, db.ListAccountLeadsParams{
		AccountID: scope.AccountID, ViewerUserID: scope.ViewerUserID,
		Q: q, SourceType: source, Kind: kind, Unread: unread, LimitCount: f.Limit, OffsetCount: f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountAccountLeads(ctx, db.CountAccountLeadsParams{
		AccountID: scope.AccountID, ViewerUserID: scope.ViewerUserID,
		Q: q, SourceType: source, Kind: kind, Unread: unread,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.Lead, 0, len(rows))
	for _, row := range rows {
		out = append(out, toLead(db.GetAccountLeadRow(row)))
	}
	return out, total, nil
}

func (r *Postgres) Get(ctx context.Context, scope Scope, id uuid.UUID) (*model.Lead, error) {
	row, err := r.q.GetAccountLead(ctx, db.GetAccountLeadParams{
		Uuid: id, AccountID: scope.AccountID, ViewerUserID: scope.ViewerUserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	l := toLead(row)
	return &l, nil
}

func (r *Postgres) MarkRead(ctx context.Context, accountID int64, id uuid.UUID) (bool, error) {
	n, err := r.q.MarkAccountLeadRead(ctx, db.MarkAccountLeadReadParams{Uuid: id, AccountID: accountID})
	return n > 0, err
}

func (r *Postgres) Delete(ctx context.Context, accountID int64, id uuid.UUID) (bool, error) {
	n, err := r.q.DeleteAccountLead(ctx, db.DeleteAccountLeadParams{Uuid: id, AccountID: accountID})
	if err != nil || n == 0 {
		return false, err
	}
	_, err = r.q.DeleteExchangeCopyNotifications(ctx, exchangeCopyEvent(id))
	return true, err
}

func (r *Postgres) DeleteVisitorExchange(ctx context.Context, id uuid.UUID) (bool, error) {
	// The copy first: if the second statement fails, the link still works for a retry.
	if _, err := r.q.DeleteExchangeCopyNotifications(ctx, exchangeCopyEvent(id)); err != nil {
		return false, err
	}
	_, err := r.q.DeleteVisitorExchange(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func exchangeCopyEvent(id uuid.UUID) pgtype.Text {
	return pgtype.Text{String: model.ExchangeCopyEvent(id), Valid: true}
}

func (r *Postgres) NotifyUserIDs(ctx context.Context, accountID int64) ([]int64, error) {
	return r.q.ListLeadNotifyUserIDs(ctx, accountID)
}

func (r *Postgres) ProfileName(ctx context.Context, profileID int64, locale string) (string, error) {
	name, err := r.q.GetLeadProfileName(ctx, db.GetLeadProfileNameParams{ID: profileID, Locale: locale})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return name, err
}

func (r *Postgres) SenderProfile(ctx context.Context, accountID int64, id uuid.UUID) (*SenderProfile, error) {
	row, err := r.q.GetExchangeSenderProfile(ctx, db.GetExchangeSenderProfileParams{Uuid: id, AccountID: accountID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &SenderProfile{
		ID: row.ID, UserID: int8Ptr(row.UserID),
		FirstName: row.FirstName, LastName: row.LastName, Company: row.CompanyName, Title: row.Title,
		Email: row.Email, Phone: row.Phone,
	}, nil
}

func (r *Postgres) HasLocked(ctx context.Context, accountID int64) (bool, error) {
	return r.q.HasLockedLeads(ctx, accountID)
}

func (r *Postgres) UnlockBefore(ctx context.Context, accountID int64, monthStart time.Time) (int64, error) {
	return r.q.UnlockLeadsBefore(ctx, db.UnlockLeadsBeforeParams{AccountID: accountID, MonthStart: tsArg(monthStart)})
}

func (r *Postgres) UnlockMonth(ctx context.Context, accountID int64, kind string, monthStart time.Time, maxRows int32) (int64, error) {
	return r.q.UnlockMonthLeads(ctx, db.UnlockMonthLeadsParams{
		AccountID: accountID, Kind: kind, MonthStart: tsArg(monthStart), MaxRows: maxRows,
	})
}

func tsArg(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func toLead(row db.GetAccountLeadRow) model.Lead {
	l := model.Lead{
		UUID: row.Uuid, SourceType: row.SourceType, RoutePath: row.RoutePath,
		ProfileUUID: uuidPtr(row.ProfileUuid), CardUUID: uuidPtr(row.CardUuid), Kind: row.Kind,
		Name: row.Name, Email: row.Email, Phone: row.Phone, Message: row.Message,
		Company: row.Company, JobTitle: row.JobTitle,
		SenderProfileUUID: uuidPtr(row.SenderProfileUuid), SenderUsername: row.SenderUsername.String,
		Locale:    row.Locale,
		ConsentAt: row.ConsentAt.Time, ConsentVersion: row.ConsentVersion, CreatedAt: row.CreatedAt.Time,
		Locked: row.Locked,
	}
	if row.ReadAt.Valid {
		t := row.ReadAt.Time
		l.ReadAt = &t
	}
	return l
}

func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func int8Arg(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func uuidPtr(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	u := uuid.UUID(v.Bytes)
	return &u
}

func textArg(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func boolArg(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}

var _ Repository = (*Postgres)(nil)
