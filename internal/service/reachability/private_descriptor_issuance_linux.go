//go:build linux

package reachability

import (
	"crypto"
	"crypto/ed25519"
	"errors"
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
