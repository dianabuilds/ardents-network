package api

import "time"

type ReasonSnapshot struct {
	Code                   string `json:"code,omitempty"`
	Domain                 string `json:"domain,omitempty"`
	Summary                string `json:"summary,omitempty"`
	Detail                 string `json:"detail,omitempty"`
	Impact                 string `json:"impact,omitempty"`
	Recovery               string `json:"recovery,omitempty"`
	OperatorActionRequired bool   `json:"operator_action_required,omitempty"`
	Resource               string `json:"resource,omitempty"`
}

type SubsystemHealthSnapshot struct {
	Domain    string          `json:"domain,omitempty"`
	State     string          `json:"state,omitempty"`
	UpdatedAt time.Time       `json:"updated_at,omitempty"`
	Reason    *ReasonSnapshot `json:"reason,omitempty"`
}

type HealthSnapshot struct {
	State                  string                    `json:"state,omitempty"`
	UpdatedAt              time.Time                 `json:"updated_at,omitempty"`
	OperatorActionRequired bool                      `json:"operator_action_required,omitempty"`
	PrimaryReason          *ReasonSnapshot           `json:"primary_reason,omitempty"`
	Subsystems             []SubsystemHealthSnapshot `json:"subsystems,omitempty"`
}

type OperationSnapshot struct {
	ID             string     `json:"id,omitempty"`
	Kind           string     `json:"kind,omitempty"`
	State          string     `json:"state,omitempty"`
	Domain         string     `json:"domain,omitempty"`
	Resource       string     `json:"resource,omitempty"`
	Reason         string     `json:"reason,omitempty"`
	Recoverable    bool       `json:"recoverable,omitempty"`
	RecoveryAction string     `json:"recovery_action,omitempty"`
	StartedAt      time.Time  `json:"started_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

type EventEnvelope struct {
	Seq      int64          `json:"seq,omitempty"`
	Time     time.Time      `json:"time,omitempty"`
	Domain   string         `json:"domain,omitempty"`
	Type     string         `json:"type,omitempty"`
	Resource string         `json:"resource,omitempty"`
	Payload  map[string]any `json:"payload,omitempty"`
}

type DiagSnapshot struct {
	Health            HealthSnapshot      `json:"health"`
	RecentEvents      []EventEnvelope     `json:"recent_events,omitempty"`
	PendingOperations []OperationSnapshot `json:"pending_operations,omitempty"`
}

type FailureExplanationSnapshot struct {
	Scope      string          `json:"scope,omitempty"`
	ResourceID string          `json:"resource_id,omitempty"`
	State      string          `json:"state,omitempty"`
	Reason     *ReasonSnapshot `json:"reason,omitempty"`
	Impact     string          `json:"impact,omitempty"`
	Recovery   string          `json:"recovery,omitempty"`
	NextSteps  []string        `json:"next_steps,omitempty"`
}
