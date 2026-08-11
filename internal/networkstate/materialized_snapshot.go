package networkstate

func attachMaterializedRecord(index uint32, decision *candidateDecision) {
	if index >= uint32(len(decision.accepted)) {
		return
	}
	record := decision.accepted[index]
	domain, err := assignedDomain(decision.epoch, record.family)
	if err != nil {
		return
	}
	decision.snapshot.RecordPresent = true
	decision.snapshot.NodeID = record.nodeID
	copy(decision.snapshot.NodePublicKey[:], record.publicKey)
	decision.snapshot.RecordGeneration = record.generation
	decision.snapshot.RecordValidFrom = record.notBefore
	decision.snapshot.RecordValidUntil = record.notAfter
	decision.snapshot.DeclaredFamily = record.family
	decision.snapshot.ProbeEndpoint = record.endpoint
	decision.snapshot.ProbeCapacity = record.capacity
	decision.snapshot.Assignment = domain
	decision.snapshot.AssignmentDigest = assignmentDigest(decision.epoch, record.family, domain)
}
