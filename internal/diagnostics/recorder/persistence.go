package recorder

import (
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	"ardents/internal/diagnostics/reason"
	"time"
)

func (r *Recorder) saveLocked() error {
	return Save(r.path, Ledger{
		Seq:            r.seq,
		Health:         health.CloneSummary(r.health),
		RetainedHealth: health.CloneSummary(r.retained),
		Events:         CloneEventRecords(r.events),
		Operations:     operation.Compact(operation.Records(r.operations), r.maxClosedOps),
	})
}

func (r *Recorder) restoreHealthLocked(in health.Summary) {
	primary, primarySet, primaryState, subsystems := health.Restore(in)
	r.primary = reason.Clone(primary)
	r.primarySet = primarySet
	r.primaryState = primaryState
	r.subsystems = cloneSubsystems(subsystems)
	r.refreshHealthLocked(in.UpdatedAt)
}

func (r *Recorder) persistLocked() {
	if err := r.saveLocked(); err != nil {
		r.markPersistenceFailureLocked(err)
		return
	}
	if !r.clearPersistenceFailureLocked(time.Now().UTC()) {
		return
	}
	if err := r.saveLocked(); err != nil {
		r.markPersistenceFailureLocked(err)
	}
}
