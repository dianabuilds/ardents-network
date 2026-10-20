package epoch

import (
	"bytes"
	"crypto/ed25519"
	"errors"
)

// VerifyInitialClosedRecord verifies one signed generation-one ARNR-v2 Record
// against the exact canonical public preparation. Returned facts are detached
// copies. A valid self-asserted Record grants no membership, duty or currentness.
func VerifyInitialClosedRecord(prepared, signed []byte) (InitialClosedRecord, error) {
	r, err := parseRecord(signed)
	if err != nil || r.generation != 1 || r.capability != 2 || !r.signatureValid() {
		return InitialClosedRecord{}, errors.New("initial closed Record verification failed")
	}
	facts := InitialClosedRecord{NetworkID: r.networkID, NodeID: r.nodeID,
		ValidFrom: r.notBefore, ValidUntil: r.notAfter, Family: r.family,
		Endpoint: r.endpoint, Carrier: r.carrier, Capacity: r.capacity,
		PublicKey: bytes.Clone(r.publicKey)}
	canonical, err := PrepareInitialClosedRecord(facts)
	if err != nil || !bytes.Equal(canonical, prepared) || !bytes.Equal(canonical, signed[:len(signed)-ed25519.SignatureSize]) {
		return InitialClosedRecord{}, errors.New("initial closed Record preparation mismatch")
	}
	return facts, nil
}
