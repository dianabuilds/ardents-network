package state

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
)

const (
	rejectMalformed       = uint16(1)
	rejectNetwork         = uint16(2)
	rejectSignature       = uint16(3)
	rejectTime            = uint16(4)
	rejectProfile         = uint16(5)
	rejectCapacity        = uint16(6)
	rejectSourceCollision = uint16(7)
	rejectNodeCollision   = uint16(8)
	rejectGeneration      = uint16(9)
	rejectKeyCollision    = uint16(10)
	rejectEndpoint        = uint16(11)
	rejectCarrier         = uint16(12)
)

type rejection struct {
	index uint32
	code  uint16
	raw   []byte
}

type evaluatedRecord struct {
	index  int
	record nodeRecord
	code   uint16
}

func verifyEpochCandidate(config epochPolicy, current *epochVerificationSnapshot, epochBytes []byte, inputs [][]byte, materials []materialization, requireMaterials bool) (verifiedEpochDecision, error) {
	if err := preflightDecision(epochBytes, inputs, materials); err != nil {
		return verifiedEpochDecision{}, err
	}
	epoch, err := verifyEpoch(config, current, epochBytes)
	if err != nil {
		return verifiedEpochDecision{}, err
	}
	if len(inputs) != int(epoch.cutoff) || len(inputs) > 64 {
		return verifiedEpochDecision{}, errors.New("input log does not match the committed cutoff")
	}
	if epochCommitmentRoot(inputs, emptyInputTag) != epoch.inputRoot {
		return verifiedEpochDecision{}, errors.New("input log root does not match the epoch")
	}
	accepted, rejected := evaluateInputs(config, epoch, inputs)
	if err := verifyViewCommitment(epoch, accepted, rejected); err != nil {
		return verifiedEpochDecision{}, err
	}
	if err := verifyMaterializations(epoch, accepted, materials, requireMaterials); err != nil {
		return verifiedEpochDecision{}, err
	}
	generation := fmt.Sprintf("%x", epoch.digest)
	decision := verifiedEpochDecision{
		epoch:      epoch,
		EpochBytes: append([]byte(nil), epochBytes...),
		Inputs:     cloneInputs(inputs),
		accepted:   accepted,
		rejections: rejected,
		Snapshot: epochVerificationSnapshot{
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
		return verifiedEpochDecision{}, err
	}
	if err := attachDestinationResolutionGateway(&decision); err != nil {
		return verifiedEpochDecision{}, err
	}
	if err := attachTransitIssuanceDuty(&decision); err != nil {
		return verifiedEpochDecision{}, err
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

func evaluateInputs(config epochPolicy, epoch epochEnvelope, inputs [][]byte) ([]nodeRecord, []rejection) {
	evaluated := make([]evaluatedRecord, len(inputs))
	for index, raw := range inputs {
		record, err := parseRecord(raw)
		evaluated[index] = evaluatedRecord{index: index, record: record}
		switch {
		case err != nil:
			evaluated[index].code = rejectMalformed
		case record.networkID != config.NetworkID:
			evaluated[index].code = rejectNetwork
		case !record.signatureValid():
			evaluated[index].code = rejectSignature
		case epoch.validFrom.Before(record.notBefore) || !epoch.validFrom.Before(record.notAfter):
			evaluated[index].code = rejectTime
		case record.capability != requiredCapability(epoch.profile):
			evaluated[index].code = rejectProfile
		case record.capacity == 0 || record.capacity > 1024:
			evaluated[index].code = rejectCapacity
		case !validCarrierProfile(record.carrier):
			evaluated[index].code = rejectCarrier
		case authorityOwnsKey(config, record.keyID):
			evaluated[index].code = rejectSourceCollision
		}
	}
	markCollisions(evaluated)
	accepted := make([]nodeRecord, 0, len(evaluated))
	rejected := make([]rejection, 0, len(evaluated))
	for _, item := range evaluated {
		if item.code != 0 {
			rejected = append(rejected, rejection{index: uint32(item.index), code: item.code, raw: append([]byte(nil), inputs[item.index]...)})
			continue
		}
		accepted = append(accepted, item.record)
	}
	sort.Slice(accepted, func(i, j int) bool {
		return bytes.Compare(accepted[i].nodeID[:], accepted[j].nodeID[:]) < 0
	})
	return accepted, rejected
}

func authorityOwnsKey(config epochPolicy, keyID [32]byte) bool {
	_, exists := config.Authorities[keyID]
	return exists
}

func markCollisions(records []evaluatedRecord) {
	nodes := make(map[[32]byte]int)
	generations := make(map[string]int)
	keys := make(map[[32]byte]int)
	endpoints := make(map[string]int)
	for _, item := range records {
		if item.code != 0 {
			continue
		}
		nodes[item.record.nodeID]++
		generations[generationKey(item.record)]++
		keys[item.record.keyID]++
		endpoints[item.record.endpoint]++
	}
	for index := range records {
		item := &records[index]
		if item.code != 0 {
			continue
		}
		switch {
		case nodes[item.record.nodeID] > 1:
			item.code = rejectNodeCollision
		case generations[generationKey(item.record)] > 1:
			item.code = rejectGeneration
		case keys[item.record.keyID] > 1:
			item.code = rejectKeyCollision
		case endpoints[item.record.endpoint] > 1:
			item.code = rejectEndpoint
		}
	}
}

func generationKey(record nodeRecord) string {
	return string(record.nodeID[:]) + fmt.Sprintf("/%d", record.generation)
}

func verifyViewCommitment(epoch epochEnvelope, accepted []nodeRecord, rejected []rejection) error {
	acceptedBytes := make([][]byte, len(accepted))
	families := make(map[string][2]uint32)
	var capacity uint32
	for index, record := range accepted {
		acceptedBytes[index] = record.raw
		capacity += uint32(record.capacity)
		current := families[record.family]
		current[0]++
		current[1] += uint32(record.capacity)
		families[record.family] = current
	}
	rejectionLeaves := make([][32]byte, len(rejected))
	for index, item := range rejected {
		rejectionLeaves[index] = epochRejectionLeaf(item.index, item.code, item.raw)
	}
	var maxCount, maxCapacity uint32
	for _, summary := range families {
		maxCount = max(maxCount, summary[0])
		maxCapacity = max(maxCapacity, summary[1])
	}
	if uint32(len(accepted)) != epoch.viewLength || uint32(len(rejected)) != epoch.rejectedLength ||
		epochCommitmentRoot(acceptedBytes, emptyViewTag) != epoch.viewRoot ||
		epochHashedCommitmentRoot(rejectionLeaves, emptyRejectionTag) != epoch.rejectedRoot {
		return errors.New("candidate view or rejection commitment is inconsistent")
	}
	if epoch.eligibleCount != uint32(len(accepted)) || epoch.eligibleCapacity != capacity ||
		epoch.familyCount != uint16(len(families)) || epoch.maxFamilyCount != uint16(maxCount) ||
		epoch.maxFamilyCapacity != maxCapacity {
		return errors.New("candidate view summaries are inconsistent")
	}
	if err := verifyDomainSummaries(epoch, families); err != nil {
		return err
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
