package diagnostics

import (
	diagapi "ardents/internal/diagnostics/api"
	"ardents/internal/diagnostics/projection"
)

func DiagnosticsSnapshot(in Snapshot) diagapi.DiagSnapshot {
	return projection.DiagnosticsSnapshot(in.Health, in.RecentEvents, in.PendingOperations)
}

func OperationSnapshots(in []OperationRecord) []diagapi.OperationSnapshot {
	return projection.OperationSnapshots(in)
}

func HealthSnapshot(in HealthSummary) diagapi.HealthSnapshot {
	return projection.HealthSnapshot(in)
}

func EventEnvelopes(in []EventRecord) []diagapi.EventEnvelope {
	return projection.EventEnvelopes(in)
}

func ReasonSnapshot(in *Reason) diagapi.ReasonSnapshot {
	return projection.ReasonSnapshot(in)
}

func ReasonSnapshotPtr(in *Reason) *diagapi.ReasonSnapshot {
	return projection.ReasonSnapshotPtr(in)
}

func FailureExplanation(scope, resourceID, state string, reason diagapi.ReasonSnapshot) diagapi.FailureExplanationSnapshot {
	return projection.FailureExplanation(scope, resourceID, state, reason)
}
