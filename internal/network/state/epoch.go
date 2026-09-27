package state

import (
	"fmt"
)

func verifyDecision(config config, previous *epochVerificationSnapshot, epochBytes []byte, inputs, materials [][]byte, requireMaterials bool) (verifiedEpochDecision, error) {
	policy := epochPolicy{
		NetworkID: config.networkID, Authorities: config.authorities,
		Threshold: config.threshold, Profile: config.acceptedProfile, Now: config.now,
		MaterializationIndex: config.sourceInfo.MaterialIndex, Previous: previous,
	}
	return verifyEpochDecision(policy, epochBytes, inputs, materials, requireMaterials)
}

func epochPredecessor(current *Snapshot) *epochVerificationSnapshot {
	if current == nil {
		return nil
	}
	return &epochVerificationSnapshot{Epoch: current.Epoch, Digest: current.Digest}
}

func snapshotFromEpoch(value epochVerificationSnapshot) Snapshot {
	return Snapshot{
		Generation: value.Generation, NetworkID: value.NetworkID,
		Epoch: value.Epoch, Digest: value.Digest,
		EpochValidFrom: value.EpochValidFrom, ValidUntil: value.ValidUntil,
		Profile: value.Profile, ViewRoot: value.ViewRoot, ViewLength: value.ViewLength,
		RejectedRoot: value.RejectedRoot, RejectedLength: value.RejectedLength,
		RecordPresent: value.RecordPresent, NodeID: value.NodeID,
		NodePublicKey: value.NodePublicKey, RecordGeneration: value.RecordGeneration,
		RecordValidFrom: value.RecordValidFrom, RecordValidUntil: value.RecordValidUntil,
		DeclaredFamily: value.DeclaredFamily, ProbeEndpoint: value.ProbeEndpoint,
		CarrierProfile: value.CarrierProfile,
		ProbeCapacity:  value.ProbeCapacity, Assignment: value.Assignment,
		AssignmentDigest:                 value.AssignmentDigest,
		DestinationResolutionNodeID:      value.DestinationResolutionNodeID,
		DestinationResolutionProfile:     value.DestinationResolutionProfile,
		DestinationResolutionProfileSize: value.DestinationResolutionProfileSize,
		TransitIssuanceNodeID:            value.TransitIssuanceNodeID,
		TransitIssuanceProfile:           value.TransitIssuanceProfile,
		TransitIssuanceProfileSize:       value.TransitIssuanceProfileSize,
	}
}

func verifyDecisionMaterials(decision verifiedEpochDecision, materials [][]byte) error {
	if err := decision.VerifyMaterials(materials); err != nil {
		return fmt.Errorf("verify Candidate Materialization: %w", err)
	}
	return nil
}
