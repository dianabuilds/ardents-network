package credential

import (
	"crypto/ed25519"
	"time"
)

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
