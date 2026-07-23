package diagnostics

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
)

const (
	HealthReady    = health.Ready
	HealthDegraded = health.Degraded
	HealthFailed   = health.Failed
)

const persistenceFailureCode = PersistenceFailureCode

const OperationRecovering = operation.Recovering

type Reason = health.Reason
type SubsystemStatus = health.SubsystemStatus
type HealthSummary = health.Summary
type EventRecord = event.Record
type OperationRecord = operation.Record

type EventWriter interface {
	RecordEventCommand(RecordEventCommand) EventEnvelope
}

// DurableEventWriter reports whether the event reached the persisted ledger.
// Security outboxes use it to acknowledge delivery without losing an event
// when operations.json cannot be replaced atomically.
type DurableEventWriter interface {
	RecordEventCommandDurable(RecordEventCommand) (EventEnvelope, error)
}

type Writer interface {
	EventWriter
	BeginOperationCommand(BeginOperationCommand) OperationSnapshot
	CompleteOperationCommand(TransitionOperationCommand)
	FailOperationCommand(TransitionOperationCommand)
	RecoverOperationCommand(TransitionOperationCommand)
	AbandonOperationCommand(TransitionOperationCommand)
	SetPrimaryHealth(SetPrimaryHealthCommand)
	ClearPrimary()
	SetSubsystemHealth(SetSubsystemHealthCommand)
	ClearSubsystem(string)
}

type Service interface {
	DiagnosticsSnapshot() DiagSnapshot
	PendingOperations() []OperationSnapshot
	GetHealthSummary() HealthSnapshot
	ExplainFailure(string, string) FailureExplanationSnapshot
	ListRecentEvents(int, string) ([]EventEnvelope, string)
}
