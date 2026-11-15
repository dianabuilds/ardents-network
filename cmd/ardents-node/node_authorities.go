package main

import (
	"crypto/ed25519"
	"crypto/sha256"
)

// nodeAuthorityPins projects detached public keys for a single State opener.
// It supplies no authenticated Network view and shares no live state.
func nodeAuthorityPins(authorities [][32]byte) (map[[32]byte]ed25519.PublicKey, bool) {
	pins := make(map[[32]byte]ed25519.PublicKey, len(authorities))
	for _, public := range authorities {
		id := sha256.Sum256(public[:])
		if _, duplicate := pins[id]; duplicate || public == [32]byte{} {
			return nil, false
		}
		pins[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	return pins, true
}
