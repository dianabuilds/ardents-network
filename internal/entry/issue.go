package entry

import (
	"crypto/ed25519"
	"errors"
	"time"
)

// IssueInput is one bounded, State-referenced Entry Invite issuance request.
// The issuer supplies no Route, Target, Service, or User information.
type IssueInput struct {
	NetworkID, Digest, RecipientPublicKey [32]byte
	Epoch                                 uint64
	Candidate                             Candidate
	NotBefore, NotAfter                   time.Time
	Slot, Generation                      byte
	Replaces                              *[32]byte
}

// Issue returns the canonical signed Entry Invite v2. Custody of the selected
// candidate signer remains outside Entry; this function cannot import, retain,
// or activate the issued Invite.
func Issue(input IssueInput, signer ed25519.PrivateKey) ([]byte, error) {
	if err := validIssueInput(input, signer); err != nil {
		return nil, err
	}
	body := make([]byte, 0, 256)
	body = appendIssueUint16(body, inviteWireVersion)
	body = append(body, input.NetworkID[:]...)
	body = appendIssueUint64(body, input.Epoch)
	body = append(body, input.Digest[:]...)
	body = append(body, byte(len(profileID)))
	body = append(body, profileID...)
	body = append(body, input.RecipientPublicKey[:]...)
	for _, value := range [][32]byte{input.Candidate.KeyID, input.Candidate.NodeID, input.Candidate.FamilyID,
		input.Candidate.RecordDigest, input.Candidate.DomainProofDigest} {
		body = append(body, value[:]...)
	}
	body = appendIssueUint64(body, uint64(input.Candidate.AssignmentNotAfter.Unix()))
	body = appendIssueUint64(body, uint64(input.NotBefore.Unix()))
	body = appendIssueUint64(body, uint64(input.NotAfter.Unix()))
	body = append(body, input.Generation, input.Slot)
	if input.Replaces == nil {
		body = append(body, 0)
	} else {
		body = append(body, 1)
		body = append(body, input.Replaces[:]...)
	}
	signature := ed25519.Sign(signer, signatureInput(body))
	raw := make([]byte, 0, len(inviteMagic)+2+len(body)+len(signature))
	raw = append(raw, inviteMagic...)
	raw = appendIssueUint16(raw, uint16(len(body)))
	raw = append(raw, body...)
	return append(raw, signature...), nil
}

func appendIssueUint16(destination []byte, value uint16) []byte {
	return append(destination, byte(value>>8), byte(value))
}

func appendIssueUint64(destination []byte, value uint64) []byte {
	for shift := uint(56); ; shift -= 8 {
		destination = append(destination, byte(value>>shift))
		if shift == 0 {
			return destination
		}
	}
}

func validIssueInput(input IssueInput, signer ed25519.PrivateKey) error {
	if len(signer) != ed25519.PrivateKeySize || input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.RecipientPublicKey == [32]byte{} || input.Epoch == 0 ||
		input.Candidate.NodeID == [32]byte{} || input.Candidate.KeyID == [32]byte{} || input.Candidate.FamilyID == [32]byte{} ||
		input.Candidate.RecordDigest == [32]byte{} || input.Candidate.DomainProofDigest == [32]byte{} || input.Candidate.Domain != "initiator" ||
		input.Candidate.AssignmentNotAfter.IsZero() || input.NotBefore.IsZero() || input.NotAfter.IsZero() ||
		!input.NotBefore.Equal(input.NotBefore.UTC().Truncate(time.Second)) || !input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) ||
		!input.Candidate.AssignmentNotAfter.Equal(input.Candidate.AssignmentNotAfter.UTC().Truncate(time.Second)) || !input.NotBefore.Before(input.NotAfter) ||
		input.NotBefore.Before(input.Candidate.ValidFrom) || input.NotAfter.After(input.Candidate.ValidUntil) ||
		input.NotAfter.After(input.Candidate.AssignmentNotAfter) || input.Slot > 1 || input.Generation < 1 || input.Generation > 2 ||
		(input.Generation == 1 && input.Replaces != nil) || (input.Generation == 2 && input.Replaces == nil) {
		return errors.New("entry Invite issue input is invalid")
	}
	return nil
}
