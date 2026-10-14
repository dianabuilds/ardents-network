package wire

const (
	// Profile is fixed independently of the Route profile; peers cannot select it.
	Profile = "ardents-service-connection-profile-v3"
	// MaximumDataBytes bounds one record allocation, not the logical stream.
	MaximumDataBytes = 16 << 10
	// InitialRequestBytes is exactly Challenge followed by initial Continuity.
	// It bounds read-ahead of that flight, never subsequent Application records.
	InitialRequestBytes = 2*(len(connectionPrefix)+2+commonHeaderSize) + 136 + 161
	// InitialResponseBytes is exactly Instance Proof followed by Continuity.
	InitialResponseBytes = 2*(len(connectionPrefix)+2+commonHeaderSize) + 96 + 161
)

// Challenge asks the exact independently verified Instance to authenticate
// this logical context. Fields alone confer neither local nor remote authority.
type Challenge struct {
	Network, Target, Context, Nonce [32]byte
	InstanceGeneration              uint64
}

// Proof binds the Instance signature to the complete canonical Challenge.
type Proof struct {
	ChallengeDigest [32]byte
	Signature       [64]byte
}

// Role is the fixed endpoint direction in the authenticated transcript.
type Role byte

const (
	RoleInitiator Role = 1
	RolePublisher Role = 2
)

// Continuity binds retained logical offsets to one fresh TLS Attachment.
type Continuity struct {
	Role                                    Role
	AttachmentGeneration                    uint64
	SendBase, SendEnd, ReceiveNext          uint64
	Nonce, Context, ExporterCommitment, MAC [32]byte
}

// Data carries one nonempty bounded range in a logical direction.
type Data struct {
	AttachmentGeneration uint64
	Offset               uint64
	Payload              []byte
}

// Acknowledgement carries a data ACK, Terminal receipt (marker 1), or receipt
// confirmation (marker 2). Confirmation always implies Terminal.
type Acknowledgement struct {
	AttachmentGeneration           uint64
	Offset                         uint64
	Terminal, TerminalConfirmation bool
}

// Terminal closes one logical direction at its exact final offset.
type Terminal struct {
	AttachmentGeneration uint64
	Offset               uint64
}

// Record has exactly one kind. Unknown kinds and mixed records are refused.
type Record struct {
	Challenge       *Challenge
	Proof           *Proof
	Continuity      *Continuity
	Data            *Data
	Acknowledgement *Acknowledgement
	Terminal        *Terminal
}

// StreamRecord excludes every initial and recovery authentication record.
type StreamRecord struct {
	Data            *Data
	Acknowledgement *Acknowledgement
	Terminal        *Terminal
}
