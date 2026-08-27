package state

func attachMaterializedRecord(index uint32, decision *verifiedEpochDecision) {
	if index >= uint32(len(decision.accepted)) {
		return
	}
	record := decision.accepted[index]
	domain, err := assignedDomain(decision.epoch, record.family)
	if err != nil {
		return
	}
	decision.Snapshot.RecordPresent = true
	decision.Snapshot.NodeID = record.nodeID
	copy(decision.Snapshot.NodePublicKey[:], record.publicKey)
	decision.Snapshot.RecordGeneration = record.generation
	decision.Snapshot.RecordValidFrom = record.notBefore
	decision.Snapshot.RecordValidUntil = record.notAfter
	decision.Snapshot.DeclaredFamily = record.family
	decision.Snapshot.ProbeEndpoint = record.endpoint
	decision.Snapshot.CarrierProfile = record.carrier
	decision.Snapshot.ProbeCapacity = record.capacity
	decision.Snapshot.Assignment = domain
	decision.Snapshot.AssignmentDigest = epochAssignmentDigest(decision.epoch.networkID, decision.epoch.number,
		decision.epoch.assignmentSeed, record.family, domain)
}
