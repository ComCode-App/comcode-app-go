package registry

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Store provides access and management of event definitions.
type Store interface {
	domain.Registry
	List(ctx context.Context, category *string, enabled *bool, isPublic *bool) ([]domain.EventDefinition, error)
	GetByID(ctx context.Context, id int64) (*domain.EventDefinition, error)
	Create(ctx context.Context, def domain.EventDefinition) (*domain.EventDefinition, error)
	Update(ctx context.Context, def domain.EventDefinitionPatch) (*domain.EventDefinition, error)
	Delete(ctx context.Context, id int64) error
	Refresh(ctx context.Context) error
}

// CachedStore implements Store backed by PostgreSQL and an in-memory RWMutex cache.
type CachedStore struct {
	q     *db.Queries
	mu    sync.RWMutex
	byKey map[string]domain.EventDefinition
	byID  map[int64]domain.EventDefinition
	ttl   time.Duration
	expAt time.Time
}

// NewCachedStore initializes a CachedStore.
func NewCachedStore(q *db.Queries, ttl time.Duration) *CachedStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CachedStore{
		q:     q,
		byKey: make(map[string]domain.EventDefinition),
		byID:  make(map[int64]domain.EventDefinition),
		ttl:   ttl,
	}
}

// Get looks up an event definition by its unique key.
func (s *CachedStore) Get(ctx context.Context, key string) (*domain.EventDefinition, error) {
	s.mu.RLock()
	if time.Now().Before(s.expAt) {
		if def, ok := s.byKey[key]; ok {
			s.mu.RUnlock()
			return &def, nil
		}
	}
	s.mu.RUnlock()

	// If missing or cache expired, refresh cache
	if err := s.Refresh(ctx); err == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		if def, ok := s.byKey[key]; ok {
			return &def, nil
		}
	}

	// Fallback direct DB query
	row, err := s.q.GetEventDefinitionByKey(ctx, key)
	if err != nil {
		return nil, domain.ErrDefinitionNotFound
	}
	def := mapRowToDefinition(row)
	return &def, nil
}

// GetByID looks up an event definition by its numeric ID.
func (s *CachedStore) GetByID(ctx context.Context, id int64) (*domain.EventDefinition, error) {
	s.mu.RLock()
	if time.Now().Before(s.expAt) {
		if def, ok := s.byID[id]; ok {
			s.mu.RUnlock()
			return &def, nil
		}
	}
	s.mu.RUnlock()

	row, err := s.q.GetEventDefinitionByID(ctx, id)
	if err != nil {
		return nil, domain.ErrDefinitionNotFound
	}
	def := mapRowToDefinition(row)
	return &def, nil
}

// List returns all event definitions matching optional filters.
func (s *CachedStore) List(ctx context.Context, category *string, enabled *bool, isPublic *bool) ([]domain.EventDefinition, error) {
	var cat pgtype.Text
	if category != nil {
		cat = pgtype.Text{String: *category, Valid: true}
	}
	var en pgtype.Bool
	if enabled != nil {
		en = pgtype.Bool{Bool: *enabled, Valid: true}
	}
	var pub pgtype.Bool
	if isPublic != nil {
		pub = pgtype.Bool{Bool: *isPublic, Valid: true}
	}

	rows, err := s.q.ListEventDefinitions(ctx, db.ListEventDefinitionsParams{
		Category: cat,
		Enabled:  en,
		IsPublic: pub,
	})
	if err != nil {
		return nil, fmt.Errorf("list event definitions: %w", err)
	}

	result := make([]domain.EventDefinition, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapRowToDefinition(row))
	}
	return result, nil
}

// Create inserts a new event definition.
func (s *CachedStore) Create(ctx context.Context, def domain.EventDefinition) (*domain.EventDefinition, error) {
	row, err := s.q.CreateEventDefinition(ctx, db.CreateEventDefinitionParams{
		Key:                def.Key,
		Name:               def.Name,
		Description:        textOrNull(def.Description),
		Category:           def.Category,
		SchemaVersion:      int32(def.SchemaVersion),
		Enabled:            def.Enabled,
		IsPublic:           def.IsPublic,
		AggregateEnabled:   def.AggregateEnabled,
		RetentionDays:      int32(def.RetentionDays),
		RequiredProperties: def.RequiredProperties,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrDefinitionExists
		}
		return nil, fmt.Errorf("create event definition: %w", err)
	}

	_ = s.Refresh(ctx)
	out := mapRowToDefinition(row)
	return &out, nil
}

// Update modifies an existing event definition.
func (s *CachedStore) Update(ctx context.Context, def domain.EventDefinitionPatch) (*domain.EventDefinition, error) {
	params := db.UpdateEventDefinitionParams{ID: def.ID, RequiredProperties: def.RequiredProperties}
	if def.Name != nil {
		params.Name = pgtype.Text{String: *def.Name, Valid: true}
	}
	if def.Description.Set {
		params.Description = pgtype.Text{String: def.Description.Value, Valid: true}
	}
	if def.Category != nil {
		params.Category = pgtype.Text{String: *def.Category, Valid: true}
	}
	if def.SchemaVersion != nil {
		params.SchemaVersion = pgtype.Int4{Int32: *def.SchemaVersion, Valid: true}
	}
	if def.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *def.Enabled, Valid: true}
	}
	if def.IsPublic != nil {
		params.IsPublic = pgtype.Bool{Bool: *def.IsPublic, Valid: true}
	}
	if def.AggregateEnabled != nil {
		params.AggregateEnabled = pgtype.Bool{Bool: *def.AggregateEnabled, Valid: true}
	}
	if def.RetentionDays != nil {
		params.RetentionDays = pgtype.Int4{Int32: *def.RetentionDays, Valid: true}
	}
	row, err := s.q.UpdateEventDefinition(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update event definition: %w", err)
	}

	_ = s.Refresh(ctx)
	out := mapRowToDefinition(row)
	return &out, nil
}

// Delete removes an event definition by ID.
func (s *CachedStore) Delete(ctx context.Context, id int64) error {
	if err := s.q.DeleteEventDefinition(ctx, id); err != nil {
		return fmt.Errorf("delete event definition: %w", err)
	}
	_ = s.Refresh(ctx)
	return nil
}

// Refresh reloads all event definitions from the database into memory.
func (s *CachedStore) Refresh(ctx context.Context) error {
	rows, err := s.q.ListEventDefinitions(ctx, db.ListEventDefinitionsParams{})
	if err != nil {
		return err
	}

	byKey := make(map[string]domain.EventDefinition, len(rows))
	byID := make(map[int64]domain.EventDefinition, len(rows))

	for _, r := range rows {
		def := mapRowToDefinition(r)
		byKey[def.Key] = def
		byID[def.ID] = def
	}

	s.mu.Lock()
	s.byKey = byKey
	s.byID = byID
	s.expAt = time.Now().Add(s.ttl)
	s.mu.Unlock()

	return nil
}

func mapRowToDefinition(r db.EventDefinition) domain.EventDefinition {
	desc := ""
	if r.Description.Valid {
		desc = r.Description.String
	}
	reqProps := r.RequiredProperties
	if reqProps == nil {
		reqProps = []string{}
	}
	return domain.EventDefinition{
		ID:                 r.ID,
		Key:                r.Key,
		Name:               r.Name,
		Description:        desc,
		Category:           r.Category,
		SchemaVersion:      int(r.SchemaVersion),
		Enabled:            r.Enabled,
		IsPublic:           r.IsPublic,
		AggregateEnabled:   r.AggregateEnabled,
		RetentionDays:      int(r.RetentionDays),
		RequiredProperties: reqProps,
		CreatedAt:          r.CreatedAt.Time,
		UpdatedAt:          r.UpdatedAt.Time,
	}
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
