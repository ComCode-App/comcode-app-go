package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/model"
)

type PostgresRepository struct {
	q *db.Queries
}

func NewPostgres(q *db.Queries) *PostgresRepository {
	return &PostgresRepository{q: q}
}

func (r *PostgresRepository) ListAccountCards(ctx context.Context, accountID int64, q *string, isActive *bool, limit, offset int32) ([]model.CardSummary, int64, error) {
	rows, err := r.q.ListAccountCards(ctx, db.ListAccountCardsParams{
		AccountID:   accountID,
		Q:           textToPgtype(q),
		IsActive:    boolToPgtype(isActive),
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := r.q.CountAccountCards(ctx, db.CountAccountCardsParams{
		AccountID: accountID,
		Q:         textToPgtype(q),
		IsActive:  boolToPgtype(isActive),
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]model.CardSummary, 0, len(rows))
	for _, row := range rows {
		var customData map[string]any
		if len(row.CustomData) > 0 {
			_ = json.Unmarshal(row.CustomData, &customData)
		}
		if customData == nil {
			customData = make(map[string]any)
		}

		var profUUID *uuid.UUID
		if row.ProfileUuid.Valid {
			u := uuid.UUID(row.ProfileUuid.Bytes)
			profUUID = &u
		}

		summary := model.CardSummary{
			UUID:               row.Uuid,
			Name:               row.Name,
			CardUID:            pgtypeToString(row.CardUid),
			SmartCode:          row.SmartCode,
			TargetType:         row.TargetType,
			ProfileUUID:        profUUID,
			ProfileAvatarURL:   pgtypeToString(row.ProfileAvatarUrl),
			ProfileLocale:      pgtypeToString(row.ProfilePrimaryLocale),
			DirectURL:          row.DirectUrl,
			CustomData:         customData,
			CardType:           row.CardType,
			Material:           row.Material,
			ColorHex:           row.ColorHex,
			IsActive:           row.IsActive,
			IsLocked:           row.IsLocked,
			LeadCaptureEnabled: row.LeadCaptureEnabled,
			HasPasscode:        row.Passcode.Valid && row.Passcode.String != "",
			CreatedAt:          row.CreatedAt.Time,
			UpdatedAt:          row.UpdatedAt.Time,
			RouteUUID:          optionalUUID(row.RouteUuid),
		}

		items = append(items, summary)
	}

	return items, total, nil
}

func (r *PostgresRepository) GetAccountCardByUUID(ctx context.Context, id uuid.UUID, accountID int64) (*model.CardDetail, error) {
	row, err := r.q.GetAccountCardByUUID(ctx, db.GetAccountCardByUUIDParams{
		Uuid:      id,
		AccountID: accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var customData map[string]any
	if len(row.CustomData) > 0 {
		_ = json.Unmarshal(row.CustomData, &customData)
	}
	if customData == nil {
		customData = make(map[string]any)
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

	detail := &model.CardDetail{
		ID:                 row.ID,
		UUID:               row.Uuid,
		UserID:             row.UserID,
		Name:               row.Name,
		CardUID:            pgtypeToString(row.CardUid),
		ActivationCode:     pgtypeToString(row.ActivationCode),
		SmartCode:          row.SmartCode,
		TargetType:         row.TargetType,
		ProfileID:          profID,
		ProfileUUID:        profUUID,
		ProfileAvatarURL:   pgtypeToString(row.ProfileAvatarUrl),
		ProfileLocale:      pgtypeToString(row.ProfilePrimaryLocale),
		DirectURL:          row.DirectUrl,
		CustomData:         customData,
		CardType:           row.CardType,
		Material:           row.Material,
		ColorHex:           row.ColorHex,
		IsActive:           row.IsActive,
		IsLocked:           row.IsLocked,
		LeadCaptureEnabled: row.LeadCaptureEnabled,
		HasPasscode:        row.Passcode.Valid && row.Passcode.String != "",
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}

	return detail, nil
}

func (r *PostgresRepository) GetCardBySmartCode(ctx context.Context, smartCode string) (*model.CardDetail, error) {
	row, err := r.q.GetCardBySmartCode(ctx, smartCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var customData map[string]any
	if len(row.CustomData) > 0 {
		_ = json.Unmarshal(row.CustomData, &customData)
	}
	if customData == nil {
		customData = make(map[string]any)
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

	var passcode *string
	if row.Passcode.Valid {
		passcode = &row.Passcode.String
	}

	detail := &model.CardDetail{
		ID:                 row.ID,
		UUID:               row.Uuid,
		UserID:             row.UserID,
		Name:               row.Name,
		SmartCode:          row.SmartCode,
		TargetType:         row.TargetType,
		ProfileID:          profID,
		ProfileUUID:        profUUID,
		ProfileAvatarURL:   pgtypeToString(row.ProfileAvatarUrl),
		ProfileLocale:      pgtypeToString(row.ProfilePrimaryLocale),
		DirectURL:          row.DirectUrl,
		CustomData:         customData,
		CardType:           row.CardType,
		Material:           row.Material,
		ColorHex:           row.ColorHex,
		IsActive:           row.IsActive,
		IsLocked:           row.IsLocked,
		LeadCaptureEnabled: row.LeadCaptureEnabled,
		HasPasscode:        row.Passcode.Valid && row.Passcode.String != "",
		Passcode:           passcode,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}

	return detail, nil
}

func (r *PostgresRepository) GetCardByActivationCode(ctx context.Context, code string) (*model.CardDetail, error) {
	c, err := r.q.GetCardByActivationCode(ctx, pgtype.Text{String: code, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &model.CardDetail{
		ID:             c.ID,
		UUID:           c.Uuid,
		UserID:         c.UserID,
		Name:           c.Name,
		CardUID:        pgtypeToString(c.CardUid),
		ActivationCode: pgtypeToString(c.ActivationCode),
		SmartCode:      c.SmartCode,
		TargetType:     c.TargetType,
		CardType:       c.CardType,
		Material:       c.Material,
		ColorHex:       c.ColorHex,
		IsActive:       c.IsActive,
		IsLocked:       c.IsLocked,
		CreatedAt:      c.CreatedAt.Time,
		UpdatedAt:      c.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) CreateCard(ctx context.Context, accountID, creatorUserID int64, smartCode string, input model.UpsertCardInput, profileID *int64) (*model.CardDetail, error) {
	customJSON := []byte("{}")
	if input.CustomData != nil {
		customJSON, _ = json.Marshal(input.CustomData)
	}

	cardType := "physical_nfc"
	if input.CardType != "" {
		cardType = input.CardType
	}

	material := "matte_black"
	if input.Material != "" {
		material = input.Material
	}

	colorHex := "#18181b"
	if input.ColorHex != "" {
		colorHex = input.ColorHex
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	isLocked := false
	if input.IsLocked != nil {
		isLocked = *input.IsLocked
	}

	leadCapture := false
	if input.LeadCaptureEnabled != nil {
		leadCapture = *input.LeadCaptureEnabled
	}

	row, err := r.q.CreateCard(ctx, db.CreateCardParams{
		AccountID:          accountID,
		UserID:             creatorUserID,
		Name:               input.Name,
		CardUid:            textToPgtype(input.CardUID),
		ActivationCode:     pgtype.Text{Valid: false},
		SmartCode:          smartCode,
		TargetType:         input.TargetType,
		ProfileID:          int64ToPgtype(profileID),
		DirectUrl:          input.DirectURL,
		CustomData:         customJSON,
		CardType:           cardType,
		Material:           material,
		ColorHex:           colorHex,
		IsActive:           isActive,
		IsLocked:           isLocked,
		LeadCaptureEnabled: leadCapture,
		Passcode:           textToPgtype(input.Passcode),
	})
	if err != nil {
		return nil, err
	}

	return &model.CardDetail{
		ID:                 row.ID,
		UUID:               row.Uuid,
		UserID:             row.UserID,
		Name:               row.Name,
		CardUID:            pgtypeToString(row.CardUid),
		SmartCode:          row.SmartCode,
		TargetType:         row.TargetType,
		ProfileID:          profileID,
		DirectURL:          row.DirectUrl,
		CardType:           row.CardType,
		Material:           row.Material,
		ColorHex:           row.ColorHex,
		IsActive:           row.IsActive,
		IsLocked:           row.IsLocked,
		LeadCaptureEnabled: row.LeadCaptureEnabled,
		HasPasscode:        row.Passcode.Valid && row.Passcode.String != "",
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) UpdateCard(ctx context.Context, id uuid.UUID, accountID int64, input model.UpsertCardInput, profileID *int64, setProfileID bool) (*model.CardDetail, error) {
	var customJSON []byte
	if input.CustomData != nil {
		customJSON, _ = json.Marshal(input.CustomData)
	}

	var name *string
	if input.Name != "" {
		name = &input.Name
	}

	var targetType *string
	if input.TargetType != "" {
		targetType = &input.TargetType
	}

	var directURL *string
	if input.DirectURL != "" {
		directURL = &input.DirectURL
	}

	var cardType *string
	if input.CardType != "" {
		cardType = &input.CardType
	}

	var material *string
	if input.Material != "" {
		material = &input.Material
	}

	var colorHex *string
	if input.ColorHex != "" {
		colorHex = &input.ColorHex
	}

	setPasscode := input.Passcode != nil

	row, err := r.q.UpdateCard(ctx, db.UpdateCardParams{
		Uuid:               id,
		AccountID:          accountID,
		Name:               textToPgtype(name),
		TargetType:         textToPgtype(targetType),
		SetProfileID:       setProfileID,
		ProfileID:          int64ToPgtype(profileID),
		DirectUrl:          textToPgtype(directURL),
		CustomData:         customJSON,
		CardType:           textToPgtype(cardType),
		Material:           textToPgtype(material),
		ColorHex:           textToPgtype(colorHex),
		IsActive:           boolToPgtype(input.IsActive),
		IsLocked:           boolToPgtype(input.IsLocked),
		LeadCaptureEnabled: boolToPgtype(input.LeadCaptureEnabled),
		SetPasscode:        setPasscode,
		Passcode:           textToPgtype(input.Passcode),
	})
	if err != nil {
		return nil, err
	}

	var profID *int64
	if row.ProfileID.Valid {
		profID = &row.ProfileID.Int64
	}

	return &model.CardDetail{
		ID:                 row.ID,
		UUID:               row.Uuid,
		UserID:             row.UserID,
		Name:               row.Name,
		CardUID:            pgtypeToString(row.CardUid),
		SmartCode:          row.SmartCode,
		TargetType:         row.TargetType,
		ProfileID:          profID,
		DirectURL:          row.DirectUrl,
		CardType:           row.CardType,
		Material:           row.Material,
		ColorHex:           row.ColorHex,
		IsActive:           row.IsActive,
		IsLocked:           row.IsLocked,
		LeadCaptureEnabled: row.LeadCaptureEnabled,
		HasPasscode:        row.Passcode.Valid && row.Passcode.String != "",
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) ToggleCardLock(ctx context.Context, id uuid.UUID, accountID int64) (bool, error) {
	return r.q.ToggleCardLock(ctx, db.ToggleCardLockParams{
		Uuid:      id,
		AccountID: accountID,
	})
}

func (r *PostgresRepository) ActivateCardByCode(ctx context.Context, code string, accountID, userID int64, name *string, profileID *int64) (*model.CardDetail, error) {
	row, err := r.q.ActivateCardByCode(ctx, db.ActivateCardByCodeParams{
		ActivationCode: pgtype.Text{String: code, Valid: true},
		AccountID:      accountID,
		UserID:         userID,
		Name:           textToPgtype(name),
		ProfileID:      int64ToPgtype(profileID),
	})
	if err != nil {
		return nil, err
	}

	var profID *int64
	if row.ProfileID.Valid {
		profID = &row.ProfileID.Int64
	}

	return &model.CardDetail{
		ID:                 row.ID,
		UUID:               row.Uuid,
		UserID:             row.UserID,
		Name:               row.Name,
		CardUID:            pgtypeToString(row.CardUid),
		SmartCode:          row.SmartCode,
		TargetType:         row.TargetType,
		ProfileID:          profID,
		DirectURL:          row.DirectUrl,
		CardType:           row.CardType,
		Material:           row.Material,
		ColorHex:           row.ColorHex,
		IsActive:           row.IsActive,
		IsLocked:           row.IsLocked,
		LeadCaptureEnabled: row.LeadCaptureEnabled,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) DeleteCard(ctx context.Context, id uuid.UUID, accountID int64) error {
	return r.q.DeleteCard(ctx, db.DeleteCardParams{
		Uuid:      id,
		AccountID: accountID,
	})
}

func (r *PostgresRepository) CheckSmartCodeExists(ctx context.Context, code string, excludeUUID *uuid.UUID) (bool, error) {
	var exc pgtype.UUID
	if excludeUUID != nil {
		exc = pgtype.UUID{Bytes: *excludeUUID, Valid: true}
	}
	return r.q.CheckSmartCodeExists(ctx, db.CheckSmartCodeExistsParams{
		SmartCode:   code,
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

// Helpers
func textToPgtype(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func int64ToPgtype(i *int64) pgtype.Int8 {
	if i == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *i, Valid: true}
}

func boolToPgtype(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{Valid: false}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func pgtypeToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// optionalUUID turns a zero UUID (NULL subquery) into nil.
func optionalUUID(u uuid.UUID) *uuid.UUID {
	if u == uuid.Nil {
		return nil
	}
	return &u
}
