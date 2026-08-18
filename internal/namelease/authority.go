package namelease

import (
	"strings"
	"time"
)

func applyTransfer(current *Record, now int64, op Op, leaseDuration, graceDuration time.Duration) (Record, error) {
	if current == nil {
		return Record{}, Error{Action: opTransfer, Reason: "name is unclaimed"}
	}
	if current.Name != op.Name {
		return Record{}, Error{Action: opTransfer, Reason: "record name mismatch"}
	}
	if op.Authority != current.Authority {
		return Record{}, Error{Action: opTransfer, Reason: "authority mismatch"}
	}
	if current.State != stateActive && current.State != stateGrace {
		return Record{}, Error{Action: opTransfer, Reason: "record cannot be transferred in this state"}
	}
	if op.NewAuthority == "" {
		return Record{}, Error{Action: opTransfer, Reason: "successor authority is required"}
	}
	target := strings.TrimSpace(op.Target)
	if target == "" {
		target = current.Target
	}
	if leaseDuration <= 0 {
		return Record{}, Error{Action: opTransfer, Reason: "lease duration must be positive"}
	}
	result := *current
	result.Revision++
	result.Authority = op.NewAuthority
	result.Continuity++
	result.Target = target
	result.State = stateActive
	result.LeaseExpiresAt = now + int64(leaseDuration.Seconds())
	result.GraceExpiresAt = now + int64(leaseDuration.Seconds()) + int64(graceDuration.Seconds())
	result.RecoveryExpiresAt = 0
	result.ConflictIdentifier = ""
	return result, nil
}
