package state

import (
	"crypto/ed25519"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
)

// Config identifies one owned state root and the complete policy needed to
// open it. The broad record is intentional: Open validates cross-field trust,
// source, clock, refresh, and resource constraints atomically. Authorities and
// keys are copied. Exactly one of Now or Clock supplies verification time;
// Clock may be called concurrently after Open. The zero Config is invalid.
type Config struct {
	Root        string
	NetworkID   [32]byte
	Authorities map[[32]byte]ed25519.PublicKey
	Threshold   int
	// AcceptedProfile is empty for the Stage 1 role probe or explicitly names
	// the replaceable H3 Route tracer profile.
	AcceptedProfile string
	Now             time.Time
	Clock           func() time.Time

	Source source.Config

	// ClockObservation is the initial independent observation. ObserveClock
	// and ClockObservationFile are alternative live owners. Automatic refresh
	// requires a live observation and finite source plan. RuntimeProfile enables
	// the H3-S governor; ObserveResources receives newly allocated JSON records
	// serially, and an error terminates live operation.
	ClockObservation         time.Time
	ClockObservationFile     string
	ObserveClock             func() time.Time
	AutomaticRefreshInterval time.Duration
	RuntimeProfile           string
	ObserveResources         func([]byte) error
}

// Snapshot is the immutable, complete projection of one verified generation
// plus its finite-source state. The broad record is intentional: callers use a
// single atomic value rather than assembling security-relevant identity,
// validity, assignment, and freshness fields from separate reads.
type Snapshot struct {
	Generation         string
	NetworkID          [32]byte
	Epoch              uint64
	Digest             [32]byte
	EpochValidFrom     time.Time
	ValidUntil         time.Time
	Profile            string
	ViewRoot           [32]byte
	ViewLength         uint32
	RejectedRoot       [32]byte
	RejectedLength     uint32
	Freshness          string
	Conflicting        bool
	SourceAttempts     uint16
	SourceOutcomes     [4]string
	LatestCompleteness string
	ObservedEpochs     [4]uint64
	ObservedDigests    [4][32]byte
	TrustedTime        time.Time
	NextAutomatic      time.Time
	PendingEpoch       uint64
	PendingDigest      [32]byte
	PendingAt          time.Time
	RecordPresent      bool
	NodeID             [32]byte
	NodePublicKey      [32]byte
	RecordGeneration   uint64
	RecordValidFrom    time.Time
	RecordValidUntil   time.Time
	DeclaredFamily     string
	ProbeEndpoint      string
	ProbeCapacity      uint16
	Assignment         string
	AssignmentDigest   [32]byte
	CandidateCount     uint8
	Candidates         [64]routeCandidate
}

type routeCandidate struct {
	NodeID     [32]byte
	PublicKey  [32]byte
	Family     string
	Endpoint   string
	Capacity   uint16
	Domain     string
	ValidFrom  time.Time
	ValidUntil time.Time
}
