//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
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

func parseNetworkSigningKey(raw []byte) (key ed25519.PrivateKey, err error) {
	block, rest := pem.Decode(raw)
	if block == nil {
		return nil, errNetworkSigning
	}
	defer clear(block.Bytes)
	canonical := pem.EncodeToMemory(block)
	defer clear(canonical)
	if len(rest) != 0 || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 || !bytes.Equal(canonical, raw) {
		return nil, errNetworkSigning
	}
	// ParsePKCS8PrivateKey deliberately ignores ASN.1 remainder and accepts
	// other algorithms. Admit one complete canonical Ed25519 structure first:
	// unsupported private objects must never be constructed merely to reject them.
	var encoding struct {
		Version    int
		Algorithm  pkix.AlgorithmIdentifier
		PrivateKey []byte
	}
	remainder, decodeErr := asn1.Unmarshal(block.Bytes, &encoding)
	defer clear(encoding.PrivateKey)
	if decodeErr != nil || len(remainder) != 0 || encoding.Version != 0 || !encoding.Algorithm.Algorithm.Equal(asn1.ObjectIdentifier{1, 3, 101, 112}) || len(encoding.Algorithm.Parameters.FullBytes) != 0 {
		return nil, errNetworkSigning
	}
	canonicalDER, marshalErr := asn1.Marshal(encoding)
	defer clear(canonicalDER)
	if marshalErr != nil || !bytes.Equal(canonicalDER, block.Bytes) {
		return nil, errNetworkSigning
	}
	var seed []byte
	remainder, decodeErr = asn1.Unmarshal(encoding.PrivateKey, &seed)
	defer clear(seed)
	if decodeErr != nil || len(remainder) != 0 || len(seed) != ed25519.SeedSize {
		return nil, errNetworkSigning
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok || len(key) != ed25519.PrivateKeySize {
		clear(key)
		return nil, errNetworkSigning
	}
	// Ed25519's cached public half must agree with its seed. Never admit an
	// inconsistent private key simply because Public() exposes its cached half.
	derived := ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])
	defer clear(derived)
	if !bytes.Equal(derived, key) {
		clear(key)
		return nil, errNetworkSigning
	}
	return key, nil
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
