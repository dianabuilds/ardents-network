package epoch

import "crypto/sha256"

func attachCandidates(decision *candidateDecision, accepted []nodeRecord, epoch epochEnvelope) error {
	for index, record := range accepted {
		domain, err := assignedDomain(epoch, record.family)
		if err != nil {
			return err
		}
		var public [32]byte
		copy(public[:], record.publicKey)
		decision.NodeIDs = append(decision.NodeIDs, record.nodeID)
		decision.KeyIDs = append(decision.KeyIDs, record.keyID)
		decision.PublicKeys = append(decision.PublicKeys, public)
		decision.FamilyIDs = append(decision.FamilyIDs, sha256.Sum256([]byte(record.family)))
		decision.Families = append(decision.Families, record.family)
		decision.RecordDigests = append(decision.RecordDigests, sha256.Sum256(record.raw))
		proof, err := decision.Materialization(uint32(index))
		if err != nil {
			return err
		}
		decision.DomainProofs = append(decision.DomainProofs, proof)
		decision.Endpoints = append(decision.Endpoints, record.endpoint)
		decision.Capacities = append(decision.Capacities, record.capacity)
		decision.Domains = append(decision.Domains, domain)
		decision.ValidFrom = append(decision.ValidFrom, record.notBefore)
		decision.ValidUntil = append(decision.ValidUntil, record.notAfter)
		decision.AssignmentNotAfter = append(decision.AssignmentNotAfter, epoch.validUntil)
	}
	return nil
}
