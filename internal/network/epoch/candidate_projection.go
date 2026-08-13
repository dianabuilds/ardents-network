package epoch

func attachCandidates(decision *candidateDecision, accepted []nodeRecord, epoch epochEnvelope) error {
	for _, record := range accepted {
		domain, err := assignedDomain(epoch, record.family)
		if err != nil {
			return err
		}
		var public [32]byte
		copy(public[:], record.publicKey)
		decision.NodeIDs = append(decision.NodeIDs, record.nodeID)
		decision.KeyIDs = append(decision.KeyIDs, record.keyID)
		decision.PublicKeys = append(decision.PublicKeys, public)
		decision.Families = append(decision.Families, record.family)
		decision.Endpoints = append(decision.Endpoints, record.endpoint)
		decision.Capacities = append(decision.Capacities, record.capacity)
		decision.Domains = append(decision.Domains, domain)
		decision.ValidFrom = append(decision.ValidFrom, record.notBefore)
		decision.ValidUntil = append(decision.ValidUntil, record.notAfter)
	}
	return nil
}
