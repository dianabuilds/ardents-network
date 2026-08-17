package bridge

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"
)

const (
	classAccepted            Class = "accepted"
	classAlreadyPresent      Class = "already-present"
	classInvalid             Class = "invalid"
	classIncompatible        Class = "incompatible"
	classWrongDomain         Class = "wrong-domain"
	classConflictingRole     Class = "conflicting-role"
	classSetFull             Class = "set-full"
	classReplacementRejected Class = "replacement-rejected"
	classExpired             Class = "expired"
	classReplay              Class = "replay"
)

func (owner *owner) validate(raw []byte) (invite, Class, error) {
	decoded, decodeClass := decodeInvite(raw)
	if decodeClass != classAccepted {
		return invite{}, decodeClass, nil
	}
	snapshot, err := owner.config.CurrentNetwork()
	if err != nil {
		return decoded, classInvalid, errors.New("read current authenticated Network State")
	}
	candidate, known := snapshot.BridgeCandidateByKey(decoded.issuerID)
	if !known {
		return decoded, classInvalid, nil
	}
	public := ed25519.PublicKey(candidate.PublicKey[:])
	signatureInput := make([]byte, 0, len(signatureTag)+1+len(decoded.body))
	signatureInput = append(signatureInput, signatureTag...)
	signatureInput = append(signatureInput, 0)
	signatureInput = append(signatureInput, decoded.body...)
	if !ed25519.Verify(public, signatureInput, decoded.signature) {
		return decoded, classInvalid, nil
	}
	if decoded.networkID != snapshot.NetworkID || decoded.epoch != snapshot.Epoch ||
		decoded.epochDigest != snapshot.Digest || decoded.profile != owner.config.RouteProfile {
		return decoded, classIncompatible, nil
	}
	if decoded.roleDomain != 1 ||
		decoded.identity != candidate.NodeID || decoded.family != candidate.FamilyID ||
		decoded.recordDigest != candidate.RecordDigest ||
		sha256.Sum256(decoded.domainProof) != candidate.DomainProofDigest ||
		decoded.assignmentNotAfter != candidate.AssignmentNotAfter.Unix() ||
		candidate.Domain != "initiator" {
		return decoded, classWrongDomain, nil
	}
	if snapshot.Freshness != "fresh" {
		return decoded, classIncompatible, nil
	}
	if snapshot.Conflicting {
		return decoded, classConflictingRole, nil
	}
	conflicting, err := owner.config.RoleConflict(decoded.identity, decoded.family)
	if err != nil {
		return decoded, classInvalid, errors.New("read current local role facts")
	}
	if conflicting {
		return decoded, classConflictingRole, nil
	}
	if !owner.config.TimeConfidence() {
		return decoded, classIncompatible, nil
	}
	now := owner.config.Clock().UTC()
	notBefore := time.Unix(decoded.notBefore, 0).UTC()
	notAfter := time.Unix(decoded.notAfter, 0).UTC()
	if now.Before(notBefore) {
		return decoded, classIncompatible, nil
	}
	if !notBefore.Before(notAfter) || !now.Before(notAfter) ||
		notBefore.Before(snapshot.EpochValidFrom) || notAfter.After(snapshot.ValidUntil) ||
		notBefore.Before(candidate.ValidFrom) || notAfter.After(candidate.ValidUntil) ||
		notAfter.After(candidate.AssignmentNotAfter) {
		return decoded, classExpired, nil
	}
	commitment, profile, err := owner.config.ValidateCandidate(decoded.candidate, decoded.identity)
	if err != nil || commitment == ([32]byte{}) || len(profile) > 63 || !ascii(profile) {
		return decoded, classInvalid, nil
	}
	decoded.commitment = commitment
	decoded.adapterProfile = profile
	return decoded, classAccepted, nil
}
