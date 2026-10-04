package epoch

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

// InitialClosedEpoch contains the fixed fresh closed State input. Signed Node
// Records remain ordered Source inputs; AuthorityKeys are separate State keys.
type InitialClosedEpoch struct {
	NetworkID, AssignmentSeed [32]byte
	ValidFrom, ValidUntil     time.Time
	Domains                   []string
	Records                   [][]byte
	AuthorityKeys             []ed25519.PublicKey
}

// PrepareInitialClosedEpoch returns unsigned existing AREP-v3 bytes. It checks
// every supplied Node signature and computes the current owner's commitments
// and assignments; it signs nothing and cannot admit State or replace a floor.
func PrepareInitialClosedEpoch(input InitialClosedEpoch) ([]byte, error) {
	if input.NetworkID == [32]byte{} || input.AssignmentSeed == [32]byte{} || !initialPreparationInterval(input.ValidFrom, input.ValidUntil) || len(input.Records) == 0 || len(input.Records) > 64 || len(input.Domains) == 0 || len(input.Domains) > 16 || len(input.AuthorityKeys) == 0 || len(input.AuthorityKeys) > 16 {
		return nil, errors.New("initial closed Epoch facts are invalid")
	}
	policy := Policy{NetworkID: input.NetworkID, Authorities: map[[32]byte]ed25519.PublicKey{}}
	for _, public := range input.AuthorityKeys {
		if len(public) != ed25519.PublicKeySize {
			return nil, errors.New("initial State authority key is invalid")
		}
		id := sha256.Sum256(public)
		if _, exists := policy.Authorities[id]; exists {
			return nil, errors.New("initial State authority keys are duplicated")
		}
		policy.Authorities[id] = public
	}
	value := epochEnvelope{networkID: input.NetworkID, number: 1, validFrom: input.ValidFrom, validUntil: input.ValidUntil, profile: closedRouteProfile, version: 3, cutoff: uint32(len(input.Records)), assignmentSeed: input.AssignmentSeed}
	for index, domain := range input.Domains {
		if index > 0 && input.Domains[index-1] >= domain {
			return nil, errors.New("initial Role Domains are not in canonical order")
		}
		if _, err := appendInitialPreparationText(nil, domain, 32); err != nil {
			return nil, err
		}
		value.domains = append(value.domains, roleDomain{id: domain})
	}
	for _, raw := range input.Records {
		record, err := parseRecord(raw)
		if err != nil || raw[4] != 2 || record.generation != 1 || record.networkID == [32]byte{} || record.nodeID == [32]byte{} || input.ValidUntil.After(record.notAfter) {
			return nil, errors.New("initial Node Record is invalid or ends before its Epoch")
		}
		prepared, err := PrepareInitialClosedRecord(InitialClosedRecord{NetworkID: record.networkID, NodeID: record.nodeID, ValidFrom: record.notBefore, ValidUntil: record.notAfter, Family: record.family, Endpoint: record.endpoint, Carrier: record.carrier, Capacity: record.capacity, PublicKey: record.publicKey})
		if err != nil || !bytes.Equal(prepared, raw[:len(raw)-ed25519.SignatureSize]) {
			return nil, errors.New("initial Node Record does not match the bounded preparation contract")
		}
	}
	accepted, rejected, candidateView, err := evaluateInputs(policy, value, input.Records)
	if err != nil {
		return nil, err
	}
	if len(rejected) != 0 || len(accepted) != len(input.Records) {
		return nil, errors.New("initial Epoch requires individually valid non-colliding signed closed records")
	}
	view := make([][]byte, len(accepted))
	for index, record := range accepted {
		view[index] = record.raw
	}
	value.inputRoot = epochCommitmentRoot(input.Records, emptyInputTag)
	value.viewRoot = epochCommitmentRoot(view, emptyViewTag)
	value.rejectedRoot = epochHashedCommitmentRoot(nil, emptyRejectionTag)
	value.viewLength = uint32(len(accepted))
	value.eligibleCount = value.viewLength
	summary := candidateView.Summary()
	value.eligibleCapacity, value.familyCount = summary.Capacity, summary.FamilyCount
	value.maxFamilyCount, value.maxFamilyCapacity = summary.MaxFamilyCount, summary.MaxFamilyCapacity
	domains, err := candidateDomainSummaries(candidateView, value)
	if err != nil {
		return nil, err
	}
	for index := range value.domains {
		assigned := domains[value.domains[index].id]
		value.domains[index].count, value.domains[index].capacity = uint16(assigned[0]), assigned[1]
	}
	return encodeInitialClosedEpoch(value)
}

func encodeInitialClosedEpoch(value epochEnvelope) ([]byte, error) {
	message := append([]byte("AREP"), 3)
	message = append(message, value.networkID[:]...)
	message = binary.BigEndian.AppendUint64(message, 1)
	message = append(message, make([]byte, 32)...)
	message = binary.BigEndian.AppendUint64(message, uint64(value.validFrom.Unix()))
	message = binary.BigEndian.AppendUint64(message, uint64(value.validUntil.Unix()))
	message = binary.BigEndian.AppendUint32(message, value.cutoff)
	message, err := appendInitialPreparationText(message, value.profile, 64)
	if err != nil {
		return nil, err
	}
	message = append(message, value.inputRoot[:]...)
	message = append(message, value.viewRoot[:]...)
	message = binary.BigEndian.AppendUint32(message, value.viewLength)
	message = append(message, value.rejectedRoot[:]...)
	message = binary.BigEndian.AppendUint32(message, 0)
	message = append(message, value.assignmentSeed[:]...)
	message, err = appendInitialPreparationText(message, assignmentV1, 64)
	if err != nil {
		return nil, err
	}
	message = binary.BigEndian.AppendUint32(message, value.eligibleCount)
	message = binary.BigEndian.AppendUint32(message, value.eligibleCapacity)
	message = binary.BigEndian.AppendUint16(message, value.familyCount)
	message = binary.BigEndian.AppendUint16(message, value.maxFamilyCount)
	message = binary.BigEndian.AppendUint32(message, value.maxFamilyCapacity)
	message = append(message, byte(len(value.domains)))
	for _, domain := range value.domains {
		message, err = appendInitialPreparationText(message, domain.id, 32)
		if err != nil {
			return nil, err
		}
		message = binary.BigEndian.AppendUint16(message, domain.count)
		message = binary.BigEndian.AppendUint32(message, domain.capacity)
	}
	// Closed v3 does not carry the retired interactive Gateway/Transit profiles.
	message = append(message, make([]byte, 68)...)
	return message, nil
}
