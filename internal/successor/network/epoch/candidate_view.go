package epoch

import (
	"bytes"
	"errors"
	"fmt"
)

type rejection struct {
	index uint32
	code  uint16
	raw   []byte
}

func verifyEpochCandidate(config Policy, epochBytes []byte, inputs [][]byte, materials []materialization, requireMaterials bool) (Decision, error) {
	if err := preflightDecision(epochBytes, inputs, materials); err != nil {
		return Decision{}, err
	}
	epoch, err := verifyEpoch(config, epochBytes)
	if err != nil {
		return Decision{}, err
	}
	if len(inputs) != int(epoch.cutoff) || len(inputs) > 64 {
		return Decision{}, errors.New("input log does not match the committed cutoff")
	}
	if epochCommitmentRoot(inputs, emptyInputTag) != epoch.inputRoot {
		return Decision{}, errors.New("input log root does not match the epoch")
	}
	accepted, rejected, view, err := evaluateInputs(config, epoch, inputs)
	if err != nil {
		return Decision{}, err
	}
	if err := verifyViewCommitment(epoch, accepted, rejected); err != nil {
		return Decision{}, err
	}
	if err := verifyCandidateSummaries(epoch, view); err != nil {
		return Decision{}, err
	}
	if err := verifyMaterializations(epoch, accepted, materials, requireMaterials); err != nil {
		return Decision{}, err
	}
	generation := fmt.Sprintf("%x", epoch.digest)
	decision := Decision{
		epoch:      epoch,
		Header:     headerFromEnvelope(epoch),
		EpochBytes: append([]byte(nil), epochBytes...),
		Inputs:     cloneInputs(inputs),
		accepted:   accepted,
		rejections: rejected,
		Snapshot: Snapshot{
			Generation:     generation,
			NetworkID:      epoch.networkID,
			Epoch:          epoch.number,
			Digest:         epoch.digest,
			PreviousDigest: epoch.previous,
			EpochValidFrom: epoch.validFrom,
			ValidUntil:     epoch.validUntil,
			Profile:        epoch.profile,
			ViewRoot:       epoch.viewRoot,
			ViewLength:     epoch.viewLength,
			RejectedRoot:   epoch.rejectedRoot,
			RejectedLength: epoch.rejectedLength,
		},
	}
	if err := attachCandidates(&decision, accepted, epoch); err != nil {
		return Decision{}, err
	}
	if err := attachDestinationResolutionGateway(&decision); err != nil {
		return Decision{}, err
	}
	if err := attachTransitIssuanceDuty(&decision); err != nil {
		return Decision{}, err
	}
	attachMaterializedRecord(config.MaterializationIndex, &decision)
	return decision, nil
}

func preflightDecision(epoch []byte, inputs [][]byte, materials []materialization) error {
	if len(epoch) == 0 || len(epoch) > maximumEpochBytes || len(inputs) > 64 || len(materials) > 64 {
		return errors.New("offline decision exceeds its framing bounds")
	}
	for _, input := range inputs {
		if len(input) == 0 || len(input) > maximumRecordBytes {
			return errors.New("offline input exceeds its framing bounds")
		}
	}
	for _, material := range materials {
		if len(material.record) == 0 || len(material.record) > maximumRecordBytes || len(material.siblings) > 64 {
			return errors.New("materialization exceeds its framing bounds")
		}
	}
	return nil
}

func verifyViewCommitment(epoch epochEnvelope, accepted []nodeRecord, rejected []rejection) error {
	acceptedBytes := make([][]byte, len(accepted))
	for index, record := range accepted {
		acceptedBytes[index] = record.raw
	}
	rejectionLeaves := make([][32]byte, len(rejected))
	for index, item := range rejected {
		rejectionLeaves[index] = epochRejectionLeaf(item.index, item.code, item.raw)
	}
	if uint32(len(accepted)) != epoch.viewLength || uint32(len(rejected)) != epoch.rejectedLength ||
		epochCommitmentRoot(acceptedBytes, emptyViewTag) != epoch.viewRoot ||
		epochHashedCommitmentRoot(rejectionLeaves, emptyRejectionTag) != epoch.rejectedRoot {
		return errors.New("candidate view or rejection commitment is inconsistent")
	}
	return nil
}

func verifyMaterializations(epoch epochEnvelope, accepted []nodeRecord, materials []materialization, required bool) error {
	if len(accepted) > 0 && required && len(materials) == 0 {
		return errors.New("candidate materialization is required")
	}
	seen := make(map[uint32]bool, len(materials))
	for _, material := range materials {
		if material.epochDigest != epoch.digest || material.index >= uint32(len(accepted)) || seen[material.index] {
			return errors.New("candidate materialization identity is invalid")
		}
		seen[material.index] = true
		record := accepted[material.index].raw
		if !bytes.Equal(material.record, record) || !verifyEpochCommitment(record, material.index, uint32(len(accepted)), material.siblings, epoch.viewRoot) {
			return errors.New("candidate materialization proof is invalid")
		}
	}
	return nil
}

func cloneInputs(inputs [][]byte) [][]byte {
	cloned := make([][]byte, len(inputs))
	for index := range inputs {
		cloned[index] = append([]byte(nil), inputs[index]...)
	}
	return cloned
}
