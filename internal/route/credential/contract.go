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

// ClosedIssuerRootReceipt contains only immutable public key-profile bytes.
type ClosedIssuerRootReceipt struct {
	Profile       []byte
	ProfileDigest [32]byte
}
