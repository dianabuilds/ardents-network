package diagnostics

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	"time"
)

func (r *Recorder) SetPrimary(state string, item *Reason) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if item == nil {
		r.primary = nil
		r.primarySet = false
		r.primaryState = ""
		r.refreshHealthLocked(time.Now().UTC())
		r.persistLocked()
		return
	}
	r.primary = health.Clone(item)
	r.primarySet = state == health.Degraded || state == health.Failed
	r.primaryState = state
	r.refreshHealthLocked(time.Now().UTC())
	r.persistLocked()
}

func (r *Recorder) ClearPrimary() {
	r.SetPrimary("", nil)
}

func (r *Recorder) RetainCurrentHealth() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.health.PrimaryReason != nil || len(r.health.Subsystems) != 0 {
		r.retained = health.CloneSummary(r.health)
	}
	r.primary = nil
	r.primarySet = false
	r.primaryState = ""
	r.subsystems = map[string]health.SubsystemStatus{}
	r.refreshHealthLocked(time.Now().UTC())
	r.persistLocked()
}

func (r *Recorder) SetSubsystem(domain, state string, item *Reason) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	if state == "" || state == health.Ready {
		delete(r.subsystems, domain)
		r.refreshHealthLocked(now)
		r.persistLocked()
		return
	}
	r.subsystems[domain] = health.SubsystemStatus{
		Domain:    domain,
		State:     state,
		Reason:    health.Clone(item),
		UpdatedAt: now,
	}
	r.refreshHealthLocked(now)
	r.persistLocked()
}

func (r *Recorder) ClearSubsystem(domain string) {
	r.SetSubsystem(domain, health.Ready, nil)
}

func (r *Recorder) Health() health.Summary {
	r.mu.Lock()
	defer r.mu.Unlock()
	return health.CloneSummary(r.health)
}

func (r *Recorder) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Snapshot{
		Health:            health.CloneSummary(r.health),
		RecentEvents:      event.Clone(r.events),
		PendingOperations: operation.Clone(r.pendingOperationsLocked()),
	}
}

func (r *Recorder) refreshHealthLocked(now time.Time) {
	r.health = health.Compose(now, r.primaryState, r.primarySet, health.Clone(r.primary), cloneSubsystems(r.subsystems))
}

func cloneSubsystems(in map[string]health.SubsystemStatus) map[string]health.SubsystemStatus {
	out := make(map[string]health.SubsystemStatus, len(in))
	for key, item := range in {
		out[key] = health.CloneSubsystem(item)
	}
	return out
}

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
	r.primary = health.Clone(primary)
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

func (r *Recorder) markPersistenceFailureLocked(err error) {
	now := time.Now().UTC()
	item := &Reason{
		Code:                   PersistenceFailureCode,
		Domain:                 "diagnostics",
		Summary:                "diagnostics persistence failed",
		Detail:                 err.Error(),
		Impact:                 "diagnostics updates will not survive restart",
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               "operations",
	}
	r.subsystems["diagnostics"] = health.SubsystemStatus{
		Domain:    "diagnostics",
		State:     health.Degraded,
		Reason:    item,
		UpdatedAt: now,
	}
	if !r.primarySet || (r.primary != nil && r.primary.Code == PersistenceFailureCode) {
		r.primary = health.Clone(item)
		r.primarySet = true
		r.primaryState = health.Degraded
	}
	r.refreshHealthLocked(now)
}

func (r *Recorder) clearPersistenceFailureLocked(now time.Time) bool {
	changed := false
	if item, ok := r.subsystems["diagnostics"]; ok && item.Reason != nil && item.Reason.Code == PersistenceFailureCode {
		delete(r.subsystems, "diagnostics")
		changed = true
	}
	if r.primary != nil && r.primary.Code == PersistenceFailureCode {
		r.primary = nil
		r.primarySet = false
		r.primaryState = ""
		changed = true
	}
	if changed {
		r.refreshHealthLocked(now)
	}
	return changed
}
