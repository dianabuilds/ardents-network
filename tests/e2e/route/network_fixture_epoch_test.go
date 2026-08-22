package route_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"sort"

	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
	"github.com/dianabuilds/ardents-network/tests/epochfixture/merkle"
)

type domainSummary struct {
	id       string
	count    uint16
	capacity uint32
}

// BuildEpoch returns one canonical signed Epoch and every accepted proof.
func BuildEpoch(spec EpochSpec) (Epoch, error) {
	if err := validateEpochSpec(spec); err != nil {
		return Epoch{}, err
	}
	accepted := append([]Record(nil), spec.Accepted...)
	sort.Slice(accepted, func(i, j int) bool { return bytes.Compare(accepted[i].NodeID[:], accepted[j].NodeID[:]) < 0 })
	view := make([][]byte, len(accepted))
	for index := range accepted {
		view[index] = accepted[index].Raw
	}
	rejectionLeaves := make([][32]byte, 0, len(spec.Rejections))
	indices := make([]int, 0, len(spec.Rejections))
	for index := range spec.Rejections {
		indices = append(indices, int(index))
	}
	sort.Ints(indices)
	for _, index := range indices {
		if index < 0 || index >= len(spec.Inputs) {
			return Epoch{}, errors.New("rejection index is outside inputs")
		}
		rejectionLeaves = append(rejectionLeaves, merkle.RejectionLeaf(uint32(index), spec.Rejections[uint32(index)], spec.Inputs[index]))
	}
	summaries, families, capacity, maxFamilyCount, maxFamilyCapacity, err := summarize(spec, accepted)
	if err != nil {
		return Epoch{}, err
	}
	buffer := encodeEpochCommitment(spec, view, rejectionLeaves, summaries, families, capacity, maxFamilyCount, maxFamilyCapacity)
	digest := sha256.Sum256(buffer.Bytes())
	raw, err := signEpoch(buffer.Bytes(), digest, spec.Authorities)
	if err != nil {
		return Epoch{}, err
	}
	materials := make([][]byte, len(view))
	for index, record := range view {
		materials[index] = material(digest, uint32(index), record, merkle.Proof(view, index, 0x11))
	}
	return Epoch{Number: spec.Number, Seed: spec.AssignmentSeed, Raw: raw, Digest: digest,
		Inputs: cloneBytes(spec.Inputs), Materials: materials}, nil
}

func validateEpochSpec(spec EpochSpec) error {
	profile := spec.Profile
	if profile == "" {
		profile = "h3-role-probe-v1"
	}
	if spec.Number == 0 || spec.ValidFrom.IsZero() || !spec.ValidUntil.After(spec.ValidFrom) ||
		len(spec.Inputs) > 64 || len(spec.Accepted) > 64 || len(spec.Rejections) > 64 ||
		len(spec.Domains) == 0 || len(spec.Domains) > 16 || len(spec.Authorities) == 0 || len(spec.Authorities) > 16 ||
		profile != "h3-role-probe-v1" && profile != "h3-route-tracer-v1" {
		return errors.New("epoch fixture specification is invalid")
	}
	for _, domain := range spec.Domains {
		if domain == "" || len(domain) > 32 {
			return errors.New("epoch fixture domain is invalid")
		}
	}
	for _, private := range spec.Authorities {
		if len(private) != ed25519.PrivateKeySize {
			return errors.New("epoch fixture authority is invalid")
		}
	}
	return nil
}

func summarize(spec EpochSpec, records []Record) ([]domainSummary, uint16, uint32, uint16, uint32, error) {
	values := make([]domainSummary, len(spec.Domains))
	for index, domain := range spec.Domains {
		values[index].id = domain
	}
	families := make(map[string][2]uint32)
	var capacity uint32
	for _, record := range records {
		selected, err := assignment.Select(spec.NetworkID, spec.Number, spec.AssignmentSeed, record.Family, spec.Domains)
		if err != nil {
			return nil, 0, 0, 0, 0, err
		}
		for index := range values {
			if values[index].id == selected {
				values[index].count++
				values[index].capacity += uint32(record.Capacity)
			}
		}
		family := families[record.Family]
		family[0]++
		family[1] += uint32(record.Capacity)
		families[record.Family] = family
		capacity += uint32(record.Capacity)
	}
	var maxCount, maxCapacity uint32
	for _, family := range families {
		maxCount = max(maxCount, family[0])
		maxCapacity = max(maxCapacity, family[1])
	}
	return values, uint16(len(families)), capacity, uint16(maxCount), maxCapacity, nil
}
