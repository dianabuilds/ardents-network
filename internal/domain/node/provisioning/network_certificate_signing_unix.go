//go:build linux || darwin

package provisioning

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

// This fixed purpose borrows the original offline signing custody and output
// lifetime. Neither caller-supplied X.509 fields nor a generic signer cross it.
func admitNodeCertificate(raw, signed []byte, pin string) (*networkSigning, error) {
	var plan struct {
		Schema    string          `json:"schema"`
		Reference string          `json:"reference_time"`
		Record    json.RawMessage `json:"record"`
	}
	if preparationObject(raw, &plan, "schema", "reference_time", "record") != nil || plan.Schema != "ardents-node-certificate-plan-v1" {
		return nil, errNetworkSigning
	}
	reference, err := preparationTime(plan.Reference)
	if err != nil {
		return nil, errNetworkSigning
	}
	_, prepared, err := PrepareInitialNodeRecord(plan.Record)
	digest := sha256.Sum256(signed)
	if err != nil || pin != hex.EncodeToString(digest[:]) {
		return nil, errNetworkSigning
	}
	facts, err := epoch.VerifyInitialClosedRecord(prepared, signed)
	if err != nil || facts.Carrier != epoch.CarrierClosedTCP {
		return nil, errNetworkSigning
	}
	return &networkSigning{operation: "prepare-node-certificate", public: bytes.Clone(facts.PublicKey),
		networkSigningInterval: networkSigningInterval{reference: reference, from: facts.ValidFrom, until: facts.ValidUntil}}, nil
}

func (s *networkSigning) nodeCertificate(key ed25519.PrivateKey) ([]networkSignedArtifact, error) {
	return s.nodeCertificateWithRandom(key, rand.Reader)
}

// The reader seam isolates entropy failure; the production operation always
// supplies crypto/rand.Reader and accepts no caller-controlled randomness.
func (s *networkSigning) nodeCertificateWithRandom(key ed25519.PrivateKey, random io.Reader) ([]networkSignedArtifact, error) {
	serial, err := rand.Int(random, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, errors.Join(errNetworkSigning, err)
	}
	if serial.Sign() <= 0 || s.checkTime(time.Now()) != nil {
		return nil, errNetworkSigning
	}
	facts := epoch.InitialClosedRecord{PublicKey: s.public, ValidFrom: s.from, ValidUntil: s.until}
	raw, err := createNodeCertificateWithSerial(facts, key, random, serial)
	if err != nil || len(raw) == 0 {
		return nil, errors.Join(errNetworkSigning, err)
	}
	return []networkSignedArtifact{{"node-certificate.pem", raw}}, nil
}
