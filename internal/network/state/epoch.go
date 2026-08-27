package state

import (
	"fmt"
)

type candidateDecision struct {
	epoch      epochEnvelope
	epochBytes []byte
	inputs     [][]byte
	snapshot   Snapshot
	verified   verifiedEpochDecision
}

func verifyDecision(config config, current *Snapshot, epochBytes []byte, inputs, materials [][]byte, requireMaterials bool) (candidateDecision, error) {
	policy := epochPolicy{
		NetworkID: config.networkID, Authorities: config.authorities,
		Threshold: config.threshold, Profile: config.acceptedProfile, Now: config.now,
		MaterializationIndex: config.sourceInfo.MaterialIndex,
	}
	if current != nil {
		policy.Previous = &epochVerificationSnapshot{Epoch: current.Epoch, Digest: current.Digest}
	}
	verified, err := verifyEpochDecision(policy, epochBytes, inputs, materials, requireMaterials)
	if err != nil {
		return candidateDecision{}, err
	}
	snapshot := snapshotFromEpoch(verified.Snapshot)
	return candidateDecision{
		epoch:      verified.epoch,
		epochBytes: verified.EpochBytes,
		inputs:     verified.Inputs,
		snapshot:   snapshot,
		verified:   verified,
	}, nil
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

func verifyDecisionMaterials(decision candidateDecision, materials [][]byte) error {
	if err := decision.verified.VerifyMaterials(materials); err != nil {
		return fmt.Errorf("verify Candidate Materialization: %w", err)
	}
	return nil
}
