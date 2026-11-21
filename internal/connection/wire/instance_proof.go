package wire

import (
	"crypto/ed25519"
	"errors"
)

// verifyInstanceProof checks the exact pending canonical Challenge, never the
// digest supplied by the peer alone. The initial exchange must separately pin
// challenge fields to original verified authority and fresh Attachment facts,
// verify TLS and Continuity, and reobserve live owners before any effect. This
// mechanism returns no initial-state receipt or signing authority.
func verifyInstanceProof(expected [32]byte, challenge Challenge, proof Proof) error {
	if expected == [32]byte{} {
		return errors.New("service Instance proof key is absent")
	}
	digest, err := ChallengeDigest(challenge)
	if err != nil || digest != proof.ChallengeDigest ||
		!ed25519.Verify(ed25519.PublicKey(expected[:]), digest[:], proof.Signature[:]) {
		return errors.Join(errors.New("service Instance proof differs"), err)
	}
	return nil
}
