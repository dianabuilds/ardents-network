package connection

import "time"

const (
	// Profile is the only endpoint record profile accepted by native Service
	// Connection v1. It is never negotiated or chosen by a peer.
	Profile = "ardents-interactive-route-v2"

	// MaximumDataBytes is a parser/allocation bound, not a product stream
	// limit or workload contract.
	MaximumDataBytes = 16 << 10
)

// ContextInput contains every immutable fact bound to one logical connection.
// The package hashes this exact canonical sequence; callers cannot supply a
// precomputed context or select a profile.
type ContextInput struct {
	Network, Target, InstancePublic, PublicationDigest        [32]byte
	InstanceGeneration                                        uint64
	CandidateView, IsolationContext, DestinationBinding       [32]byte
	WorkSafetyNotAfter, WorkSafetyMaximum, NoNewRecoveryAfter int64
}

// DestinationBinding is the immutable Service Name provenance that a logical
// connection pins for its entire lifetime. It contains no Namespace record or
// mutable resolution owner.
type DestinationBinding struct {
	Name             string
	Generation       uint64
	Revision         uint64
	Authority        string
	Target           [32]byte
	ParentName       string
	ParentGeneration uint64
	RecordDigest     [32]byte
	Commitment       [32]byte
}

// Recovery fixes the immutable constraints for one fresh Route Attachment.
// The lifecycle replaces only the attachment generation and deadline; it must
// carry every other fact unchanged into each opening attempt.
type Recovery struct {
	Generation         uint64
	Deadline           time.Time
	NetworkID          [32]byte
	CandidateView      [32]byte
	IsolationContext   [32]byte
	DestinationBinding [32]byte
	RouteProfile       string
	Role               string
	WorkSafetyNotAfter int64
	WorkSafetyMaximum  int64
	NoNewRecoveryAfter int64
}

// Challenge proves the exact Target and Instance before application success.
type Challenge struct {
	Network, Target, Context, Nonce [32]byte
	InstanceGeneration              uint64
}

// Proof binds an Instance signature to the exact canonical Challenge record.
type Proof struct {
	ChallengeDigest [32]byte
	Signature       [64]byte
}

// Role identifies the fixed endpoint direction of a continuity exchange.
type Role byte

const (
	RoleClient    Role = 1
	RolePublisher Role = 2
)

// Continuity binds one fresh TLS attachment to its immutable logical context.
type Continuity struct {
	Role                                    Role
	AttachmentGeneration                    uint64
	SendBase, SendEnd, ReceiveNext          uint64
	Nonce, Context, ExporterCommitment, MAC [32]byte
}

// Data carries one bounded logical stream range.
type Data struct {
	AttachmentGeneration uint64
	Offset               uint64
	Payload              []byte
}

// Acknowledgement advances one logical send acknowledgement.
type Acknowledgement struct {
	AttachmentGeneration uint64
	Offset               uint64
}

// Terminal closes one attachment at the exact received logical offset.
type Terminal struct {
	AttachmentGeneration uint64
	Offset               uint64
}

// Record is one member of the closed native record set. Exactly one field is
// non-nil after Read; unknown record kinds cannot be represented.
type Record struct {
	Challenge       *Challenge
	Proof           *Proof
	Continuity      *Continuity
	Data            *Data
	Acknowledgement *Acknowledgement
	Terminal        *Terminal
}

// StreamRecord is the closed record subset permitted after Instance and
// Continuity authentication. Handshake records can never be interpreted as
// application-stream traffic.
type StreamRecord struct {
	Data            *Data
	Acknowledgement *Acknowledgement
	Terminal        *Terminal
}
