package recorder

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	"ardents/internal/diagnostics/reason"
	"time"
)

func (r *Recorder) SetPrimary(state string, item *reason.Reason) {
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
	r.primary = reason.Clone(item)
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

func (r *Recorder) SetSubsystem(domain, state string, item *reason.Reason) {
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
		Reason:    reason.Clone(item),
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
	r.health = health.Compose(now, r.primaryState, r.primarySet, reason.Clone(r.primary), cloneSubsystems(r.subsystems))
}

func cloneSubsystems(in map[string]health.SubsystemStatus) map[string]health.SubsystemStatus {
	out := make(map[string]health.SubsystemStatus, len(in))
	for key, item := range in {
		out[key] = health.CloneSubsystem(item)
	}
	return out
}
