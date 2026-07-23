package identity

import "crypto/ed25519"

type Service interface {
	EnsureNode(StateStore, KeyStore) (Summary, ed25519.PrivateKey, error)
	NodeSummary() Summary
	NodeSource() string
	NodeState() string
}
