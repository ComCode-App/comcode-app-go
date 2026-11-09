package domain

import "encoding/json"

// EventDefinitionPatch preserves omitted fields during administrative updates.
// Key is immutable and is deliberately not part of the patch.
type EventDefinitionPatch struct {
	ID                 int64               `json:"-"`
	Name               *string             `json:"name"`
	Description        OptionalDescription `json:"description"`
	Category           *string             `json:"category"`
	SchemaVersion      *int32              `json:"schema_version"`
	Enabled            *bool               `json:"enabled"`
	IsPublic           *bool               `json:"is_public"`
	AggregateEnabled   *bool               `json:"aggregate_enabled"`
	RetentionDays      *int32              `json:"retention_days"`
	RequiredProperties []string            `json:"required_properties"`
}

// OptionalDescription distinguishes omission from an explicit empty string or null.
type OptionalDescription struct {
	Set   bool
	Value string
}

// UnmarshalJSON treats null as an explicitly cleared description.
func (d *OptionalDescription) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	d.Set, d.Value = true, ""
	if value != nil {
		d.Value = *value
	}
	return nil
}
