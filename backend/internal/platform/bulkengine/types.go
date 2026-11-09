package bulkengine

import (
	"context"
	"encoding/json"
)

// BulkActionDef describes one bulk action exposed via resource meta.
type BulkActionDef struct {
	ID          string `json:"id"`
	LabelKey    string `json:"label_key"`
	Permission  string `json:"permission"`
	Destructive bool   `json:"destructive"`
	Reversible  bool   `json:"reversible"`
	ConfirmKey  string `json:"confirm_key,omitempty"`
}

// BulkTarget selects rows by explicit ids or list query filters.
type BulkTarget struct {
	Scope string            `json:"scope"` // ids | query
	IDs   []string          `json:"ids,omitempty"`
	Query map[string]string `json:"query,omitempty"`
}

// BulkItemResult is the outcome of applying one bulk item.
type BulkItemResult struct {
	EntityUUID string
	EntityType string
	OK         bool
	Error      string
	Op         string // update | delete
	Previous   map[string]any
}

// BulkSummary aggregates job results.
type BulkSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	// Errors lists the first failed items (at most MaxSummaryErrors).
	Errors []BulkItemError `json:"errors,omitempty"`
}

// BulkItemError is one failed item of a job.
type BulkItemError struct {
	EntityUUID string `json:"entity_uuid"`
	Error      string `json:"error"`
}

// MaxSummaryErrors caps BulkSummary.Errors so a large failing job keeps a small result row.
const MaxSummaryErrors = 50

// AddError counts a failed item and keeps its message while under MaxSummaryErrors.
func (s *BulkSummary) AddError(entityUUID, msg string) {
	s.Failed++
	if len(s.Errors) < MaxSummaryErrors {
		s.Errors = append(s.Errors, BulkItemError{EntityUUID: entityUUID, Error: msg})
	}
}

// BulkAdapter connects bulkengine to a list resource.
type BulkAdapter interface {
	Resource() string
	BulkActions() []BulkActionDef
	ResolveTargets(ctx context.Context, action string, target BulkTarget) ([]string, error)
	ApplyItem(ctx context.Context, action, entityUUID string) (BulkItemResult, error)
	RevertItem(ctx context.Context, action, entityUUID string, previous map[string]any) error
}

// AccountScoped is implemented by adapters of app resources: their jobs run inside one account
// (jobscope.Scope on the context of every adapter call) and take action parameters. The engine
// persists every job of such an adapter, also the inline ones, so each run can be rolled back,
// and refuses to run it without an account.
type AccountScoped interface {
	BulkAdapter
	// ValidateParams rejects bad parameters before any target is touched. The context carries
	// the job scope.
	ValidateParams(ctx context.Context, action string, params json.RawMessage) error
}
