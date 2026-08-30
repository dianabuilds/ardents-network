package credential

import (
	"context"
	"crypto/ed25519"
	"time"
)

const (
	maximumProfileBytes = 4096
	messageSize         = 768
	maximumEnvelopeSize = 2 << 10
)

// Profile is the issuer Node's signed common OHTTP configuration. State
// authenticates the selected Node association; this package verifies the
// Node-owned key material and declared purpose-scoped Grant signer.
type Profile struct {
	Version                          uint8
	NetworkID, NodeID, GrantSignerID [32]byte
	GrantSignerPublicKey             [32]byte
	KeyConfig                        []byte
	KeyConfigDigest                  [32]byte
	AssignmentNotAfter               time.Time
	Signature                        []byte
}

// Request is the whole plaintext of one OHTTP issuance exchange. It is the
// same adjacent-hop tuple that the resulting Transit Grant binds, and has no
// Service Name, Target, Descriptor, Publisher, or sealed introduction.
type Request struct {
	RequestID, NetworkID, Digest, IntroductionNodeID, AttachmentID, ClientKeyDigest [32]byte
	Epoch                                                                           uint64
	NotAfter                                                                        time.Time
}

// Outcome is the authenticated, fixed-size issuer result vocabulary.
type Outcome string

const (
	Issued      Outcome = "issued"
	Exhausted   Outcome = "exhausted"
	Withdrawn   Outcome = "withdrawn"
	Unavailable Outcome = "unavailable"
)

// Result contains a Transit Grant only for Issued. Every Result uses the same
// padded OHTTP plaintext size.
type Result struct {
	Outcome Outcome
	Grant   []byte
}

// Authorization is the issuer-local current State and bounded-admission
// decision. It receives no Entry Invite, browser demand, or Service data.
type Authorization func(Request, time.Time) bool

// StateDuty is the current State fact an issuer must recheck before it signs.
// It names the already-selected issuer and Initiator duties only; it carries
// no Entry Invite, Service, Endpoint, or browser input.
type StateDuty struct {
	NetworkID, Digest, IssuerNodeID, IssuerPublicKey, InitiatorNodeID, InitiatorPublicKey, GrantSignerPublicKey [32]byte
	Epoch                                                                                                       uint64
	NotAfter                                                                                                    time.Time
	Withdrawn                                                                                                   bool
}

// CurrentDuty returns the current authenticated State projection for the
// issuer. The surrounding Node lifecycle owns State acquisition and refresh;
// Issuer accepts no static replacement duty.
type CurrentDuty func() (StateDuty, bool)

// IssuerConfig owns one State-selected, project-operated alpha issuer duty.
// IdentityKey signs its public OHTTP profile; GrantSigner is the distinct
// purpose-scoped signer selected by that State-authenticated profile. CurrentDuty rechecks the
// selected issuer/Initiator duty before every signature. Authorize owns the
// project-operated alpha's separate admission decision. DutyRoot owns the
// stable OHTTP secret plus bounded budget/idempotency state.
type IssuerConfig struct {
	NetworkID, NodeID  [32]byte
	IdentityKey        ed25519.PrivateKey
	GrantSigner        ed25519.PrivateKey
	InitiatorNodeID    [32]byte
	InitiatorPublicKey [32]byte
	DutyRoot           string
	CreateDutyRoot     bool
	Budget             uint16
	CurrentDuty        CurrentDuty
	Clock              func() time.Time
	Authorize          Authorization
}

// ClientConfig supplies one already State-selected issuer identity/profile
// and one Endpoint-owned opaque exchange. It cannot discover an issuer or
// supply an HTTP destination literal.
type ClientConfig struct {
	NetworkID, IssuerPublic [32]byte
	Profile                 Profile
	Exchange                Exchange
	At, Deadline            time.Time
}

// Exchange carries exactly one encapsulated OHTTP message through the
// already-admitted Credential Relay. It has no retry, target, or URL input.
type Exchange func(context.Context, []byte) ([]byte, error)
