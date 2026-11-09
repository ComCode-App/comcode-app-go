package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/slug"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidRequest = errors.New("invalid request")
	ErrConflict       = errors.New("conflict")
)

type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func New(pool *pgxpool.Pool, q *db.Queries) *Service {
	return &Service{pool: pool, q: q}
}

type FeatureDefinition struct {
	Key         string  `json:"key"`
	ValueType   string  `json:"value_type"`
	Label       string  `json:"label"`
	Description *string `json:"description,omitempty"`
	SortOrder   int32   `json:"sort_order"`
}

type PlanLimit struct {
	FeatureKey  string  `json:"feature_key"`
	ValueType   string  `json:"value_type"`
	Label       string  `json:"label"`
	IntValue    *int32  `json:"int_value,omitempty"`
	BoolValue   *bool   `json:"bool_value,omitempty"`
	Description *string `json:"description,omitempty"`
}

type PlanSummary struct {
	UUID            uuid.UUID `json:"uuid"`
	Slug            string    `json:"slug"`
	Name            string    `json:"name"`
	Description     *string   `json:"description,omitempty"`
	BillingInterval string    `json:"billing_interval"`
	PriceMinor      int64     `json:"price_minor"`
	Currency        string    `json:"currency"`
	TrialDays       int32     `json:"trial_days"`
	IsActive        bool      `json:"is_active"`
	SortOrder       int32     `json:"sort_order"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
}

type PlanDetail struct {
	PlanSummary
	Limits []PlanLimit `json:"limits"`
}

type CreatePlanInput struct {
	Slug            string
	Name            string
	Description     *string
	BillingInterval string
	PriceMinor      int64
	Currency        string
	TrialDays       int32
	IsActive        bool
	SortOrder       int32
	Limits          []PlanLimitInput
}

// PlanLimitInput is one limit of a plan create/patch body. IntValue is int64 so an
// out-of-range number becomes a field error instead of a body decode failure.
type PlanLimitInput struct {
	FeatureKey string `json:"feature_key"`
	IntValue   *int64 `json:"int_value,omitempty"`
	BoolValue  *bool  `json:"bool_value,omitempty"`
}

type PatchPlanInput struct {
	Name            *string
	Description     *string
	BillingInterval *string
	PriceMinor      *int64
	Currency        *string
	TrialDays       *int32
	IsActive        *bool
	SortOrder       *int32
	Limits          *[]PlanLimitInput
}

func (s *Service) ListFeatures(ctx context.Context) ([]FeatureDefinition, error) {
	rows, err := s.q.ListFeatureDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]FeatureDefinition, 0, len(rows))
	for _, row := range rows {
		var desc *string
		if row.Description.Valid {
			v := row.Description.String
			desc = &v
		}
		out = append(out, FeatureDefinition{
			Key: row.Key, ValueType: string(row.ValueType), Label: row.Label,
			Description: desc, SortOrder: row.SortOrder,
		})
	}
	return out, nil
}

func (s *Service) List(ctx context.Context, limit, offset int32, q string) ([]PlanSummary, int64, error) {
	qArg := pgtype.Text{}
	if strings.TrimSpace(q) != "" {
		qArg = pgtype.Text{String: strings.TrimSpace(q), Valid: true}
	}
	rows, err := s.q.ListPlansFiltered(ctx, db.ListPlansFilteredParams{
		Q: qArg, LimitCount: limit, OffsetCount: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountPlans(ctx, qArg)
	if err != nil {
		return nil, 0, err
	}
	out := make([]PlanSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPlanSummary(row))
	}
	return out, total, nil
}

func (s *Service) Get(ctx context.Context, planUUID uuid.UUID) (PlanDetail, error) {
	row, err := s.q.GetPlanByUUID(ctx, planUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanDetail{}, ErrNotFound
		}
		return PlanDetail{}, err
	}
	limits, err := s.listLimits(ctx, row.ID)
	if err != nil {
		return PlanDetail{}, err
	}
	return PlanDetail{PlanSummary: mapPlanSummary(row), Limits: limits}, nil
}

func (s *Service) Create(ctx context.Context, in CreatePlanInput) (PlanDetail, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Slug = slug.FromName(strings.TrimSpace(in.Slug))
	if in.Name == "" || in.Slug == "" {
		return PlanDetail{}, fmt.Errorf("%w: name and slug are required", ErrInvalidRequest)
	}
	interval, err := parseBillingInterval(in.BillingInterval)
	if err != nil {
		return PlanDetail{}, err
	}
	priceMinor, currency, trialDays, err := normalizePricing(interval, in.PriceMinor, in.Currency, in.TrialDays)
	if err != nil {
		return PlanDetail{}, err
	}
	if err := s.validateLimits(ctx, in.Limits); err != nil {
		return PlanDetail{}, err
	}
	var row db.Plan
	err = s.withTx(ctx, func(q *db.Queries) error {
		var err error
		row, err = q.CreatePlan(ctx, db.CreatePlanParams{
			Slug: in.Slug, Name: in.Name, Description: textNarg(in.Description),
			BillingInterval: interval, IsActive: in.IsActive, SortOrder: in.SortOrder,
			PriceMinor: priceMinor, Currency: currency, TrialDays: trialDays,
		})
		if err != nil {
			return err
		}
		return replaceLimits(ctx, q, row.ID, in.Limits)
	})
	if err != nil {
		return PlanDetail{}, err
	}
	return s.Get(ctx, row.Uuid)
}

func (s *Service) Patch(ctx context.Context, planUUID uuid.UUID, in PatchPlanInput) (PlanDetail, error) {
	if in.Name == nil && in.Description == nil && in.BillingInterval == nil &&
		in.PriceMinor == nil && in.Currency == nil && in.TrialDays == nil &&
		in.IsActive == nil && in.SortOrder == nil && in.Limits == nil {
		return PlanDetail{}, fmt.Errorf("%w: at least one field is required", ErrInvalidRequest)
	}
	current, err := s.q.GetPlanByUUID(ctx, planUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanDetail{}, ErrNotFound
		}
		return PlanDetail{}, err
	}
	params := db.UpdatePlanParams{Uuid: planUUID}
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		if v == "" {
			return PlanDetail{}, fmt.Errorf("%w: name cannot be empty", ErrInvalidRequest)
		}
		params.Name = pgtype.Text{String: v, Valid: true}
	}
	if in.Description != nil {
		params.Description = textNarg(in.Description)
	}
	if in.BillingInterval != nil {
		interval, err := parseBillingInterval(*in.BillingInterval)
		if err != nil {
			return PlanDetail{}, err
		}
		params.BillingInterval = db.NullBillingInterval{BillingInterval: interval, Valid: true}
	}
	if in.IsActive != nil {
		params.IsActive = pgtype.Bool{Bool: *in.IsActive, Valid: true}
	}
	if in.SortOrder != nil {
		params.SortOrder = pgtype.Int4{Int32: *in.SortOrder, Valid: true}
	}
	if in.PriceMinor != nil {
		params.PriceMinor = pgtype.Int8{Int64: *in.PriceMinor, Valid: true}
	}
	if in.Currency != nil {
		params.Currency = pgtype.Text{String: strings.TrimSpace(*in.Currency), Valid: true}
	}
	if in.TrialDays != nil {
		params.TrialDays = pgtype.Int4{Int32: *in.TrialDays, Valid: true}
	}
	nextInterval := current.BillingInterval
	if in.BillingInterval != nil {
		nextInterval = params.BillingInterval.BillingInterval
	}
	nextPrice := current.PriceMinor
	if in.PriceMinor != nil {
		nextPrice = *in.PriceMinor
	}
	nextCurrency := current.Currency
	if in.Currency != nil {
		nextCurrency = strings.TrimSpace(*in.Currency)
	}
	nextTrial := current.TrialDays
	if in.TrialDays != nil {
		nextTrial = *in.TrialDays
	}
	if _, _, _, err := normalizePricing(nextInterval, nextPrice, nextCurrency, nextTrial); err != nil {
		return PlanDetail{}, err
	}
	if in.Limits != nil {
		if err := s.validateLimits(ctx, *in.Limits); err != nil {
			return PlanDetail{}, err
		}
	}
	err = s.withTx(ctx, func(q *db.Queries) error {
		row, err := q.UpdatePlan(ctx, params)
		if err != nil {
			return err
		}
		if in.Limits == nil {
			return nil
		}
		return replaceLimits(ctx, q, row.ID, *in.Limits)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanDetail{}, ErrNotFound
		}
		return PlanDetail{}, err
	}
	return s.Get(ctx, planUUID)
}

func (s *Service) Delete(ctx context.Context, planUUID uuid.UUID) error {
	plan, err := s.q.GetPlanByUUID(ctx, planUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if plan.Slug == "free" {
		return fmt.Errorf("%w: free plan cannot be deleted", ErrConflict)
	}
	if err := s.q.DeletePlan(ctx, planUUID); err != nil {
		return err
	}
	return nil
}

func (s *Service) listLimits(ctx context.Context, planID int64) ([]PlanLimit, error) {
	rows, err := s.q.ListPlanLimitsByPlanID(ctx, planID)
	if err != nil {
		return nil, err
	}
	out := make([]PlanLimit, 0, len(rows))
	for _, row := range rows {
		pl := PlanLimit{
			FeatureKey: row.FeatureKey,
			ValueType:  string(row.ValueType),
			Label:      row.Label,
		}
		if row.FeatureDescription.Valid {
			v := row.FeatureDescription.String
			pl.Description = &v
		}
		if row.ValueInt.Valid {
			v := row.ValueInt.Int32
			pl.IntValue = &v
		}
		if row.ValueBool.Valid {
			v := row.ValueBool.Bool
			pl.BoolValue = &v
		}
		out = append(out, pl)
	}
	return out, nil
}

// withTx runs fn on queries bound to one transaction, so a plan and its limits are written together.
func (s *Service) withTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// replaceLimits replaces the plan's whole limit set. Callers validate it first (every feature
// present) and run it in the same transaction as the plan write.
func replaceLimits(ctx context.Context, q *db.Queries, planID int64, limits []PlanLimitInput) error {
	if err := q.DeletePlanLimitsByPlanID(ctx, planID); err != nil {
		return err
	}
	for _, lim := range limits {
		var intVal pgtype.Int4
		var boolVal pgtype.Bool
		switch {
		case lim.IntValue != nil:
			intVal = pgtype.Int4{Int32: int32(*lim.IntValue), Valid: true} //nolint:gosec // range checked by ValidateLimits
		case lim.BoolValue != nil:
			boolVal = pgtype.Bool{Bool: *lim.BoolValue, Valid: true}
		default:
			return fmt.Errorf("%w: limit %s requires a value", ErrInvalidRequest, lim.FeatureKey)
		}
		if err := q.UpsertPlanLimit(ctx, db.UpsertPlanLimitParams{
			PlanID: planID, FeatureKey: strings.TrimSpace(lim.FeatureKey), ValueInt: intVal, ValueBool: boolVal,
		}); err != nil {
			return err
		}
	}
	return nil
}

func mapPlanSummary(row db.Plan) PlanSummary {
	var desc *string
	if row.Description.Valid {
		v := row.Description.String
		desc = &v
	}
	return PlanSummary{
		UUID: row.Uuid, Slug: row.Slug, Name: row.Name, Description: desc,
		BillingInterval: string(row.BillingInterval), IsActive: row.IsActive,
		PriceMinor: row.PriceMinor, Currency: row.Currency, TrialDays: row.TrialDays,
		SortOrder: row.SortOrder,
		CreatedAt: row.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: row.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func normalizePricing(interval db.BillingInterval, priceMinor int64, currency string, trialDays int32) (int64, string, int32, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "TRY"
	}
	if currency != "TRY" {
		return 0, "", 0, fmt.Errorf("%w: only TRY currency is supported", ErrInvalidRequest)
	}
	if trialDays < 0 {
		return 0, "", 0, fmt.Errorf("%w: trial_days cannot be negative", ErrInvalidRequest)
	}
	if interval == db.BillingIntervalFree {
		if priceMinor != 0 {
			return 0, "", 0, fmt.Errorf("%w: free plans must have price_minor 0", ErrInvalidRequest)
		}
		return 0, currency, trialDays, nil
	}
	if priceMinor <= 0 {
		return 0, "", 0, fmt.Errorf("%w: paid plans require price_minor > 0", ErrInvalidRequest)
	}
	return priceMinor, currency, trialDays, nil
}

func parseBillingInterval(v string) (db.BillingInterval, error) {
	switch db.BillingInterval(v) {
	case db.BillingIntervalFree, db.BillingIntervalMonth, db.BillingIntervalYear, db.BillingIntervalLifetime:
		return db.BillingInterval(v), nil
	default:
		return "", fmt.Errorf("%w: invalid billing_interval", ErrInvalidRequest)
	}
}

func textNarg(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
