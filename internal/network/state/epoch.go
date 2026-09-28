package state

import (
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

const (
	interactiveRouteProfile                  = epoch.ProfileInteractiveRoute
	closedRouteProfile                       = epoch.ProfileClosedRoute
	closedTCPCarrierProfile                  = epoch.CarrierClosedTCP
	maximumEpochBytes                        = epoch.MaxEpochBytes
	maximumRecordBytes                       = epoch.MaxRecordBytes
	maximumDestinationResolutionProfileBytes = epoch.MaxDestinationResolutionProfileBytes
	maximumTransitIssuanceProfileBytes       = epoch.MaxTransitIssuanceProfileBytes
	destinationResolutionDomain              = epoch.DomainDestinationResolution
	transitIssuanceDomain                    = epoch.DomainTransitIssuance
)

func verifyDecision(config config, previous *epoch.Snapshot, epochBytes []byte, inputs, materials [][]byte, requireMaterials bool) (epoch.Decision, error) {
	policy := epoch.Policy{
		NetworkID: config.networkID, Authorities: config.authorities,
		Threshold: config.threshold, Profile: config.acceptedProfile, Now: config.now,
		MaterializationIndex: config.sourceInfo.MaterialIndex, Previous: previous,
	}
	return epoch.Verify(policy, epochBytes, inputs, materials, requireMaterials)
}

func epochPredecessor(current *epoch.Decision) *epoch.Snapshot {
	if current == nil {
		return nil
	}
	return &epoch.Snapshot{Epoch: current.Snapshot.Epoch, Digest: current.Snapshot.Digest}
}

func snapshotFromEpoch(value epoch.Snapshot) Snapshot {
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

func verifyDecisionMaterials(decision epoch.Decision, materials [][]byte) error {
	if err := decision.VerifyMaterials(materials); err != nil {
		return fmt.Errorf("verify Candidate Materialization: %w", err)
	}
	return nil
}
