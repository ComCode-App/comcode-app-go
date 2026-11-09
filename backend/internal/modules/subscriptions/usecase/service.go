package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidRequest = errors.New("invalid request")
	ErrConflict       = errors.New("conflict")
)

type Service struct {
	q *db.Queries
}

func New(q *db.Queries) *Service {
	return &Service{q: q}
}

type SubjectSummary struct {
	Type string    `json:"type"`
	UUID uuid.UUID `json:"uuid"`
	Name string    `json:"name"`
}

type SubscriptionSummary struct {
	UUID      uuid.UUID       `json:"uuid"`
	Status    string          `json:"status"`
	PlanUUID  uuid.UUID       `json:"plan_uuid"`
	PlanSlug  string          `json:"plan_slug"`
	PlanName  string          `json:"plan_name"`
	Subject   SubjectSummary  `json:"subject"`
	StartsAt  string          `json:"starts_at"`
	EndsAt    *string         `json:"ends_at,omitempty"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

type CreateSubscriptionInput struct {
	PlanUUID    uuid.UUID
	Status      string
	UserUUID    *uuid.UUID
	CompanyUUID *uuid.UUID
	StartsAt    *time.Time
	EndsAt      *time.Time
}

type PatchSubscriptionInput struct {
	PlanUUID *uuid.UUID
	Status   *string
	StartsAt *time.Time
	EndsAt   **time.Time
}

func (s *Service) List(ctx context.Context, limit, offset int32, q, status string) ([]SubscriptionSummary, int64, error) {
	qArg := pgtype.Text{}
	if strings.TrimSpace(q) != "" {
		qArg = pgtype.Text{String: strings.TrimSpace(q), Valid: true}
	}
	statusArg := db.NullSubscriptionStatus{}
	if strings.TrimSpace(status) != "" {
		statusArg = db.NullSubscriptionStatus{
			SubscriptionStatus: db.SubscriptionStatus(status),
			Valid:              true,
		}
	}
	rows, err := s.q.ListSubscriptionsFiltered(ctx, db.ListSubscriptionsFilteredParams{
		Q: qArg, Status: statusArg, LimitCount: limit, OffsetCount: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountSubscriptions(ctx, db.CountSubscriptionsParams{Q: qArg, Status: statusArg})
	if err != nil {
		return nil, 0, err
	}
	out := make([]SubscriptionSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapSubscriptionRow(row))
	}
	return out, total, nil
}

func (s *Service) Get(ctx context.Context, subUUID uuid.UUID) (SubscriptionSummary, error) {
	row, err := s.q.GetSubscriptionByUUID(ctx, subUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubscriptionSummary{}, ErrNotFound
		}
		return SubscriptionSummary{}, err
	}
	return mapSubscriptionDetailRow(row), nil
}

func (s *Service) Create(ctx context.Context, in CreateSubscriptionInput) (SubscriptionSummary, error) {
	hasUser := in.UserUUID != nil
	hasCompany := in.CompanyUUID != nil
	if hasUser == hasCompany {
		return SubscriptionSummary{}, fmt.Errorf("%w: provide user_uuid or company_uuid", ErrInvalidRequest)
	}
	planID, err := s.q.GetPlanIDByUUID(ctx, in.PlanUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubscriptionSummary{}, fmt.Errorf("%w: plan not found", ErrNotFound)
		}
		return SubscriptionSummary{}, err
	}
	status, err := parseStatus(in.Status)
	if err != nil {
		return SubscriptionSummary{}, err
	}
	startsAt := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	if in.StartsAt != nil {
		startsAt = pgtype.Timestamptz{Time: in.StartsAt.UTC(), Valid: true}
	}
	var endsAt pgtype.Timestamptz
	if in.EndsAt != nil {
		endsAt = pgtype.Timestamptz{Time: in.EndsAt.UTC(), Valid: true}
	}
	// The subscription's subject is an account: the user's personal account or a company account.
	var accountID int64
	if hasUser {
		id, err := s.q.GetPersonalAccountIDByUserUUID(ctx, *in.UserUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return SubscriptionSummary{}, fmt.Errorf("%w: user not found", ErrNotFound)
			}
			return SubscriptionSummary{}, err
		}
		accountID = id
	} else {
		id, err := s.q.GetAccountIDByUUID(ctx, *in.CompanyUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return SubscriptionSummary{}, fmt.Errorf("%w: company account not found", ErrNotFound)
			}
			return SubscriptionSummary{}, err
		}
		accountID = id
	}
	if status == db.SubscriptionStatusActive {
		if err := s.q.CancelActiveAccountSubscriptions(ctx, accountID); err != nil {
			return SubscriptionSummary{}, err
		}
	}
	row, err := s.q.CreateSubscription(ctx, db.CreateSubscriptionParams{
		PlanID: planID, Status: status, AccountID: accountID,
		StartsAt: startsAt, EndsAt: endsAt,
	})
	if err != nil {
		return SubscriptionSummary{}, err
	}
	return s.Get(ctx, row.Uuid)
}

func (s *Service) Patch(ctx context.Context, subUUID uuid.UUID, in PatchSubscriptionInput) (SubscriptionSummary, error) {
	if in.PlanUUID == nil && in.Status == nil && in.StartsAt == nil && in.EndsAt == nil {
		return SubscriptionSummary{}, fmt.Errorf("%w: at least one field is required", ErrInvalidRequest)
	}
	params := db.UpdateSubscriptionParams{Uuid: subUUID}
	if in.PlanUUID != nil {
		planID, err := s.q.GetPlanIDByUUID(ctx, *in.PlanUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return SubscriptionSummary{}, fmt.Errorf("%w: plan not found", ErrNotFound)
			}
			return SubscriptionSummary{}, err
		}
		params.PlanID = pgtype.Int8{Int64: planID, Valid: true}
	}
	if in.Status != nil {
		status, err := parseStatus(*in.Status)
		if err != nil {
			return SubscriptionSummary{}, err
		}
		params.Status = db.NullSubscriptionStatus{SubscriptionStatus: status, Valid: true}
	}
	if in.StartsAt != nil {
		params.StartsAt = pgtype.Timestamptz{Time: in.StartsAt.UTC(), Valid: true}
	}
	if in.EndsAt != nil {
		if *in.EndsAt == nil {
			params.EndsAt = pgtype.Timestamptz{}
		} else {
			params.EndsAt = pgtype.Timestamptz{Time: (*in.EndsAt).UTC(), Valid: true}
		}
	}
	if _, err := s.q.UpdateSubscription(ctx, params); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubscriptionSummary{}, ErrNotFound
		}
		return SubscriptionSummary{}, err
	}
	return s.Get(ctx, subUUID)
}

func mapSubscriptionRow(row db.ListSubscriptionsFilteredRow) SubscriptionSummary {
	return mapSubscriptionCommon(
		row.Uuid, row.Status, row.PlanUuid, row.PlanSlug, row.PlanName,
		row.UserUuid, row.UserEmail, row.UserName, row.UserSurname,
		row.CompanyUuid, row.CompanyName, row.CompanySlug,
		row.StartsAt, row.EndsAt, row.CreatedAt, row.UpdatedAt,
	)
}

func mapSubscriptionDetailRow(row db.GetSubscriptionByUUIDRow) SubscriptionSummary {
	return mapSubscriptionCommon(
		row.Uuid, row.Status, row.PlanUuid, row.PlanSlug, row.PlanName,
		row.UserUuid, row.UserEmail, row.UserName, row.UserSurname,
		row.CompanyUuid, row.CompanyName, row.CompanySlug,
		row.StartsAt, row.EndsAt, row.CreatedAt, row.UpdatedAt,
	)
}

func mapSubscriptionCommon(
	subUUID uuid.UUID,
	status db.SubscriptionStatus,
	planUUID uuid.UUID,
	planSlug, planName string,
	userUUID pgtype.UUID,
	userEmail pgtype.Text,
	userName, userSurname pgtype.Text,
	companyUUID pgtype.UUID,
	companyName, companySlug pgtype.Text,
	startsAt, endsAt, createdAt, updatedAt pgtype.Timestamptz,
) SubscriptionSummary {
	subject := SubjectSummary{}
	if userUUID.Valid {
		subject.Type = "user"
		subject.UUID = uuid.UUID(userUUID.Bytes)
		name := strings.TrimSpace(userName.String + " " + userSurname.String)
		if name == "" {
			name = userEmail.String
		}
		subject.Name = name
	} else if companyUUID.Valid {
		subject.Type = "company"
		subject.UUID = uuid.UUID(companyUUID.Bytes)
		subject.Name = companyName.String
		if subject.Name == "" {
			subject.Name = companySlug.String
		}
	}
	var ends *string
	if endsAt.Valid {
		v := endsAt.Time.Format(time.RFC3339)
		ends = &v
	}
	return SubscriptionSummary{
		UUID: subUUID, Status: string(status),
		PlanUUID: planUUID, PlanSlug: planSlug, PlanName: planName,
		Subject: subject,
		StartsAt: startsAt.Time.Format(time.RFC3339),
		EndsAt: ends,
		CreatedAt: createdAt.Time.Format(time.RFC3339),
		UpdatedAt: updatedAt.Time.Format(time.RFC3339),
	}
}

func parseStatus(v string) (db.SubscriptionStatus, error) {
	if v == "" {
		return db.SubscriptionStatusActive, nil
	}
	switch db.SubscriptionStatus(v) {
	case db.SubscriptionStatusActive, db.SubscriptionStatusTrialing,
		db.SubscriptionStatusPastDue, db.SubscriptionStatusCanceled:
		return db.SubscriptionStatus(v), nil
	default:
		return "", fmt.Errorf("%w: invalid status", ErrInvalidRequest)
	}
}
