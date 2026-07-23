package identity

import (
	"crypto/ed25519"
	"errors"
)

var ErrInvalidIdentifier = errors.New("identity identifier is invalid")

// PrincipalID derives the canonical portable Principal identifier from an
// Ed25519 public key.
func PrincipalID(public ed25519.PublicKey) (string, error) {
	return identityID("p1_", "ardents:principal:v1\x00", public)
}

// DeviceID derives the canonical revocation identifier for an Ed25519 device
// public key. A DeviceID is not a Principal.
func DeviceID(public ed25519.PublicKey) (string, error) {
	return identityID("d1_", "ardents:device:v1\x00", public)
}

func identityID(prefix, domain string, public ed25519.PublicKey) (string, error) {
	if len(public) != ed25519.PublicKeySize {
		return "", ErrInvalidIdentifier
	}
	return digestID(prefix, []byte(domain), public), nil
}
