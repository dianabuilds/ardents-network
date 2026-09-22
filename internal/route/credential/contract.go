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

// ClosedIssuerRootConfig creates or reopens one fresh owner-only RSA key root
// for the finite closed admission profile. IdentityKey signs only the exported
// public key profile; no admission authority key enters this configuration.
type ClosedIssuerRootConfig struct {
	Root                string
	NetworkID, NodeID   [32]byte
	IdentityKey         ed25519.PrivateKey
	NotBefore, NotAfter time.Time
	Clock               func() time.Time
}

// ClosedIssuerKey is one public class/window RSA-PSS configuration from an
// immutable closed issuer root.
type ClosedIssuerKey struct {
	WindowStart time.Time
	Class       byte
	SPKI        []byte
	KeyID       [32]byte
}

// ClosedIssuerProfile is the Node-signed public inventory that State uses as
// exact input when preparing the closed profile. It contains no private key,
// permission, holder, or admission-authority material.
type ClosedIssuerProfile struct {
	NetworkID, NodeID   [32]byte
	NotBefore, NotAfter time.Time
	Keys                []ClosedIssuerKey
	Signature           [ed25519.SignatureSize]byte
}

// ClosedIssuerRootReceipt contains only immutable public key-profile bytes.
type ClosedIssuerRootReceipt struct {
	Profile       []byte
	ProfileDigest [32]byte
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
