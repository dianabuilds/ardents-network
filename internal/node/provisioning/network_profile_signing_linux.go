//go:build linux

package provisioning

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
)

// This declaration admits only closed-profile signing. The State authority is
// an explicit independent pin, never derived from private material or the
// Admission permission authority inside the nested public profile.
type closedProfileSigningPlan struct {
	Schema    string          `json:"schema"`
	Reference string          `json:"reference_time"`
	Authority string          `json:"state_authority_key"`
	Profile   json.RawMessage `json:"profile"`
}

func admitClosedProfileSigning(raw, prepared []byte, pin string) (*networkSigning, error) {
	var plan closedProfileSigningPlan
	if preparationObject(raw, &plan, "schema", "reference_time", "state_authority_key", "profile") != nil || plan.Schema != "ardents-closed-profile-signing-plan-v1" {
		return nil, errNetworkSigning
	}
	authorityPin, ok := decodeHex32(plan.Authority)
	if !ok || authorityPin == [32]byte{} {
		return nil, errNetworkSigning
	}
	authority := authorityPin[:]
	reference, err := preparationTime(plan.Reference)
	if err != nil {
		return nil, errNetworkSigning
	}
	input, body, err := PrepareClosedNetworkProfile(plan.Profile)
	digest := sha256.Sum256(body)
	if err != nil || pin != hex.EncodeToString(digest[:]) || !bytes.Equal(body, prepared) {
		return nil, errNetworkSigning
	}
	if bytes.Equal(authority, input.IssuanceAuthorityKey[:]) {
		return nil, errNetworkSigning
	}
	context := &closedprofile.Context{NetworkID: input.NetworkID, StateGeneration: input.StateGeneration,
		EpochDigest: input.EpochDigest, Epoch: input.Epoch, Authority: authority, Now: reference}
	if _, err = closedprofile.SigningTranscript(body, *context); err != nil {
		return nil, errNetworkSigning
	}
	return &networkSigning{operation: "sign-closed-profile", networkSigningInterval: networkSigningInterval{reference: reference, from: input.NotBefore, until: input.NotAfter}, message: body, public: authority, network: input.NetworkID, profile: context}, nil
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
