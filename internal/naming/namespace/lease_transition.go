package namespace

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
	Name                string
	Generation          uint64
	Revision            uint64
	Lease               string
	Consistency         string
	Recovery            string
	Authority           string
	Target              [32]byte
	RecordNotAfter      int64
	ParentName          string
	ParentGeneration    uint64
	LeaseExpiresAt      int64
	GraceExpiresAt      int64
	RecoveryExpiresAt   int64
	RecoveryStartedAt   int64
	RecoveryOperation   [32]byte
	RecoverySuccessor   [32]byte
	RecoveryPolicy      [32]byte
	RecoveryPolicyRev   uint64
	RecoveryPolicyDelay int64
	PendingPolicy       [32]byte
	PendingPolicyRev    uint64
	PendingPolicyDelay  int64
	PolicyActivatesAt   int64
	Continuity          uint64
	ConflictIdentifier  string
}

// Op is one transition input. Every transition against an existing Record is
// bound to its exact generation and revision. Parents are ordered immediate
// parent first and root last.
type Op struct {
	Kind                  string
	Name                  string
	Generation            uint64
	ClaimOrdinal          uint32
	ExpectedGeneration    uint64
	ExpectedRevision      uint64
	Authority             string
	Target                [32]byte
	RecordNotAfter        int64
	Parents               []Record
	LeaseDuration         time.Duration
	GraceDuration         time.Duration
	ConflictContext       string
	SuccessorAuthority    string
	PolicyDigest          [32]byte
	PolicyRevision        uint64
	PolicyDelay           time.Duration
	PolicyActivatesAt     int64
	RecoveryAuthorization Authorization
}

type transitionError struct {
	Action string
	Reason string
}

func (e transitionError) Error() string {
	return fmt.Sprintf("naming transition %q failed: %s", e.Action, e.Reason)
}

const (
	opClaim                  = "claim"
	opRenew                  = "renew"
	opRelease                = "release"
	opAdvance                = "advance"
	opConflict               = "conflict"
	opPublish                = "publish"
	opRotate                 = "rotate"
	opTransfer               = "transfer"
	opScheduleRecoveryPolicy = "schedule-recovery-policy"
	opActivateRecoveryPolicy = "activate-recovery-policy"
	opStartRecovery          = "start-recovery"
	opCancelRecovery         = "cancel-recovery"
	opCompleteRecovery       = "complete-recovery"
	opResumeRecovery         = "resume-recovery"
)

// Apply applies one transition without authenticating its caller. Authority
// authentication and ordering proofs belong to later, explicitly authorized
// Stage 6 slices.
func Apply(current *Record, now int64, op Op, policy Policy) (Record, error) {
	return apply(current, now, now*1_000, op, policy)
}

// ApplyAt applies one transition against a single Gateway-owned decision time.
// Lease state retains epoch seconds, while signed Recovery and Policy boundaries
// retain epoch milliseconds. Callers must not reconstruct one unit from the
// other before passing this Module the original decision time.
func ApplyAt(current *Record, now time.Time, op Op, policy Policy) (Record, error) {
	return apply(current, now.Unix(), now.UnixMilli(), op, policy)
}

func apply(current *Record, seconds, milliseconds int64, op Op, policy Policy) (Record, error) {
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
		return applyClaim(current, seconds, op, leaseDuration, graceDuration)
	case opRenew:
		return applyRenew(current, seconds, op, leaseDuration, graceDuration)
	case opRelease:
		return applyRelease(current, op)
	case opAdvance:
		return applyAdvance(current, seconds, op)
	case opConflict:
		return applyConflict(current, op)
	case opPublish:
		return applyPublish(current, seconds, op)
	case opRotate, opTransfer:
		return applyRotate(current, seconds, op)
	case opScheduleRecoveryPolicy, opActivateRecoveryPolicy:
		return applyRecoveryPolicy(current, seconds, milliseconds, op)
	case opStartRecovery, opCancelRecovery, opCompleteRecovery, opResumeRecovery:
		return applyRecovery(current, seconds, milliseconds, op)
	default:
		return Record{}, transitionError{Action: op.Kind, Reason: "operation is unavailable in S6.1"}
	}
}
