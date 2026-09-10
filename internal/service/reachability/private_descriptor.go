package reachability

import (
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"time"
)

// MaximumPrivateDescriptorSize bounds the complete generation-3 proof before
// it enters a fixed-size protected terminal request or response.
const MaximumPrivateDescriptorSize = 15000

const privateDescriptorVersion = uint16(3)

// PrivateIntroduction is the Instance-signed live recipient fact. It exposes
// no data Rendezvous, join secret, endpoint address or local Isolation Context.
type PrivateIntroduction struct {
	Revision                   uint64
	NodeID, Slot, RecipientKey [32]byte
	NotBefore, NotAfter        time.Time
}

// PrivateIssueInput supplies the already verified publication, independently
// generated recipient slot/key and the current State profile commitment. The
// signer must belong to the exact delegated Instance; it grants no State duty.
type PrivateIssueInput struct {
	Current        publication.Current
	ProfileDigest  [32]byte
	Introduction   PrivateIntroduction
	InstanceSigner crypto.Signer
}

// VerifyPrivate requires the exact Target, Network and current State profile.
// The legacy Verify interface deliberately cannot accept this generation.
func VerifyPrivate(raw []byte, target, network, profile [32]byte, at time.Time) (Verified, error) {
	if target == [32]byte{} || network == [32]byte{} || profile == [32]byte{} || at.IsZero() {
		return Verified{}, errors.New("private reachability verification input is invalid")
	}
	value, err := decodePrivateDescriptor(raw)
	if err != nil || value.NetworkID != network || value.Target != target || value.ProfileDigest != profile ||
		publication.Target(value.AuthorityPublic) != target {
		return Verified{}, errors.New("private reachability target or profile differs")
	}
	current, err := publication.Decode(value.Publication, ed25519.PublicKey(value.AuthorityPublic[:]), network, at)
	if err != nil || current.Credential.Target != target || current.Credential.AuthorityPublic != value.AuthorityPublic ||
		current.Digest != value.PublicationDigest || sha256.Sum256(value.Publication) != value.PublicationDigest ||
		!validPrivateIntroduction(value.Private, current.Credential) || at.Before(value.Private.NotBefore) || !at.Before(value.Private.NotAfter) {
		return Verified{}, errors.New("private reachability publication or recipient is invalid")
	}
	if !ed25519.Verify(ed25519.PublicKey(current.Credential.InstancePublic[:]), privateDescriptorTranscript(value), value.Signature[:]) {
		return Verified{}, errors.New("private reachability Instance signature is invalid")
	}
	return Verified{Descriptor: value, Current: current}, nil
}

func validPrivateIntroduction(value PrivateIntroduction, credential publication.Credential) bool {
	return value.Revision != 0 && value.NodeID != [32]byte{} && value.Slot != [32]byte{} && value.RecipientKey != [32]byte{} &&
		value.NotBefore.Unix() > 0 && value.NotBefore.Equal(value.NotBefore.UTC().Truncate(time.Second)) &&
		value.NotAfter.Equal(value.NotAfter.UTC().Truncate(time.Second)) && value.NotBefore.Before(value.NotAfter) &&
		value.NotAfter.Sub(value.NotBefore) <= 600*time.Second && value.NotBefore.Unix() >= credential.NotBefore &&
		value.NotAfter.Unix() <= credential.NotAfter
}

// VerifyPrivatePublication verifies a publisher's complete proof when the
// receiving Store has no preselected Target. Target derivation from the
// Authority and exact publication remains mandatory; lookup uses VerifyPrivate
// with its independently selected Target instead.
func VerifyPrivatePublication(raw []byte, network, profile [32]byte, at time.Time) (Verified, error) {
	descriptor, err := decodePrivateDescriptor(raw)
	if err != nil {
		return Verified{}, err
	}
	return VerifyPrivate(raw, descriptor.Target, network, profile, at)
}
