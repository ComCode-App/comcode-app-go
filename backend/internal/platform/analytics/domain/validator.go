package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var eventTypeRegex = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)

const (
	MaxPropertyKeyLength = 64
	MaxPropertiesCount   = 100
	MaxPropertyDepth     = 5
	MaxFutureSkew        = 5 * time.Minute
	MaxPastSkew          = 30 * 24 * time.Hour
)

// EventValidator verifies events against domain boundaries and the definition registry.
type EventValidator interface {
	Validate(ctx context.Context, event *Event, isPublicIngress bool) error
}

// Registry provides read access to event definitions.
type Registry interface {
	Get(ctx context.Context, key string) (*EventDefinition, error)
}

// DefaultValidator validates event structure, safety limits, and registry schemas.
type DefaultValidator struct {
	registry Registry
}

// NewValidator creates a DefaultValidator.
func NewValidator(registry Registry) *DefaultValidator {
	return &DefaultValidator{registry: registry}
}

// Validate checks an event envelope for structural, business, and security constraints.
func (v *DefaultValidator) Validate(ctx context.Context, event *Event, isPublicIngress bool) error {
	if event == nil {
		return fmt.Errorf("%w: event is nil", ErrInvalidEventType)
	}

	event.Type = strings.TrimSpace(event.Type)
	if event.Type == "" || len(event.Type) > 100 || !eventTypeRegex.MatchString(event.Type) {
		return fmt.Errorf("%w: %q", ErrInvalidEventType, event.Type)
	}

	event.Subject.Type = strings.TrimSpace(event.Subject.Type)
	event.Subject.ID = strings.TrimSpace(event.Subject.ID)
	if event.Subject.Type == "" || event.Subject.ID == "" {
		return ErrInvalidSubject
	}
	if len(event.Subject.Type) > 64 || len(event.Subject.ID) > 128 {
		return fmt.Errorf("%w: subject type/id exceeds length limit", ErrInvalidSubject)
	}

	if event.Actor != nil {
		event.Actor.Type = strings.TrimSpace(event.Actor.Type)
		event.Actor.ID = strings.TrimSpace(event.Actor.ID)
		if event.Actor.Type == "" || event.Actor.ID == "" {
			event.Actor = nil
		} else if len(event.Actor.Type) > 64 || len(event.Actor.ID) > 128 {
			return fmt.Errorf("%w: actor type/id exceeds length limit", ErrInvalidSubject)
		}
	}

	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	} else {
		now := time.Now().UTC()
		if event.OccurredAt.After(now.Add(MaxFutureSkew)) {
			return ErrFutureTimestamp
		}
		if event.OccurredAt.Before(now.Add(-MaxPastSkew)) {
			return fmt.Errorf("occurred_at timestamp is too old (max %v)", MaxPastSkew)
		}
	}

	if len(event.Properties) > MaxPropertiesCount {
		return fmt.Errorf("%w: max %d properties allowed", ErrPayloadTooLarge, MaxPropertiesCount)
	}

	if err := validatePropertiesDepth(event.Properties, 1); err != nil {
		return err
	}

	if v.registry != nil {
		def, err := v.registry.Get(ctx, event.Type)
		if err != nil {
			if isPublicIngress {
				return fmt.Errorf("%w: %s", ErrDefinitionNotFound, event.Type)
			}
		} else if def != nil {
			if !def.Enabled {
				return fmt.Errorf("%w: %s", ErrEventDisabled, event.Type)
			}
			if isPublicIngress && !def.IsPublic {
				return fmt.Errorf("%w: %s", ErrEventInternalOnly, event.Type)
			}
			for _, reqProp := range def.RequiredProperties {
				reqProp = strings.TrimSpace(reqProp)
				if reqProp == "" {
					continue
				}
				if val, ok := event.Properties[reqProp]; !ok || val == nil || fmt.Sprint(val) == "" {
					return fmt.Errorf("%w: %s", ErrMissingProperty, reqProp)
				}
			}
			if event.SchemaVersion <= 0 {
				event.SchemaVersion = def.SchemaVersion
			}
		}
	}

	if event.SchemaVersion <= 0 {
		event.SchemaVersion = 1
	}

	return nil
}

func validatePropertiesDepth(m map[string]any, depth int) error {
	if depth > MaxPropertyDepth {
		return ErrPayloadTooDeep
	}
	for k, v := range m {
		if len(k) > MaxPropertyKeyLength {
			return fmt.Errorf("%w: property key %q exceeds %d chars", ErrPayloadTooLarge, k, MaxPropertyKeyLength)
		}
		if childMap, ok := v.(map[string]any); ok {
			if err := validatePropertiesDepth(childMap, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
