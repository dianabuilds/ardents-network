package diagnostics

import (
	"ardents/internal/diagnostics/health"
	core "ardents/internal/diagnostics/recorder"
)

func CloneReason(in *Reason) *Reason {
	return core.CloneReason(in)
}

func CloneSubsystemStatus(in SubsystemStatus) SubsystemStatus {
	return health.CloneSubsystem(in)
}

func CloneHealthSummary(in HealthSummary) HealthSummary {
	return core.CloneHealthSummary(in)
}

func CloneEventRecords(in []EventRecord) []EventRecord {
	return core.CloneEventRecords(in)
}

func CloneOperationRecords(in []OperationRecord) []OperationRecord {
	return core.CloneOperationRecords(in)
}
