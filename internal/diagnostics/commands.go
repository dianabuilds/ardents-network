// Package diagnostics owns operational evidence aggregation, queries, and redaction.
// It does not own recomputing product-owned truth.
package diagnostics

type RecordEventCommand struct {
	Domain     string         `json:"domain,omitempty"`
	Type       string         `json:"type,omitempty"`
	Resource   string         `json:"resource,omitempty"`
	Message    string         `json:"message,omitempty"`
	ReasonCode string         `json:"reason_code,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
}

type BeginOperationCommand struct {
	Kind           string `json:"kind,omitempty"`
	Domain         string `json:"domain,omitempty"`
	Resource       string `json:"resource,omitempty"`
	Recoverable    bool   `json:"recoverable,omitempty"`
	RecoveryAction string `json:"recovery_action,omitempty"`
}

type TransitionOperationCommand struct {
	ID     string `json:"id,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type SetPrimaryHealthCommand struct {
	State  string          `json:"state,omitempty"`
	Reason *ReasonSnapshot `json:"reason,omitempty"`
}

type SetSubsystemHealthCommand struct {
	Domain string          `json:"domain,omitempty"`
	State  string          `json:"state,omitempty"`
	Reason *ReasonSnapshot `json:"reason,omitempty"`
}

type SnapshotQuery struct{}
type PendingOperationsQuery struct{}
type GetHealthSummaryQuery struct{}

type ExplainFailureQuery struct {
	Scope      string `json:"scope,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
}

type ListRecentEventsQuery struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}
