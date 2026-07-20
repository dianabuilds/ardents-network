package config

import (
	"context"
	"time"
)

type Outcome string

const (
	OutcomeUnchanged         Outcome = "unchanged"
	OutcomeApplied           Outcome = "applied"
	OutcomeRestartRequired   Outcome = "restart_required"
	OutcomeRejectedInvalid   Outcome = "rejected_invalid"
	OutcomeRejectedImmutable Outcome = "rejected_immutable"
	OutcomeRolledBack        Outcome = "rolled_back"
)

type Applier interface {
	Prepare(context.Context, Document, Document) error
	Apply(context.Context, Document, Document) error
	Rollback(context.Context, Document) error
}

type Validator func(Document) error
type Resolver func(Document) (Document, error)

type Service interface {
	GetEffectiveConfig() EffectiveSnapshot
	ReloadConfig(context.Context) ReloadResult
}

type ReloadResult struct {
	Outcome             Outcome  `json:"outcome"`
	ActiveGeneration    uint64   `json:"active_generation"`
	CandidateGeneration uint64   `json:"candidate_generation"`
	RestartRequired     []string `json:"restart_required,omitempty"`
	Immutable           []string `json:"immutable,omitempty"`
	Reason              string   `json:"reason,omitempty"`
}

type EffectiveSnapshot struct {
	APIVersion          string         `json:"api_version"`
	ActiveGeneration    uint64         `json:"active_generation"`
	CandidateGeneration uint64         `json:"candidate_generation"`
	Fingerprint         string         `json:"fingerprint"`
	LoadedAt            time.Time      `json:"loaded_at"`
	Effective           map[string]any `json:"effective"`
	PendingRestart      []string       `json:"pending_restart,omitempty"`
	LastReload          ReloadResult   `json:"last_reload"`
}
