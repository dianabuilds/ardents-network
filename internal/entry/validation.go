package entry

import (
	"crypto/ed25519"
	"errors"
	"net"
	"strconv"
	"time"
)

func (owner *owner) validate(raw []byte) (invite, Candidate, Class, error) {
	decoded, class := decodeInvite(raw)
	if class != Accepted {
		return invite{}, Candidate{}, class, nil
	}
	view, err := owner.config.Current()
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
	conflicting, err := owner.config.Conflict(decoded.nodeID, decoded.familyID)
	if err != nil {
		return decoded, Candidate{}, Invalid, errors.New("read current local Entry duty facts")
	}
	if conflicting {
		return decoded, Candidate{}, ConflictingRole, nil
	}
	if !owner.config.TimeConfident() {
		return decoded, Candidate{}, Incompatible, nil
	}
	now := owner.config.Clock().UTC()
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

func candidateByKey(view View, keyID [32]byte) (Candidate, bool) {
	if len(view.Candidates) == 0 || len(view.Candidates) > 64 {
		return Candidate{}, false
	}
	for _, candidate := range view.Candidates {
		if candidate.KeyID == keyID && candidate.PublicKey != [32]byte{} {
			return candidate, true
		}
	}
	return Candidate{}, false
}

func validEndpoint(value string) bool {
	host, port, err := net.SplitHostPort(value)
	number, portErr := strconv.Atoi(port)
	return err == nil && net.ParseIP(host) != nil && portErr == nil && number >= 1 && number <= 65535
}
