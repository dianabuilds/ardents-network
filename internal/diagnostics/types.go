package diagnostics

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	"ardents/internal/diagnostics/reason"
	"ardents/internal/diagnostics/recorder"
)

const (
	HealthReady    = health.Ready
	HealthDegraded = health.Degraded
	HealthFailed   = health.Failed
)

const persistenceFailureCode = recorder.PersistenceFailureCode

const (
	OperationRecovering = operation.Recovering
	OperationCompleted  = operation.Completed
	OperationFailed     = operation.Failed
	OperationAbandoned  = operation.Abandoned
)

type Recorder = recorder.Recorder
type Reason = reason.Reason
type SubsystemStatus = health.SubsystemStatus
type HealthSummary = health.Summary
type EventRecord = event.Record
type OperationRecord = operation.Record
type Snapshot = recorder.Snapshot
type CorruptLedgerError = recorder.CorruptLedgerError

func IsCorruptLedger(err error) (*CorruptLedgerError, bool) {
	return recorder.IsCorruptLedger(err)
}
