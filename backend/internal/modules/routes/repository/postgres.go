package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PostgresRepository struct {
	q *db.Queries
}

func NewPostgres(q *db.Queries) *PostgresRepository {
	return &PostgresRepository{q: q}
}

func (r *PostgresRepository) GetRouteByPath(ctx context.Context, path string) (*model.RouteDetail, error) {
	row, err := r.q.GetRouteByPath(ctx, path)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.mapRouteByPathRowToDetail(row), nil
}

func (r *PostgresRepository) GetRouteByUUID(ctx context.Context, id uuid.UUID) (*model.RouteDetail, error) {
	row, err := r.q.GetRouteByUUID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.mapRouteByUUIDRowToDetail(row), nil
}

func (r *PostgresRepository) GetRouteByCardID(ctx context.Context, cardID int64) (*model.RouteDetail, error) {
	row, err := r.q.GetRouteByCardID(ctx, pgtype.Int8{Int64: cardID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var presConfig map[string]any
	if len(row.PresentationConfig) > 0 {
		_ = json.Unmarshal(row.PresentationConfig, &presConfig)
	}
	if presConfig == nil {
		presConfig = make(map[string]any)
	}

	var legMeta map[string]any
	if len(row.LegacyMetadata) > 0 {
		_ = json.Unmarshal(row.LegacyMetadata, &legMeta)
	}
	if legMeta == nil {
		legMeta = make(map[string]any)
	}

	var targetUUID *uuid.UUID
	if row.TargetID.Valid {
		u := uuid.UUID(row.TargetID.Bytes)
		targetUUID = &u
	}

	var profID *int64
	if row.ProfileID.Valid {
		profID = &row.ProfileID.Int64
	}

	var cID *int64
	if row.CardID.Valid {
		cID = &row.CardID.Int64
	}

	var ownerID *int64
	if row.OwnerUserID.Valid {
		ownerID = &row.OwnerUserID.Int64
	}

	detail := &model.RouteDetail{
		ID:                 row.ID,
		UUID:               row.Uuid,
		Path:               row.Path,
		RouteType:          row.RouteType,
		Status:             row.Status,
		IsImmutable:        row.IsImmutable,
		TargetType:         row.TargetType,
		TargetID:           targetUUID,
		ProfileID:          profID,
		CardID:             cID,
		DirectURL:          row.DirectUrl,
		AccountID:          pgInt8Ptr(row.AccountID),
		OwnerUserID:        ownerID,
		PresentationConfig: presConfig,
		LegacyMetadata:     legMeta,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}

	return detail, nil
}

func (r *PostgresRepository) ListRoutes(ctx context.Context, accountID int64, routeType, status, q *string, limit, offset int32) ([]model.RouteSummary, int64, error) {
	rows, err := r.q.ListRoutes(ctx, db.ListRoutesParams{
		AccountID:   pgtype.Int8{Int64: accountID, Valid: true},
		RouteType:   textToPgtype(routeType),
		Status:      textToPgtype(status),
		Q:           textToPgtype(q),
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := r.q.CountRoutes(ctx, db.CountRoutesParams{
		AccountID: pgtype.Int8{Int64: accountID, Valid: true},
		RouteType: textToPgtype(routeType),
		Status:    textToPgtype(status),
		Q:         textToPgtype(q),
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]model.RouteSummary, 0, len(rows))
	for _, row := range rows {
		var profUUID *uuid.UUID
		if row.ProfileUuid.Valid {
			u := uuid.UUID(row.ProfileUuid.Bytes)
			profUUID = &u
		}

		var cardUUID *uuid.UUID
		if row.CardUuid.Valid {
			u := uuid.UUID(row.CardUuid.Bytes)
			cardUUID = &u
		}

		summary := model.RouteSummary{
			UUID:        row.Uuid,
			Path:        row.Path,
			RouteType:   row.RouteType,
			Status:      row.Status,
			IsImmutable: row.IsImmutable,
			TargetType:  row.TargetType,
			ProfileUUID: profUUID,
			CardUUID:    cardUUID,
			DirectURL:   row.DirectUrl,
			CreatedAt:   row.CreatedAt.Time,
			UpdatedAt:   row.UpdatedAt.Time,
		}

		items = append(items, summary)
	}

	return items, total, nil
}

func (r *PostgresRepository) CreateRoute(ctx context.Context, input model.CreateReservedRouteInput) (*model.RouteDetail, error) {
	presJSON := []byte("{}")
	if input.PresentationConfig != nil {
		presJSON, _ = json.Marshal(input.PresentationConfig)
	}

	legJSON := []byte("{}")
	if input.LegacyMetadata != nil {
		legJSON, _ = json.Marshal(input.LegacyMetadata)
	}

	row, err := r.q.CreateRoute(ctx, db.CreateRouteParams{
		Path:               input.Path,
		RouteType:          input.RouteType,
		Status:             input.Status,
		IsImmutable:        input.IsImmutable,
		TargetType:         input.TargetType,
		ProfileID:          int64ToPgtype(input.ProfileID),
		CardID:             int64ToPgtype(input.CardID),
		DirectUrl:          input.DirectURL,
		AccountID:          int64ToPgtype(input.AccountID),
		OwnerUserID:        int64ToPgtype(input.OwnerUserID),
		PresentationConfig: presJSON,
		LegacyMetadata:     legJSON,
	})
	if err != nil {
		return nil, err
	}

	var profID *int64
	if row.ProfileID.Valid {
		profID = &row.ProfileID.Int64
	}

	var cardID *int64
	if row.CardID.Valid {
		cardID = &row.CardID.Int64
	}

	var ownerID *int64
	if row.OwnerUserID.Valid {
		ownerID = &row.OwnerUserID.Int64
	}

	return &model.RouteDetail{
		ID:                 row.ID,
		UUID:               row.Uuid,
		Path:               row.Path,
		RouteType:          row.RouteType,
		Status:             row.Status,
		IsImmutable:        row.IsImmutable,
		TargetType:         row.TargetType,
		ProfileID:          profID,
		CardID:             cardID,
		DirectURL:          row.DirectUrl,
		AccountID:          pgInt8Ptr(row.AccountID),
		OwnerUserID:        ownerID,
		PresentationConfig: input.PresentationConfig,
		LegacyMetadata:     input.LegacyMetadata,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) UpdateRoute(ctx context.Context, id uuid.UUID, accountID int64, input model.UpdateRouteInput, profileID, cardID *int64, setProfileID, setCardID bool) (*model.RouteDetail, error) {
	var presJSON []byte
	if input.PresentationConfig != nil {
		presJSON, _ = json.Marshal(input.PresentationConfig)
	}

	row, err := r.q.UpdateRoute(ctx, db.UpdateRouteParams{
		Uuid:               id,
		AccountID:          pgtype.Int8{Int64: accountID, Valid: true},
		Path:               textToPgtype(input.Path),
		TargetType:         textToPgtype(input.TargetType),
		SetProfileID:       setProfileID,
		ProfileID:          int64ToPgtype(profileID),
		SetCardID:          setCardID,
		CardID:             int64ToPgtype(cardID),
		DirectUrl:          textToPgtype(input.DirectURL),
		Status:             textToPgtype(input.Status),
		PresentationConfig: presJSON,
	})
	if err != nil {
		return nil, err
	}

	var presConfig map[string]any
	if len(row.PresentationConfig) > 0 {
		_ = json.Unmarshal(row.PresentationConfig, &presConfig)
	}
	if presConfig == nil {
		presConfig = make(map[string]any)
	}

	var legMeta map[string]any
	if len(row.LegacyMetadata) > 0 {
		_ = json.Unmarshal(row.LegacyMetadata, &legMeta)
	}
	if legMeta == nil {
		legMeta = make(map[string]any)
	}

	var profID *int64
	if row.ProfileID.Valid {
		profID = &row.ProfileID.Int64
	}

	var cID *int64
	if row.CardID.Valid {
		cID = &row.CardID.Int64
	}

	var ownerID *int64
	if row.OwnerUserID.Valid {
		ownerID = &row.OwnerUserID.Int64
	}

	return &model.RouteDetail{
		ID:                 row.ID,
		UUID:               row.Uuid,
		Path:               row.Path,
		RouteType:          row.RouteType,
		Status:             row.Status,
		IsImmutable:        row.IsImmutable,
		TargetType:         row.TargetType,
		ProfileID:          profID,
		CardID:             cID,
		DirectURL:          row.DirectUrl,
		AccountID:          pgInt8Ptr(row.AccountID),
		OwnerUserID:        ownerID,
		PresentationConfig: presConfig,
		LegacyMetadata:     legMeta,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) DeleteRoute(ctx context.Context, id uuid.UUID, accountID int64) error {
	return r.q.DeleteRoute(ctx, db.DeleteRouteParams{
		Uuid:      id,
		AccountID: pgtype.Int8{Int64: accountID, Valid: true},
	})
}

func (r *PostgresRepository) CheckPathExists(ctx context.Context, path string, excludeUUID *uuid.UUID) (bool, error) {
	if !strings.Contains(path, "/") {
		legacy, err := r.q.GetRouteByPathAnyType(ctx, "r/legacy/"+strings.ToLower(strings.TrimSpace(path)))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return false, err
		}
		if err == nil && legacy.RouteType == model.RouteTypeLegacyRoute {
			return true, nil
		}
	}
	var exc pgtype.UUID
	if excludeUUID != nil {
		exc = pgtype.UUID{Bytes: *excludeUUID, Valid: true}
	}
	return r.q.CheckPathExists(ctx, db.CheckPathExistsParams{
		Path:        path,
		ExcludeUuid: exc,
	})
}

// GetProfileIDByUUID resolves a profile of the given account; profiles of other accounts are not found.
func (r *PostgresRepository) GetProfileIDByUUID(ctx context.Context, profileUUID uuid.UUID, accountID int64) (*int64, error) {
	prof, err := r.q.GetProfileByUUIDForAccount(ctx, db.GetProfileByUUIDForAccountParams{Uuid: profileUUID, AccountID: accountID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	id := prof.ID
	return &id, nil
}

func (r *PostgresRepository) mapRouteByPathRowToDetail(row db.GetRouteByPathRow) *model.RouteDetail {
	var presConfig map[string]any
	if len(row.PresentationConfig) > 0 {
		_ = json.Unmarshal(row.PresentationConfig, &presConfig)
	}
	if presConfig == nil {
		presConfig = make(map[string]any)
	}

	var legMeta map[string]any
	if len(row.LegacyMetadata) > 0 {
		_ = json.Unmarshal(row.LegacyMetadata, &legMeta)
	}
	if legMeta == nil {
		legMeta = make(map[string]any)
	}

	var profUUID *uuid.UUID
	if row.ProfileUuid.Valid {
		u := uuid.UUID(row.ProfileUuid.Bytes)
		profUUID = &u
	}

	var profID *int64
	if row.ProfileID.Valid {
		profID = &row.ProfileID.Int64
	}

	var cardUUID *uuid.UUID
	if row.CardUuid.Valid {
		u := uuid.UUID(row.CardUuid.Bytes)
		cardUUID = &u
	}

	var cardID *int64
	if row.CardID.Valid {
		cardID = &row.CardID.Int64
	}

	var ownerID *int64
	if row.OwnerUserID.Valid {
		ownerID = &row.OwnerUserID.Int64
	}

	detail := &model.RouteDetail{
		ID:                 row.ID,
		UUID:               row.Uuid,
		Path:               row.Path,
		RouteType:          row.RouteType,
		Status:             row.Status,
		IsImmutable:        row.IsImmutable,
		TargetType:         row.TargetType,
		ProfileID:          profID,
		ProfileUUID:        profUUID,
		ProfileAvatarURL:   pgtypeToString(row.ProfileAvatarUrl),
		ProfileLocale:      pgtypeToString(row.ProfilePrimaryLocale),
		ProfileUsername:    row.ProfileUsername,
		CardID:             cardID,
		CardUUID:           cardUUID,
		CardName:           pgtypeToString(row.CardName),
		CardIsLocked:       row.CardIsLocked.Valid && row.CardIsLocked.Bool,
		CardIsActive:       row.CardIsActive.Valid && row.CardIsActive.Bool,
		CardLeadCapture:    row.CardLeadCaptureEnabled.Valid && row.CardLeadCaptureEnabled.Bool,
		CardHasPasscode:    row.CardPasscode.Valid && row.CardPasscode.String != "",
		DirectURL:          row.DirectUrl,
		AccountID:          pgInt8Ptr(row.AccountID),
		OwnerUserID:        ownerID,
		PresentationConfig: presConfig,
		LegacyMetadata:     legMeta,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}

	// A card route follows its card: target and direct URL come from the card, not the copy
	// stored on the route when it was created.
	if row.CardTargetType.Valid && row.CardTargetType.String != "" {
		detail.TargetType = row.CardTargetType.String
		detail.DirectURL = row.CardDirectUrl.String
	}

	return detail
}

func (r *PostgresRepository) mapRouteByUUIDRowToDetail(row db.GetRouteByUUIDRow) *model.RouteDetail {
	var presConfig map[string]any
	if len(row.PresentationConfig) > 0 {
		_ = json.Unmarshal(row.PresentationConfig, &presConfig)
	}
	if presConfig == nil {
		presConfig = make(map[string]any)
	}

	var legMeta map[string]any
	if len(row.LegacyMetadata) > 0 {
		_ = json.Unmarshal(row.LegacyMetadata, &legMeta)
	}
	if legMeta == nil {
		legMeta = make(map[string]any)
	}

	var profUUID *uuid.UUID
	if row.ProfileUuid.Valid {
		u := uuid.UUID(row.ProfileUuid.Bytes)
		profUUID = &u
	}

	var profID *int64
	if row.ProfileID.Valid {
		profID = &row.ProfileID.Int64
	}

	var cardUUID *uuid.UUID
	if row.CardUuid.Valid {
		u := uuid.UUID(row.CardUuid.Bytes)
		cardUUID = &u
	}

	var cardID *int64
	if row.CardID.Valid {
		cardID = &row.CardID.Int64
	}

	var ownerID *int64
	if row.OwnerUserID.Valid {
		ownerID = &row.OwnerUserID.Int64
	}

	detail := &model.RouteDetail{
		ID:                 row.ID,
		UUID:               row.Uuid,
		Path:               row.Path,
		RouteType:          row.RouteType,
		Status:             row.Status,
		IsImmutable:        row.IsImmutable,
		TargetType:         row.TargetType,
		ProfileID:          profID,
		ProfileUUID:        profUUID,
		ProfileAvatarURL:   pgtypeToString(row.ProfileAvatarUrl),
		ProfileLocale:      pgtypeToString(row.ProfilePrimaryLocale),
		CardID:             cardID,
		CardUUID:           cardUUID,
		CardName:           pgtypeToString(row.CardName),
		DirectURL:          row.DirectUrl,
		AccountID:          pgInt8Ptr(row.AccountID),
		OwnerUserID:        ownerID,
		PresentationConfig: presConfig,
		LegacyMetadata:     legMeta,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}

	return detail
}

// Helpers
func textToPgtype(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func pgInt8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	id := v.Int64
	return &id
}

func int64ToPgtype(i *int64) pgtype.Int8 {
	if i == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *i, Valid: true}
}

func pgtypeToString(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

// RecordPathChange appends to route_history (TEC-558).
func (r *PostgresRepository) RecordPathChange(ctx context.Context, c model.PathChange) error {
	return r.q.InsertRouteHistory(ctx, db.InsertRouteHistoryParams{
		RouteID:         c.RouteID,
		RouteType:       c.RouteType,
		AccountID:       int8Ptr(c.AccountID),
		OldPath:         c.OldPath,
		NewPath:         c.NewPath,
		ChangedByUserID: int8Ptr(c.ChangedByUserID),
		Reason:          c.Reason,
	})
}

func int8Ptr(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
