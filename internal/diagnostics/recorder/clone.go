package recorder

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	"ardents/internal/diagnostics/reason"
)

func CloneReason(in *reason.Reason) *reason.Reason {
	return reason.Clone(in)
}

func CloneHealthSummary(in health.Summary) health.Summary {
	return health.CloneSummary(in)
}

func CloneEventRecords(in []event.Record) []event.Record {
	return event.Clone(in)
}

func CloneOperationRecords(in []operation.Record) []operation.Record {
	return operation.Clone(in)
}
