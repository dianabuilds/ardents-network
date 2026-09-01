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
	InitiatorNodeID                  [32]byte
	InitiatorPublicKey               [32]byte
	KeyConfig                        []byte
	KeyConfigDigest                  [32]byte
	AssignmentNotAfter               time.Time
	Signature                        []byte
}

// IssuerRootConfig identifies one explicit owner-only issuer bootstrap. It
// carries the Node identity only for this local profile-signing ceremony; the
// retained root never receives a Network State authority key.
type IssuerRootConfig struct {
	Root               string
	NetworkID          [32]byte
	NodeID             [32]byte
	IdentityKey        ed25519.PrivateKey
	InitiatorNodeID    [32]byte
	InitiatorPublicKey [32]byte
	AssignmentNotAfter time.Time
	Budget             uint16
	Clock              func() time.Time
}

// IssuerRootReceipt is the complete non-secret output of issuer
// initialization. Profile bytes are owned by the caller.
type IssuerRootReceipt struct {
	Profile       []byte
	ProfileDigest [32]byte
}

// RootIssuerConfig opens one already initialized issuer root against current
// authenticated State. Signer, OHTTP, profile, Initiator, and budget inputs
// are deliberately absent.
type RootIssuerConfig struct {
	Root        string
	NetworkID   [32]byte
	NodeID      [32]byte
	IdentityKey ed25519.PrivateKey
	CurrentDuty CurrentDuty
	Clock       func() time.Time
}

// Request is the whole plaintext of one OHTTP issuance exchange. It is the
// same adjacent-hop tuple that the resulting Transit Grant binds, and has no
// Service Name, Target, Descriptor, Publisher, or sealed introduction.
type Request struct {
	RequestID, NetworkID, Digest, TransitNodeID, AttachmentID, ClientKeyDigest [32]byte
	Epoch                                                                      uint64
	TransitRole                                                                byte
	NotAfter                                                                   time.Time
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

// StateDuty is the current State fact an issuer must recheck before it signs.
// It names the already-selected issuer and Initiator duties only; it carries
// no Entry Invite, Service, Endpoint, or browser input.
type StateDuty struct {
	Generation                                                                                                  string
	NetworkID, Digest, IssuerNodeID, IssuerPublicKey, InitiatorNodeID, InitiatorPublicKey, GrantSignerPublicKey [32]byte
	ProfileDigest                                                                                               [32]byte
	Epoch                                                                                                       uint64
	NotAfter                                                                                                    time.Time
	Fresh, Conflicting                                                                                          bool
	Withdrawn                                                                                                   bool
}

// CurrentDuty returns the current authenticated State projection for the
// issuer. The surrounding Node lifecycle owns State acquisition and refresh;
// Issuer accepts no static replacement duty.
type CurrentDuty func() (StateDuty, bool)

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
