//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

// The declaration embeds the exact public preparation plan, not arbitrary
// signing bytes. Each explicit operation admits only its own schema and field.
type networkSigningPlan struct {
	Schema    string          `json:"schema"`
	Reference string          `json:"reference_time"`
	Record    json.RawMessage `json:"record"`
	Epoch     json.RawMessage `json:"epoch"`
}

type networkSigning struct {
	operation string
	networkSigningInterval
	message []byte
	public  ed25519.PublicKey
	network [32]byte
	records [][]byte
	profile *closedprofile.Context
}

type networkSignedArtifact struct {
	name string
	raw  []byte
}

func admitNetworkSigning(operation string, raw, prepared []byte, pin string) (*networkSigning, error) {
	if !isNetworkSigning(operation) || len(raw) > maximumNetworkPreparationPlan {
		return nil, errNetworkSigning
	}
	if operation == "sign-closed-profile" {
		return admitClosedProfileSigning(raw, prepared, pin)
	}
	if operation == "prepare-node-certificate" {
		return admitNodeCertificate(raw, prepared, pin)
	}
	field, prepare := "record", "prepare-initial-node-record"
	if operation == "sign-initial-epoch" {
		field, prepare = "epoch", "prepare-initial-epoch"
	}
	var plan networkSigningPlan
	schemaPurpose := field
	if field == "record" {
		schemaPurpose = "node-record"
	}
	if preparationObject(raw, &plan, "schema", "reference_time", field) != nil || plan.Schema != "ardents-initial-"+schemaPurpose+"-signing-plan-v1" {
		return nil, errNetworkSigning
	}
	reference, err := admissionTime(plan.Reference)
	if err != nil {
		return nil, errNetworkSigning
	}
	publicPlan := plan.Record
	if field == "epoch" {
		publicPlan = plan.Epoch
	}
	_, message, err := prepareNetworkPublic(prepare, publicPlan)
	digest := sha256.Sum256(message)
	if err != nil || pin != hex.EncodeToString(digest[:]) || !bytes.Equal(message, prepared) {
		return nil, errNetworkSigning
	}
	s := &networkSigning{operation: operation, networkSigningInterval: networkSigningInterval{reference: reference}, message: message}
	if field == "record" {
		var p recordPreparationPlan
		if json.Unmarshal(publicPlan, &p) != nil {
			return nil, errNetworkSigning
		}
		s.from, _ = admissionTime(p.From)
		s.until, _ = admissionTime(p.Until)
		s.public = bytes.Clone(p.PublicKey[:])
		s.network = p.Network
	} else {
		var p epochPreparationPlan
		if json.Unmarshal(publicPlan, &p) != nil || len(p.Authorities) != 1 {
			return nil, errNetworkSigning
		}
		s.from, _ = admissionTime(p.From)
		s.until, _ = admissionTime(p.Until)
		s.public = bytes.Clone(p.Authorities[0][:])
		s.network = p.Network
		for _, record := range p.Records {
			s.records = append(s.records, bytes.Clone(record))
		}
	}
	return s, nil
}

// No private material crosses into Network. The fixed transcripts below
// are admitted only after canonical preparation, exact artifact and key checks.
func (s *networkSigning) sign(key ed25519.PrivateKey, now time.Time) ([]networkSignedArtifact, error) {
	if s.checkTime(now) != nil || len(key) != ed25519.PrivateKeySize || !bytes.Equal(key[32:], s.public) {
		return nil, errNetworkSigning
	}
	if s.operation == "sign-closed-profile" {
		return s.signProfile(key, now)
	}
	if s.operation == "prepare-node-certificate" {
		return s.nodeCertificate(key)
	}
	if s.operation == "sign-initial-node-record" {
		signature := ed25519.Sign(key, s.message)
		if !ed25519.Verify(s.public, s.message, signature) {
			return nil, errNetworkSigning
		}
		return []networkSignedArtifact{{"node-record.bin", append(bytes.Clone(s.message), signature...)}}, nil
	}
	if s.operation != "sign-initial-epoch" {
		return nil, errNetworkSigning
	}
	digest, id := sha256.Sum256(s.message), sha256.Sum256(s.public)
	signature := ed25519.Sign(key, digest[:])
	if !ed25519.Verify(s.public, digest[:], signature) {
		return nil, errNetworkSigning
	}
	signed := append(bytes.Clone(s.message), 1)
	signed = append(signed, id[:]...)
	signed = append(signed, signature...)
	policy := epoch.Policy{NetworkID: s.network, Authorities: map[[32]byte]ed25519.PublicKey{id: s.public}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: now}
	decision, err := epoch.Verify(policy, signed, s.records, nil, false)
	if err != nil || len(decision.Candidates) != len(s.records) {
		return nil, errNetworkSigning
	}
	outputs := []networkSignedArtifact{{"epoch.bin", signed}}
	for i, record := range s.records {
		outputs = append(outputs, networkSignedArtifact{fmt.Sprintf("input-%02d.bin", i), record})
	}
	materials := make([][]byte, len(decision.Candidates))
	for i := range materials {
		materials[i], err = decision.Materialization(uint32(i))
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, networkSignedArtifact{fmt.Sprintf("material-%02d.bin", i), materials[i]})
	}
	if _, err = epoch.Verify(policy, signed, s.records, materials, true); err != nil {
		return nil, err
	}
	return outputs, nil
}
