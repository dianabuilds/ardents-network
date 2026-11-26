//go:build linux || darwin

package networksigning

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
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

func preparationObject(raw []byte, value any, keys ...string) error {
	var fields map[string]json.RawMessage
	if artifacts.DecodeDeclaration(raw, &fields, keys...) != nil || artifacts.DecodeObject(raw, value) != nil {
		return networkdraft.ErrUnavailable
	}
	return nil
}

func admitNetworkSigning(operation string, raw, prepared []byte, pin string) (*networkSigning, error) {
	if !IsPurpose(operation) || len(raw) > networkdraft.MaximumPlanBytes {
		return nil, errNetworkSigning
	}
	if operation == "sign-closed-profile" {
		return admitClosedProfileSigning(raw, prepared, pin)
	}
	if operation == "prepare-node-certificate" {
		return admitNodeCertificate(raw, prepared, pin)
	}
	field := "record"
	if operation == "sign-initial-epoch" {
		field = "epoch"
	}
	var plan networkSigningPlan
	schemaPurpose := field
	if field == "record" {
		schemaPurpose = "node-record"
	}
	if preparationObject(raw, &plan, "schema", "reference_time", field) != nil || plan.Schema != "ardents-initial-"+schemaPurpose+"-signing-plan-v1" {
		return nil, errNetworkSigning
	}
	reference, err := networkdraft.ParseTime(plan.Reference)
	if err != nil {
		return nil, errNetworkSigning
	}
	s := &networkSigning{operation: operation, networkSigningInterval: networkSigningInterval{reference: reference}}
	if field == "record" {
		input, message, err := networkdraft.NodeRecord(plan.Record)
		if err != nil {
			return nil, errNetworkSigning
		}
		s.from, s.until = input.ValidFrom, input.ValidUntil
		s.message, s.public, s.network = message, input.PublicKey, input.NetworkID
	} else {
		input, message, err := networkdraft.Epoch(plan.Epoch)
		if err != nil || len(input.AuthorityKeys) != 1 {
			return nil, errNetworkSigning
		}
		s.from, s.until = input.ValidFrom, input.ValidUntil
		s.message, s.public, s.network = message, input.AuthorityKeys[0], input.NetworkID
		s.records = input.Records
	}
	digest := sha256.Sum256(s.message)
	if pin != hex.EncodeToString(digest[:]) || !bytes.Equal(s.message, prepared) {
		return nil, errNetworkSigning
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
