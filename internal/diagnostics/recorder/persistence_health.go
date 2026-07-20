package recorder

import (
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/reason"
	"time"
)

func (r *Recorder) markPersistenceFailureLocked(err error) {
	now := time.Now().UTC()
	item := &reason.Reason{
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
		r.primary = reason.Clone(item)
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
