package namelease

import (
	"strings"
	"time"
)

func applyClaim(current *Record, now int64, op Op, leaseDuration, graceDuration time.Duration) (Record, error) {
	if current != nil && current.Name != "" {
		if current.State != stateReleased {
			return Record{}, Error{Action: opClaim, Reason: "name is not available for claim"}
		}
		if op.Generation != 0 && op.Generation <= current.Generation {
			return Record{}, Error{Action: opClaim, Reason: "generation is not monotonic"}
		}
	}
	if op.Authority == "" {
		return Record{}, Error{Action: opClaim, Reason: "authority is required"}
	}
	target := strings.TrimSpace(op.Target)
	if target == "" {
		return Record{}, Error{Action: opClaim, Reason: "target is required"}
	}
	if leaseDuration <= 0 {
		return Record{}, Error{Action: opClaim, Reason: "lease duration must be positive"}
	}
	generation := op.Generation
	if generation == 0 {
		if current == nil || current.State == "" {
			generation = 1
		} else {
			generation = current.Generation + 1
		}
	}
	continuity := uint64(1)
	if current != nil {
		continuity = current.Continuity + 1
	}
	return Record{Name: strings.TrimSpace(op.Name), Generation: generation, Revision: 1, State: stateActive,
		Authority: op.Authority, Target: target, ParentName: strings.TrimSpace(op.ParentName),
		LeaseExpiresAt: now + int64(leaseDuration.Seconds()),
		GraceExpiresAt: now + int64(leaseDuration.Seconds()) + int64(graceDuration.Seconds()),
		Continuity:     continuity}, nil
}

func applyRenew(current *Record, now int64, op Op, leaseDuration, graceDuration time.Duration) (Record, error) {
	if current == nil {
		return Record{}, Error{Action: opRenew, Reason: "name is unclaimed"}
	}
	if current.Name != op.Name {
		return Record{}, Error{Action: opRenew, Reason: "record name mismatch"}
	}
	switch current.State {
	case stateActive:
		if now > current.LeaseExpiresAt {
			return Record{}, Error{Action: opRenew, Reason: "lease has already expired"}
		}
	case stateGrace:
		if now > current.GraceExpiresAt {
			return Record{}, Error{Action: opRenew, Reason: "grace period has already expired"}
		}
	default:
		return Record{}, Error{Action: opRenew, Reason: "naming state cannot be renewed"}
	}
	if op.Authority != current.Authority {
		return Record{}, Error{Action: opRenew, Reason: "authority mismatch"}
	}
	if op.Generation != 0 && op.Generation != current.Generation {
		return Record{}, Error{Action: opRenew, Reason: "generation is stale or inconsistent"}
	}
	target := strings.TrimSpace(op.Target)
	if target == "" {
		target = current.Target
	}
	if leaseDuration <= 0 {
		return Record{}, Error{Action: opRenew, Reason: "lease duration must be positive"}
	}
	result := *current
	result.Revision++
	result.State = stateActive
	result.Target = target
	result.LeaseExpiresAt = now + int64(leaseDuration.Seconds())
	result.GraceExpiresAt = now + int64(leaseDuration.Seconds()) + int64(graceDuration.Seconds())
	result.RecoveryExpiresAt = 0
	result.ConflictIdentifier = ""
	return result, nil
}
