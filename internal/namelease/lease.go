package namelease

import (
	"fmt"
	"time"
)

// Policy defines default timing parameters for Stage 6 lease transitions.
type Policy struct {
	DefaultLeaseDuration time.Duration
	DefaultGraceDuration time.Duration
	DefaultRecoveryDelay time.Duration
}

// Record stores the Stage 6 naming lifecycle outcome at one point in time.
type Record struct {
	Name               string
	Generation         uint64
	Revision           uint64
	State              string
	Authority          string
	Target             string
	ParentName         string
	LeaseExpiresAt     int64
	GraceExpiresAt     int64
	RecoveryExpiresAt  int64
	Continuity         uint64
	ConflictIdentifier string
	// Signature is the Ed25519 signature by the Record's Authority
	// over the canonical Record payload. This is feasibility work while R-044
	// remains open; it is not the accepted Stage 6 cryptographic suite. It is
	// optional during Apply; a verifier that requires authentication
	// must check it through Record.Verify.
	Signature []byte
}

// Op is a command-like transition input for Apply.
type Op struct {
	Kind            string
	Name            string
	Generation      uint64
	Authority       string
	NewAuthority    string
	Target          string
	ParentName      string
	LeaseDuration   time.Duration
	GraceDuration   time.Duration
	RecoveryDelay   time.Duration
	ConflictContext string
}

// Error represents a deterministic naming transition failure.
type Error struct {
	Action string
	Reason string
}

func (e Error) Error() string {
	return fmt.Sprintf("naming transition %q failed: %s", e.Action, e.Reason)
}

const (
	stateActive   = "active"
	stateGrace    = "grace"
	stateReleased = "released"
	stateConflict = "conflict"
	stateRecovery = "recovery-pending"
)

const (
	opClaim            = "claim"
	opRenew            = "renew"
	opRelease          = "release"
	opAdvance          = "advance"
	opStartRecovery    = "start-recovery"
	opInstallSuccessor = "install-successor"
	opConflict         = "conflict"
	opResolveConflict  = "resolve-conflict"
	opTransfer         = "transfer"
)

// Apply applies one immutable transition against one naming lifecycle record.
func Apply(current *Record, now int64, op Op, policy Policy) (Record, error) {
	leaseDuration := op.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = policy.DefaultLeaseDuration
	}
	graceDuration := op.GraceDuration
	if graceDuration <= 0 {
		graceDuration = policy.DefaultGraceDuration
	}
	recoveryDelay := policy.DefaultRecoveryDelay
	if leaseDuration <= 0 {
		leaseDuration = time.Hour
	}
	if graceDuration < 0 {
		graceDuration = 0
	}
	if recoveryDelay <= 0 {
		recoveryDelay = time.Minute
	}
	if op.Name == "" && op.Kind != opAdvance {
		return Record{}, Error{Action: op.Kind, Reason: "name is required"}
	}
	switch op.Kind {
	case opClaim:
		return applyClaim(current, now, op, leaseDuration, graceDuration)
	case opRenew:
		return applyRenew(current, now, op, leaseDuration, graceDuration)
	case opRelease:
		return applyRelease(current, op)
	case opAdvance:
		return applyAdvance(current, now)
	case opStartRecovery:
		return applyStartRecovery(current, now, op, recoveryDelay)
	case opInstallSuccessor:
		return applyInstallSuccessor(current, now, op, leaseDuration, graceDuration)
	case opTransfer:
		return applyTransfer(current, now, op, leaseDuration, graceDuration)
	case opConflict:
		return applyConflict(current, op)
	case opResolveConflict:
		return applyResolveConflict(current, now, op, leaseDuration, graceDuration)
	default:
		return Record{}, Error{Action: op.Kind, Reason: "unknown naming action"}
	}
}

// CanResolve returns whether this record can be used for successful name resolution now.
func CanResolve(current Record, now int64) (bool, string) {
	switch current.State {
	case stateActive:
		if now <= current.LeaseExpiresAt {
			return true, ""
		}
		return false, "lease has expired"
	case stateGrace:
		if now <= current.GraceExpiresAt {
			return true, "name is in grace and should be treated as volatile"
		}
		return false, "grace period has expired"
	case stateRecovery:
		return false, "recovery is pending"
	case stateConflict:
		return false, "name is in explicit conflict"
	case stateReleased:
		return false, "name is released"
	default:
		return false, "invalid naming state"
	}
}

func applyRelease(current *Record, op Op) (Record, error) {
	if current == nil {
		return Record{}, Error{Action: opRelease, Reason: "name is unclaimed"}
	}
	if current.Name != op.Name {
		return Record{}, Error{Action: opRelease, Reason: "record name mismatch"}
	}
	if op.Authority != current.Authority {
		return Record{}, Error{Action: opRelease, Reason: "authority mismatch"}
	}
	if current.State == stateReleased {
		return Record{}, Error{Action: opRelease, Reason: "name is already released"}
	}
	if current.State == stateConflict {
		return Record{}, Error{Action: opRelease, Reason: "conflict cannot force release (R-039)"}
	}
	result := *current
	result.Revision++
	result.State = stateReleased
	result.LeaseExpiresAt = 0
	result.GraceExpiresAt = 0
	result.RecoveryExpiresAt = 0
	result.ConflictIdentifier = ""
	return result, nil
}

func applyAdvance(current *Record, now int64) (Record, error) {
	if current == nil {
		return Record{}, Error{Action: opAdvance, Reason: "name is unclaimed"}
	}
	result := *current
	switch current.State {
	case stateActive:
		if now <= current.LeaseExpiresAt {
			return result, nil
		}
		if current.GraceExpiresAt == 0 || now > current.GraceExpiresAt {
			result.State = stateReleased
			result.Revision++
			result.LeaseExpiresAt = 0
			result.GraceExpiresAt = 0
			result.RecoveryExpiresAt = 0
			return result, nil
		}
		result.State = stateGrace
		result.Revision++
		return result, nil
	case stateGrace:
		if now <= current.GraceExpiresAt {
			return result, nil
		}
		result.State = stateReleased
		result.Revision++
		result.GraceExpiresAt = 0
		return result, nil
	case stateRecovery:
		if current.RecoveryExpiresAt == 0 {
			return Record{}, Error{Action: opAdvance, Reason: "recovery deadline is missing"}
		}
		if now <= current.RecoveryExpiresAt {
			return result, nil
		}
		result.State = stateReleased
		result.Revision++
		result.RecoveryExpiresAt = 0
		return result, nil
	default:
		return result, nil
	}
}
