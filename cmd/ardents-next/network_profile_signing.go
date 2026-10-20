//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
)

// This declaration admits only closed-profile signing. The State authority is
// an explicit independent pin, never derived from private material or the
// Admission permission authority inside the nested public profile.
type closedProfileSigningPlan struct {
	Schema    string          `json:"schema"`
	Reference string          `json:"reference_time"`
	Authority preparationID   `json:"state_authority_key"`
	Profile   json.RawMessage `json:"profile"`
}

func admitClosedProfileSigning(raw, prepared []byte, pin string) (*networkSigning, error) {
	var plan closedProfileSigningPlan
	if preparationObject(raw, &plan, "schema", "reference_time", "state_authority_key", "profile") != nil || plan.Schema != "ardents-closed-profile-signing-plan-v1" || plan.Authority == (preparationID{}) {
		return nil, errNetworkSigning
	}
	reference, err := admissionTime(plan.Reference)
	if err != nil {
		return nil, errNetworkSigning
	}
	_, body, err := prepareNetworkPublic("prepare-closed-profile", plan.Profile)
	digest := sha256.Sum256(body)
	if err != nil || pin != hex.EncodeToString(digest[:]) || !bytes.Equal(body, prepared) {
		return nil, errNetworkSigning
	}
	var p profilePreparationPlan
	if json.Unmarshal(plan.Profile, &p) != nil || plan.Authority == p.Authority {
		return nil, errNetworkSigning
	}
	from, _ := admissionTime(p.From)
	until, _ := admissionTime(p.Until)
	context := &closedprofile.Context{NetworkID: p.Network, StateGeneration: p.Generation, EpochDigest: p.Digest, Epoch: p.Epoch, Authority: bytes.Clone(plan.Authority[:]), Now: reference}
	if _, err = closedprofile.SigningTranscript(body, *context); err != nil {
		return nil, errNetworkSigning
	}
	return &networkSigning{operation: "sign-closed-profile", networkSigningInterval: networkSigningInterval{reference: reference, from: from, until: until}, message: body, public: bytes.Clone(plan.Authority[:]), network: p.Network, profile: context}, nil
}

func (s *networkSigning) signProfile(key ed25519.PrivateKey, now time.Time) ([]networkSignedArtifact, error) {
	if s.profile == nil {
		return nil, errNetworkSigning
	}
	context := *s.profile
	context.Now = now
	transcript, err := closedprofile.SigningTranscript(s.message, context)
	if err != nil {
		return nil, errNetworkSigning
	}
	signed := append(bytes.Clone(s.message), ed25519.Sign(key, transcript)...)
	if _, err = closedprofile.Verify(signed, context); err != nil {
		return nil, errNetworkSigning
	}
	return []networkSignedArtifact{{"profile.bin", signed}}, nil
}
