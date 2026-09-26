//go:build linux

package reachability

import (
	"crypto"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// IssuePrivate binds one short-lived recipient to the exact public publication
// proof and closed profile. It does not register the slot or announce readiness.
func IssuePrivate(input PrivateIssueInput) ([]byte, Descriptor, error) {
	current, err := verifiedCurrent(input.Current)
	if err != nil || input.ProfileDigest == [32]byte{} || input.InstanceSigner == nil ||
		!validPrivateIntroduction(input.Introduction, current.Credential) {
		return nil, Descriptor{}, errors.New("private reachability issue input is invalid")
	}
	public, ok := input.InstanceSigner.Public().(ed25519.PublicKey)
	if !ok || string(public) != string(current.Credential.InstancePublic[:]) {
		return nil, Descriptor{}, errors.New("private reachability Instance signer differs")
	}
	value := Descriptor{Version: privateDescriptorVersion, NetworkID: current.Credential.NetworkID,
		Target: current.Credential.Target, AuthorityPublic: current.Credential.AuthorityPublic,
		Publication: append([]byte(nil), current.Record...), PublicationDigest: current.Digest,
		ProfileDigest: input.ProfileDigest, Private: input.Introduction}
	body, err := encodePrivateDescriptorBody(value)
	if err != nil {
		return nil, Descriptor{}, err
	}
	signature, err := input.InstanceSigner.Sign(nil, privateDescriptorTranscript(value), crypto.Hash(0))
	if err != nil || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(public, privateDescriptorTranscript(value), signature) {
		return nil, Descriptor{}, errors.New("private reachability Instance signature failed")
	}
	copy(value.Signature[:], signature)
	return append(body, signature...), cloneDescriptor(value), nil
}

// verifiedCurrent re-verifies the exact supplied publication record against its
// own Authority credential. It serves the private issuance path only; ADR-0105
// retired the generation-2 writers that once shared it.
func verifiedCurrent(value publication.Current) (publication.Current, error) {
	if value.Credential.AuthorityPublic == [32]byte{} || value.Credential.NetworkID == [32]byte{} || len(value.Record) == 0 {
		return publication.Current{}, errors.New("publication is incomplete")
	}
	return publication.Decode(value.Record, ed25519.PublicKey(value.Credential.AuthorityPublic[:]), value.Credential.NetworkID,
		time.Unix(value.Credential.NotBefore, 0).UTC())
}
