package state

import (
	"fmt"
	"time"

	stateepoch "github.com/dianabuilds/ardents-network/internal/network/epoch"
)

const (
	maximumEpochBytes  = 1 << 20
	maximumEpochChain  = 64
	maximumRecordBytes = 32 << 10
)

type epochEnvelope struct {
	number     uint64
	digest     [32]byte
	previous   [32]byte
	validFrom  time.Time
	validUntil time.Time
	cutoff     uint32
}

type candidateDecision struct {
	epoch      epochEnvelope
	epochBytes []byte
	inputs     [][]byte
	snapshot   Snapshot
	verified   stateepoch.Decision
}

func parseEpoch(raw []byte) (epochEnvelope, error) {
	value, err := stateepoch.Inspect(raw)
	if err != nil {
		return epochEnvelope{}, err
	}
	return epochEnvelope{
		number: value.Epoch, digest: value.Digest, previous: value.PreviousDigest,
		validFrom: value.EpochValidFrom, validUntil: value.ValidUntil,
		cutoff: value.ViewLength + value.RejectedLength,
	}, nil
}

func verifyDecision(config config, current *Snapshot, epochBytes []byte, inputs, materials [][]byte, requireMaterials bool) (candidateDecision, error) {
	policy := stateepoch.Policy{
		NetworkID: config.networkID, Authorities: config.authorities,
		Threshold: config.threshold, Profile: config.acceptedProfile, Now: config.now,
		MaterializationIndex: config.sourceInfo.MaterialIndex,
	}
	if current != nil {
		policy.Previous = &stateepoch.Snapshot{Epoch: current.Epoch, Digest: current.Digest}
	}
	verified, err := stateepoch.Verify(policy, epochBytes, inputs, materials, requireMaterials)
	if err != nil {
		return candidateDecision{}, err
	}
	snapshot := snapshotFromEpoch(verified.Snapshot)
	return candidateDecision{
		epoch: epochEnvelope{
			number: verified.Snapshot.Epoch, digest: verified.Snapshot.Digest,
			previous:  verified.Snapshot.PreviousDigest,
			validFrom: verified.Snapshot.EpochValidFrom, validUntil: verified.Snapshot.ValidUntil,
			cutoff: verified.Snapshot.ViewLength + verified.Snapshot.RejectedLength,
		},
		epochBytes: verified.EpochBytes,
		inputs:     verified.Inputs,
		snapshot:   snapshot,
		verified:   verified,
	}, nil
}

func snapshotFromEpoch(value stateepoch.Snapshot) Snapshot {
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
		ProbeCapacity: value.ProbeCapacity, Assignment: value.Assignment,
		AssignmentDigest: value.AssignmentDigest,
	}
}

func verifyDecisionMaterials(decision candidateDecision, materials [][]byte) error {
	if err := decision.verified.VerifyMaterials(materials); err != nil {
		return fmt.Errorf("verify Candidate Materialization: %w", err)
	}
	return nil
}
