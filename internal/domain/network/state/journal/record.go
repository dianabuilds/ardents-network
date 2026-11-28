package journal

const MaximumExposures = 2

// These values are persisted in the ARDS1D4 distribution journal. Renaming
// in-memory states must never renumber an existing byte.
// Zero is the unset purpose in a fresh root.
const Refresh byte = 1

const (
	AttemptNotStarted byte = iota
	AttemptInFlight
	AttemptCompleted
	AttemptFailed // includes a persisted interrupted attempt
)

const byDigestSlotOffset = 2

// LATEST uses slots 0 and 1; BY_DIGEST uses slots 2 and 3.
func ByDigestSlot(sourceIndex int) int { return byDigestSlotOffset + sourceIndex }

// Record contains the fields of one retained ARDS1D4 distribution journal.
type Record struct {
	Sequence            uint64
	EpochFloor          uint64
	EpochDigest         [32]byte
	TrustedTimeFloor    int64
	Conflicting         bool
	ConsecutiveFailures uint64
	BackoffLevel        byte
	NextAutomatic       int64
	History             [][32]byte
	CycleID             uint64
	CycleActive         bool
	CyclePurpose        byte
	CycleStarted        int64
	CycleDeadline       int64
	Attempts            [4]byte
	Outcomes            [4]byte
	RequestedDigests    [2][32]byte
	ObservedEpochs      [4]uint64
	ObservedDigests     [4][32]byte
	PendingDigest       [32]byte
	PendingValidFrom    int64
	CycleSeed           [32]byte
	SourceOrder         [2]byte
}

const (
	OutcomeValid byte = iota + 1
	OutcomeUnavailable
	OutcomeAuthentication
	OutcomeTimeout
	OutcomeFraming
	OutcomeResource
	OutcomeCanceled
	OutcomeInvalidState
	OutcomeInterrupted
	OutcomeNotFound
	OutcomeBusy
	OutcomeBadRequest
	OutcomeInternal
)
