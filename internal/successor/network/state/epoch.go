package state

import (
	"fmt"

	epoch2 "github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

const (
	closedRouteProfile      = epoch2.ProfileClosedRoute
	closedTCPCarrierProfile = epoch2.CarrierClosedTCP
	maximumEpochBytes       = epoch2.MaxEpochBytes
	maximumRecordBytes      = epoch2.MaxRecordBytes
)

func verifyDecision(config config, previous *epoch2.Snapshot, epochBytes []byte, inputs, materials [][]byte, requireMaterials bool) (epoch2.Decision, error) {
	policy := epoch2.Policy{
		NetworkID: config.networkID, Authorities: config.authorities,
		Threshold: config.threshold, Profile: config.acceptedProfile, Now: config.now,
		MaterializationIndex: config.sourceInfo.MaterialIndex, Previous: previous,
	}
	return epoch2.Verify(policy, epochBytes, inputs, materials, requireMaterials)
}

func authenticateDecision(config config, epochBytes []byte, inputs, materials [][]byte, requireMaterials bool) (epoch2.Decision, error) {
	return epoch2.Authenticate(epoch2.Policy{
		NetworkID: config.networkID, Authorities: config.authorities,
		Threshold: config.threshold, Profile: config.acceptedProfile, Now: config.now,
		MaterializationIndex: config.sourceInfo.MaterialIndex,
	}, epochBytes, inputs, materials, requireMaterials)
}

func epochPredecessor(current *epoch2.Decision) *epoch2.Snapshot {
	if current == nil {
		return nil
	}
	return &epoch2.Snapshot{Epoch: current.Snapshot.Epoch, Digest: current.Snapshot.Digest}
}

func snapshotFromEpoch(value epoch2.Snapshot) Snapshot {
	return Snapshot{
		Generation: value.Generation, NetworkID: value.NetworkID,
		Epoch: value.Epoch, Digest: value.Digest,
		EpochValidFrom: value.EpochValidFrom, ValidUntil: value.ValidUntil,
		Profile: value.Profile, ViewRoot: value.ViewRoot, ViewLength: value.ViewLength,
		RejectedRoot: value.RejectedRoot, RejectedLength: value.RejectedLength,
		SourceNodeID: value.NodeID, SourceFamily: value.DeclaredFamily,
	}
}

func verifyDecisionMaterials(decision epoch2.Decision, materials [][]byte) error {
	if err := decision.VerifyMaterials(materials); err != nil {
		return fmt.Errorf("verify Candidate Materialization: %w", err)
	}
	return nil
}
