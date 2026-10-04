package state

import (
	"crypto/ed25519"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
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
	// ClosedProfileAuthority is the separately selected, already pinned State
	// signer for an ardents-route-v3 profile. It is never caller-selected at
	// profile acceptance time.
	ClosedProfileAuthority ed25519.PublicKey
	// AcceptedProfile is empty or the selected ardents-route-v3 identity.
	// Earlier grammars are recovery evidence, not selectable live State modes.
	AcceptedProfile string
	Now             time.Time
	Clock           func() time.Time

	// Source declares acquisition and serving; State supplies its own TLS
	// verification clock. Callers leave Source.VerificationClock nil.
	Source source.Config

	// ClockObservation is the initial independent observation. ObserveClock
	// and ClockObservationFile are alternative live owners. Automatic refresh
	// requires a live observation and finite source plan. PermitWork borrows an
	// application admission decision; Network never samples process resources.
	ClockObservation         time.Time
	ClockObservationFile     string
	ObserveClock             func() time.Time
	AutomaticRefreshInterval time.Duration
	LocalRoleStateRoot       string
	PermitWork               func() error
}

// Snapshot is the immutable current projection of one verified generation
// plus its finite-Source diagnostics. It cannot authorize member or role work;
// only CurrentRuntime supplies a checked authenticated observation.
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
	// Source identity describes the local serving exclusion, not member authority.
	SourceNodeID [32]byte
	SourceFamily string
}
