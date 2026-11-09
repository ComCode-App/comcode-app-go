package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventStore handles raw immutable event persistence.
type EventStore interface {
	Store(ctx context.Context, event domain.Event) (*domain.Event, error)
	StoreBatch(ctx context.Context, events []domain.Event) ([]domain.Event, error)
}

// PostgresStore implements EventStore backed by partitioned PostgreSQL tables.
type PostgresStore struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewPostgresStore creates a PostgresStore.
func NewPostgresStore(pool *pgxpool.Pool, q *db.Queries) *PostgresStore {
	return &PostgresStore{
		pool: pool,
		q:    q,
	}
}

// Store inserts a single raw event into the partitioned events table.
func (s *PostgresStore) Store(ctx context.Context, event domain.Event) (*domain.Event, error) {
	propsJSON, err := json.Marshal(event.Properties)
	if err != nil || len(event.Properties) == 0 {
		propsJSON = []byte("{}")
	}

	contextJSON, err := json.Marshal(event.Context)
	if err != nil {
		contextJSON = []byte("{}")
	}

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	var actorType, actorID pgtype.Text
	if event.Actor != nil {
		actorType = pgtype.Text{String: event.Actor.Type, Valid: event.Actor.Type != ""}
		actorID = pgtype.Text{String: event.Actor.ID, Valid: event.Actor.ID != ""}
	}

	var sessionID, requestID, ipHash pgtype.Text
	if event.SessionID != "" {
		sessionID = pgtype.Text{String: event.SessionID, Valid: true}
	}
	if event.RequestID != "" {
		requestID = pgtype.Text{String: event.RequestID, Valid: true}
	}
	if event.Context.IPHash != "" {
		ipHash = pgtype.Text{String: event.Context.IPHash, Valid: true}
	}

	var lat, lon pgtype.Float8
	if event.Context.Latitude != nil {
		lat = pgtype.Float8{Float64: *event.Context.Latitude, Valid: true}
	}
	if event.Context.Longitude != nil {
		lon = pgtype.Float8{Float64: *event.Context.Longitude, Valid: true}
	}

	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	schemaVersion := int32(event.SchemaVersion)
	if schemaVersion <= 0 {
		schemaVersion = 1
	}

	row, err := s.q.InsertEvent(ctx, db.InsertEventParams{
		EventType:      event.Type,
		SchemaVersion:  schemaVersion,
		SubjectType:    event.Subject.Type,
		SubjectID:      event.Subject.ID,
		ActorType:      actorType,
		ActorID:        actorID,
		SessionID:      sessionID,
		RequestID:      requestID,
		IpHash:         ipHash,
		CountryCode:    pgtype.Text{String: event.Context.CountryCode, Valid: event.Context.CountryCode != ""},
		CountryName:    pgtype.Text{String: event.Context.CountryName, Valid: event.Context.CountryName != ""},
		City:           pgtype.Text{String: event.Context.City, Valid: event.Context.City != ""},
		Timezone:       pgtype.Text{String: event.Context.Timezone, Valid: event.Context.Timezone != ""},
		Latitude:       lat,
		Longitude:      lon,
		DeviceType:     pgtype.Text{String: event.Context.DeviceType, Valid: event.Context.DeviceType != ""},
		DeviceBrand:    pgtype.Text{String: event.Context.DeviceBrand, Valid: event.Context.DeviceBrand != ""},
		DeviceModel:    pgtype.Text{String: event.Context.DeviceModel, Valid: event.Context.DeviceModel != ""},
		Browser:        pgtype.Text{String: event.Context.Browser, Valid: event.Context.Browser != ""},
		BrowserVersion: pgtype.Text{String: event.Context.BrowserVersion, Valid: event.Context.BrowserVersion != ""},
		Os:             pgtype.Text{String: event.Context.OS, Valid: event.Context.OS != ""},
		OsVersion:      pgtype.Text{String: event.Context.OSVersion, Valid: event.Context.OSVersion != ""},
		Referer:        pgtype.Text{String: event.Context.Referer, Valid: event.Context.Referer != ""},
		UserAgent:      pgtype.Text{String: event.Context.UserAgent, Valid: event.Context.UserAgent != ""},
		Properties:     propsJSON,
		Context:        contextJSON,
		Metadata:       metadataJSON,
		OccurredAt:     pgtype.Timestamptz{Time: occurredAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	event.ID = row.ID
	event.CreatedAt = row.CreatedAt.Time
	return &event, nil
}

// StoreBatch inserts a slice of events within a transaction.
func (s *PostgresStore) StoreBatch(ctx context.Context, events []domain.Event) ([]domain.Event, error) {
	if len(events) == 0 {
		return nil, nil
	}

	result := make([]domain.Event, 0, len(events))
	for _, ev := range events {
		stored, err := s.Store(ctx, ev)
		if err != nil {
			return nil, err
		}
		result = append(result, *stored)
	}
	return result, nil
}

// StoreEvent stores a single event and satisfies DirectEventStore.
func (s *PostgresStore) StoreEvent(ctx context.Context, event domain.Event) error {
	_, err := s.Store(ctx, event)
	return err
}
