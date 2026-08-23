package entry

import (
	"crypto/ed25519"
	"errors"
	"time"
)

// Verify validates one R-077 Invite against current Initiator State and duty
// facts. It is the Initiator-side Entry port: callers receive only a bounded
// authorization and adjacent candidate, never a User identity or Entry root.
func Verify(raw []byte, input Verification) (Authorization, Candidate, Class, error) {
	decoded, candidate, class, err := validateInvite(raw, input)
	if err != nil || class != Accepted {
		return Authorization{}, Candidate{}, class, err
	}
	return Authorization{InviteID: decoded.id, NetworkID: decoded.networkID, Digest: decoded.epochDigest,
		Epoch: decoded.epoch, InitiatorNodeID: decoded.nodeID, NotAfter: time.Unix(decoded.notAfter, 0).UTC()}, candidate, Accepted, nil
}

func validateInvite(raw []byte, input Verification) (invite, Candidate, Class, error) {
	decoded, class := decodeInvite(raw)
	if class != Accepted {
		return invite{}, Candidate{}, class, nil
	}
	if input.Current == nil || input.Conflict == nil || input.Clock == nil || input.TimeConfident == nil {
		return decoded, Candidate{}, Invalid, errors.New("Entry verification is incomplete")
	}
	view, err := input.Current()
	if err != nil {
		return decoded, Candidate{}, Invalid, errors.New("read current authenticated Entry view")
	}
	candidate, found := candidateByKey(view, decoded.issuerID)
	if !found {
		return decoded, Candidate{}, Invalid, nil
	}
	if !ed25519.Verify(ed25519.PublicKey(candidate.PublicKey[:]), signatureInput(decoded.body), decoded.signature) {
		return decoded, Candidate{}, Invalid, nil
	}
	if view.NetworkID != decoded.networkID || view.Epoch != decoded.epoch || view.Digest != decoded.epochDigest ||
		view.Profile != profileID || decoded.profile != profileID || !view.Fresh {
		return decoded, Candidate{}, Incompatible, nil
	}
	if candidate.Domain != "initiator" || decoded.nodeID != candidate.NodeID || decoded.familyID != candidate.FamilyID ||
		decoded.recordDigest != candidate.RecordDigest || decoded.domainProofDigest != candidate.DomainProofDigest ||
		decoded.assignmentNotAfter != candidate.AssignmentNotAfter.Unix() || !validEndpoint(candidate.Endpoint) || candidate.Capacity == 0 {
		return decoded, Candidate{}, WrongDomain, nil
	}
	conflicting, err := input.Conflict(decoded.nodeID, decoded.familyID)
	if err != nil {
		return decoded, Candidate{}, Invalid, errors.New("read current local Entry duty facts")
	}
	if conflicting {
		return decoded, Candidate{}, ConflictingRole, nil
	}
	if !input.TimeConfident() {
		return decoded, Candidate{}, Incompatible, nil
	}
	now := input.Clock().UTC()
	notBefore, notAfter := time.Unix(decoded.notBefore, 0).UTC(), time.Unix(decoded.notAfter, 0).UTC()
	if now.Before(notBefore) {
		return decoded, Candidate{}, Incompatible, nil
	}
	if !notBefore.Before(notAfter) || !now.Before(notAfter) || notBefore.Before(candidate.ValidFrom) ||
		notAfter.After(candidate.ValidUntil) || notAfter.After(candidate.AssignmentNotAfter) {
		return decoded, Candidate{}, Expired, nil
	}
	return decoded, candidate, Accepted, nil
}
