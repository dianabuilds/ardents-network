package namelease

import (
	"strings"
	"time"
)

func applyStartRecovery(current *Record, now int64, op Op, defaultDelay time.Duration) (Record, error) {
	if current == nil {
		return Record{}, Error{Action: opStartRecovery, Reason: "name is unclaimed"}
	}
	if current.Name != op.Name {
		return Record{}, Error{Action: opStartRecovery, Reason: "record name mismatch"}
	}
	if op.Authority != current.Authority {
		return Record{}, Error{Action: opStartRecovery, Reason: "authority mismatch"}
	}
	if current.State != stateActive && current.State != stateGrace {
		return Record{}, Error{Action: opStartRecovery, Reason: "recovery is only valid in active or grace"}
	}
	delay := op.RecoveryDelay
	if delay <= 0 {
		delay = defaultDelay
	}
	result := *current
	result.Revision++
	result.State = stateRecovery
	result.RecoveryExpiresAt = now + int64(delay.Seconds())
	return result, nil
}

func applyInstallSuccessor(current *Record, now int64, op Op, leaseDuration, graceDuration time.Duration) (Record, error) {
	if current == nil {
		return Record{}, Error{Action: opInstallSuccessor, Reason: "name is unclaimed"}
	}
	if current.Name != op.Name {
		return Record{}, Error{Action: opInstallSuccessor, Reason: "record name mismatch"}
	}
	if op.NewAuthority == "" {
		return Record{}, Error{Action: opInstallSuccessor, Reason: "successor authority is required"}
	}
	if current.State != stateRecovery {
		return Record{}, Error{Action: opInstallSuccessor, Reason: "recovery is not pending"}
	}
	if op.Generation != 0 && op.Generation != current.Generation {
		return Record{}, Error{Action: opInstallSuccessor, Reason: "generation is stale"}
	}
	if current.RecoveryExpiresAt == 0 || now > current.RecoveryExpiresAt {
		return Record{}, Error{Action: opInstallSuccessor, Reason: "recovery window has elapsed"}
	}
	target := strings.TrimSpace(op.Target)
	if target == "" {
		target = current.Target
	}
	if leaseDuration <= 0 {
		return Record{}, Error{Action: opInstallSuccessor, Reason: "lease duration must be positive"}
	}
	result := *current
	result.Revision++
	result.State = stateActive
	result.Authority = op.NewAuthority
	result.Target = target
	result.Continuity++
	result.LeaseExpiresAt = now + int64(leaseDuration.Seconds())
	result.GraceExpiresAt = now + int64(leaseDuration.Seconds()) + int64(graceDuration.Seconds())
	result.RecoveryExpiresAt = 0
	result.ConflictIdentifier = ""
	return result, nil
}
