package namelease

import (
	"strings"
	"time"
)

func applyConflict(current *Record, op Op) (Record, error) {
	if current == nil {
		return Record{}, Error{Action: opConflict, Reason: "name is unclaimed"}
	}
	if current.Name != op.Name {
		return Record{}, Error{Action: opConflict, Reason: "record name mismatch"}
	}
	if current.State != stateActive && current.State != stateGrace {
		return Record{}, Error{Action: opConflict, Reason: "conflict can only be created for active or grace names (R-039)"}
	}
	if strings.TrimSpace(op.ConflictContext) == "" {
		return Record{}, Error{Action: opConflict, Reason: "conflict context is required"}
	}
	result := *current
	result.Revision++
	result.State = stateConflict
	result.ConflictIdentifier = strings.TrimSpace(op.ConflictContext)
	result.RecoveryExpiresAt = 0
	result.LeaseExpiresAt = 0
	result.GraceExpiresAt = 0
	return result, nil
}

func applyResolveConflict(current *Record, now int64, op Op, leaseDuration, graceDuration time.Duration) (Record, error) {
	if current == nil {
		return Record{}, Error{Action: opResolveConflict, Reason: "name is unclaimed"}
	}
	if current.Name != op.Name {
		return Record{}, Error{Action: opResolveConflict, Reason: "record name mismatch"}
	}
	if current.State != stateConflict {
		return Record{}, Error{Action: opResolveConflict, Reason: "name is not in conflict"}
	}
	if op.Authority == "" {
		return Record{}, Error{Action: opResolveConflict, Reason: "authority is required"}
	}
	if op.Generation != 0 && op.Generation != current.Generation {
		return Record{}, Error{Action: opResolveConflict, Reason: "generation is stale"}
	}
	target := strings.TrimSpace(op.Target)
	if target == "" {
		target = current.Target
	}
	if leaseDuration <= 0 {
		return Record{}, Error{Action: opResolveConflict, Reason: "lease duration must be positive"}
	}
	result := *current
	result.Revision++
	result.State = stateActive
	result.Authority = op.Authority
	result.Target = target
	result.Continuity++
	result.LeaseExpiresAt = now + int64(leaseDuration.Seconds())
	result.GraceExpiresAt = now + int64(leaseDuration.Seconds()) + int64(graceDuration.Seconds())
	result.RecoveryExpiresAt = 0
	result.ConflictIdentifier = ""
	return result, nil
}
