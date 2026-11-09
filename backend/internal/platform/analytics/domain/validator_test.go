package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
)

type mockRegistry struct {
	defs map[string]*domain.EventDefinition
}

func (m *mockRegistry) Get(ctx context.Context, key string) (*domain.EventDefinition, error) {
	if def, ok := m.defs[key]; ok {
		return def, nil
	}
	return nil, domain.ErrDefinitionNotFound
}

func TestValidator_Validate(t *testing.T) {
	reg := &mockRegistry{
		defs: map[string]*domain.EventDefinition{
			"card.view": {
				Key:                "card.view",
				Enabled:            true,
				IsPublic:           true,
				SchemaVersion:      1,
				RequiredProperties: []string{},
			},
			"internal.system.job": {
				Key:                "internal.system.job",
				Enabled:            true,
				IsPublic:           false,
				SchemaVersion:      1,
				RequiredProperties: []string{},
			},
			"payment.charge": {
				Key:                "payment.charge",
				Enabled:            true,
				IsPublic:           true,
				SchemaVersion:      2,
				RequiredProperties: []string{"amount", "currency"},
			},
			"disabled.event": {
				Key:      "disabled.event",
				Enabled:  false,
				IsPublic: true,
			},
		},
	}

	validator := domain.NewValidator(reg)
	ctx := context.Background()

	t.Run("valid public event", func(t *testing.T) {
		ev := &domain.Event{
			Type: "card.view",
			Subject: domain.EntityReference{
				Type: "card",
				ID:   "c_123",
			},
			OccurredAt: time.Now().UTC(),
		}
		if err := validator.Validate(ctx, ev, true); err != nil {
			t.Fatalf("expected valid, got: %v", err)
		}
	})

	t.Run("missing subject type or id", func(t *testing.T) {
		ev := &domain.Event{
			Type:       "card.view",
			Subject:    domain.EntityReference{Type: "card", ID: ""},
			OccurredAt: time.Now().UTC(),
		}
		if err := validator.Validate(ctx, ev, true); err == nil {
			t.Fatal("expected error for empty subject id, got nil")
		}
	})

	t.Run("public ingress rejecting internal-only event", func(t *testing.T) {
		ev := &domain.Event{
			Type: "internal.system.job",
			Subject: domain.EntityReference{
				Type: "system",
				ID:   "sys_1",
			},
			OccurredAt: time.Now().UTC(),
		}
		if err := validator.Validate(ctx, ev, true); err == nil {
			t.Fatal("expected error for internal event on public ingress, got nil")
		}
		// But valid for internal ingress:
		if err := validator.Validate(ctx, ev, false); err != nil {
			t.Fatalf("expected valid for internal ingress, got: %v", err)
		}
	})

	t.Run("disabled event rejection", func(t *testing.T) {
		ev := &domain.Event{
			Type: "disabled.event",
			Subject: domain.EntityReference{
				Type: "test",
				ID:   "t_1",
			},
			OccurredAt: time.Now().UTC(),
		}
		if err := validator.Validate(ctx, ev, true); err == nil {
			t.Fatal("expected error for disabled event, got nil")
		}
	})

	t.Run("missing required properties", func(t *testing.T) {
		ev := &domain.Event{
			Type: "payment.charge",
			Subject: domain.EntityReference{
				Type: "payment",
				ID:   "pay_1",
			},
			Properties: map[string]any{"amount": 100}, // missing "currency"
			OccurredAt: time.Now().UTC(),
		}
		if err := validator.Validate(ctx, ev, true); err == nil {
			t.Fatal("expected error for missing required property, got nil")
		}

		ev.Properties["currency"] = "USD"
		if err := validator.Validate(ctx, ev, true); err != nil {
			t.Fatalf("expected valid after adding currency, got: %v", err)
		}
	})

	t.Run("future timestamp rejection", func(t *testing.T) {
		ev := &domain.Event{
			Type: "card.view",
			Subject: domain.EntityReference{
				Type: "card",
				ID:   "c_123",
			},
			OccurredAt: time.Now().UTC().Add(2 * time.Hour),
		}
		if err := validator.Validate(ctx, ev, true); err == nil {
			t.Fatal("expected error for future timestamp, got nil")
		}
	})
}
