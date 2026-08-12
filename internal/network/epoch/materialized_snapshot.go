package epoch

import "github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"

func attachMaterializedRecord(index uint32, decision *candidateDecision) {
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
	decision.Snapshot.ProbeCapacity = record.capacity
	decision.Snapshot.Assignment = domain
	decision.Snapshot.AssignmentDigest = assignment.Digest(decision.epoch.networkID, decision.epoch.number,
		decision.epoch.assignmentSeed, record.family, domain)
}
