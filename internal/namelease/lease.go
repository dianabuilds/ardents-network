package namelease

import (
	"fmt"
	"time"
)

// Policy defines the timing parameters for Lease transitions.
type Policy struct {
	DefaultLeaseDuration time.Duration
	DefaultGraceDuration time.Duration
}

const (
	leaseActive   = "active"
	leaseGrace    = "grace"
	leaseReleased = "released"

	consistencyCurrent     = "current"
	consistencyConflict    = "conflict"
	consistencyFork        = "fork"
	consistencyUnavailable = "unavailable"

	recoveryStable  = "stable"
	recoveryPending = "recovery-pending"
)

// Record is one immutable revision inside one Name Generation.
type Record struct {
	Name               string
	Generation         uint64
	Revision           uint64
	Lease              string
	Consistency        string
	Recovery           string
	Authority          string
	Target             string
	ParentName         string
	ParentGeneration   uint64
	LeaseExpiresAt     int64
	GraceExpiresAt     int64
	RecoveryExpiresAt  int64
	Continuity         uint64
	ConflictIdentifier string
}

// Op is one transition input. Every transition against an existing Record is
// bound to its exact generation and revision. Parents are ordered immediate
// parent first and root last.
type Op struct {
	Kind               string
	Name               string
	Generation         uint64
	ExpectedGeneration uint64
	ExpectedRevision   uint64
	Authority          string
	Parents            []Record
	LeaseDuration      time.Duration
	GraceDuration      time.Duration
	ConflictContext    string
}

type transitionError struct {
	Action string
	Reason string
}

func (e transitionError) Error() string {
	return fmt.Sprintf("naming transition %q failed: %s", e.Action, e.Reason)
}

const (
	opClaim    = "claim"
	opRenew    = "renew"
	opRelease  = "release"
	opAdvance  = "advance"
	opConflict = "conflict"
)

// Apply applies one transition without authenticating its caller. Authority
// authentication and ordering proofs belong to later, explicitly authorized
// Stage 6 slices.
func Apply(current *Record, now int64, op Op, policy Policy) (Record, error) {
	leaseDuration := op.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = policy.DefaultLeaseDuration
	}
	graceDuration := op.GraceDuration
	if graceDuration <= 0 {
		graceDuration = policy.DefaultGraceDuration
	}
	if leaseDuration <= 0 {
		leaseDuration = time.Hour
	}
	if graceDuration < 0 {
		graceDuration = 0
	}
	if err := validOperationName(op.Name); err != nil {
		return Record{}, transitionError{Action: op.Kind, Reason: err.Error()}
	}

	switch op.Kind {
	case opClaim:
		return applyClaim(current, now, op, leaseDuration, graceDuration)
	case opRenew:
		return applyRenew(current, now, op, leaseDuration, graceDuration)
	case opRelease:
		return applyRelease(current, op)
	case opAdvance:
		return applyAdvance(current, now, op)
	case opConflict:
		return applyConflict(current, op)
	default:
		return Record{}, transitionError{Action: op.Kind, Reason: "operation is unavailable in S6.1"}
	}
}
