package usecase

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
)

// Field error codes returned in ValidationError details for plan limits.
const (
	LimitCodeRequired  = "required"
	LimitCodeUnknown   = "unknown_feature"
	LimitCodeDuplicate = "duplicate"
	LimitCodeType      = "type_mismatch"
	LimitCodeRange     = "out_of_range"
)

// FieldError is one field-level problem; the handler maps it onto response.Detail.
type FieldError struct {
	Field   string
	Message string
	Code    string
}

// ValidationError carries field-level details. It matches ErrInvalidRequest with errors.Is.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return ErrInvalidRequest.Error()
	}
	return fmt.Sprintf("%s: %s", ErrInvalidRequest, e.Fields[0].Message)
}

func (e *ValidationError) Unwrap() error { return ErrInvalidRequest }

// ValidateLimits checks a full limit set against the feature definitions (key → value_type).
// Rules: the key must be a defined feature and appear once; exactly one value is set and it
// matches the feature's value_type; an int is entitlement.Unlimited (-1) or 0..MaxInt32.
// Every defined feature must be listed: a missing one is a "required" error on its field.
// Errors on a known key use field "limits.<feature_key>"; otherwise "limits[<index>].feature_key".
func ValidateLimits(valueTypes map[string]string, limits []PlanLimitInput) []FieldError {
	var out []FieldError
	seen := make(map[string]bool, len(limits))
	for i, lim := range limits {
		key := strings.TrimSpace(lim.FeatureKey)
		if key == "" {
			out = append(out, FieldError{
				Field: fmt.Sprintf("limits[%d].feature_key", i), Code: LimitCodeRequired,
				Message: "feature_key is required",
			})
			continue
		}
		valueType, ok := valueTypes[key]
		if !ok {
			out = append(out, FieldError{
				Field: fmt.Sprintf("limits[%d].feature_key", i), Code: LimitCodeUnknown,
				Message: fmt.Sprintf("unknown feature %q", key),
			})
			continue
		}
		field := "limits." + key
		if seen[key] {
			out = append(out, FieldError{Field: field, Code: LimitCodeDuplicate, Message: "feature is listed more than once"})
			continue
		}
		seen[key] = true
		if lim.IntValue != nil && lim.BoolValue != nil {
			out = append(out, FieldError{Field: field, Code: LimitCodeType, Message: "set only one of int_value or bool_value"})
			continue
		}
		switch valueType {
		case "int":
			switch {
			case lim.IntValue == nil && lim.BoolValue != nil:
				out = append(out, FieldError{Field: field, Code: LimitCodeType, Message: "feature takes an integer (int_value)"})
			case lim.IntValue == nil:
				out = append(out, FieldError{Field: field, Code: LimitCodeRequired, Message: "int_value is required"})
			case *lim.IntValue != int64(entitlement.Unlimited) && (*lim.IntValue < 0 || *lim.IntValue > math.MaxInt32):
				out = append(out, FieldError{Field: field, Code: LimitCodeRange, Message: "must be -1 (unlimited) or a whole number from 0"})
			}
		case "bool":
			switch {
			case lim.BoolValue == nil && lim.IntValue != nil:
				out = append(out, FieldError{Field: field, Code: LimitCodeType, Message: "feature takes a boolean (bool_value)"})
			case lim.BoolValue == nil:
				out = append(out, FieldError{Field: field, Code: LimitCodeRequired, Message: "bool_value is required"})
			}
		default:
			out = append(out, FieldError{Field: field, Code: LimitCodeType, Message: fmt.Sprintf("feature has unsupported value_type %q", valueType)})
		}
	}
	missing := make([]string, 0, len(valueTypes))
	for key := range valueTypes {
		if !seen[key] {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	for _, key := range missing {
		out = append(out, FieldError{Field: "limits." + key, Code: LimitCodeRequired, Message: "a value is required for every feature"})
	}
	return out
}

// validateLimits loads the feature definitions and runs ValidateLimits.
func (s *Service) validateLimits(ctx context.Context, limits []PlanLimitInput) error {
	rows, err := s.q.ListFeatureDefinitions(ctx)
	if err != nil {
		return err
	}
	valueTypes := make(map[string]string, len(rows))
	for _, row := range rows {
		valueTypes[row.Key] = string(row.ValueType)
	}
	if fields := ValidateLimits(valueTypes, limits); len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}
