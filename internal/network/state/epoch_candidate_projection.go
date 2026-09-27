package state

import "crypto/sha256"

func attachCandidates(decision *verifiedEpochDecision, accepted []nodeRecord, epoch epochEnvelope) error {
	for index, record := range accepted {
		domain, err := assignedDomain(epoch, record.family)
		if err != nil {
			return err
		}
		var public [32]byte
		copy(public[:], record.publicKey)
		proof, err := decision.Materialization(uint32(index))
		if err != nil {
			return err
		}
		decision.Candidates = append(decision.Candidates, verifiedCandidate{
			NodeID: record.nodeID, KeyID: record.keyID, PublicKey: public,
			FamilyID: sha256.Sum256([]byte(record.family)), RecordDigest: sha256.Sum256(record.raw),
			DomainProof: proof, Family: record.family, Endpoint: record.endpoint,
			CarrierProfile: record.carrier, Domain: domain, Capacity: record.capacity,
			ValidFrom: record.notBefore, ValidUntil: record.notAfter,
			AssignmentNotAfter: epoch.validUntil,
		})
	}
	return nil
}
