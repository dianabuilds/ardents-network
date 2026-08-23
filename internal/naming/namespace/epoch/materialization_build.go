package epoch

import (
	"bytes"
	"errors"
	"sort"
)

const (
	emptyRecordTag  byte = 0x61
	emptyLineageTag byte = 0x62
)

type recordEntry struct {
	record Record
	signed []byte
}

func materializeRecords(network [32]byte, signed [][]byte) ([][]byte, [][]byte, error) {
	if len(signed) == 0 || len(signed) > maximumRecords {
		return nil, nil, errors.New("naming materialization Record corpus is invalid")
	}
	entries := make([]recordEntry, len(signed))
	for index, raw := range signed {
		record, err := VerifyRecord(network, raw)
		if err != nil {
			return nil, nil, errors.New("naming materialization contains an invalid signed Record")
		}
		entries[index] = recordEntry{record: record, signed: append([]byte(nil), raw...)}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].record.Name < entries[j].record.Name })
	byName := make(map[string]int, len(entries))
	for index, entry := range entries {
		if index > 0 && entries[index-1].record.Name == entry.record.Name {
			return nil, nil, errors.New("naming materialization contains duplicate Names")
		}
		byName[entry.record.Name] = index
	}
	records, leaves := make([][]byte, len(entries)), make([][]byte, len(entries))
	for index, entry := range entries {
		leaf, err := materializeLeaf(index, entries, byName)
		if err != nil {
			return nil, nil, err
		}
		records[index] = entry.signed
		leaves[index] = encodeLeaf(leaf)
	}
	return records, leaves, nil
}

func materializeLeaf(index int, entries []recordEntry, byName map[string]int) (resolutionLeaf, error) {
	head := entries[index]
	lineage, current := [][]byte{}, head.record
	state, notAfter, available := effectiveLease(current)
	seen := map[string]bool{current.Name: true}
	for depth := 0; current.ParentName != ""; depth++ {
		if depth >= 126 {
			return resolutionLeaf{}, errors.New("naming materialization lineage exceeds its bound")
		}
		parentIndex, ok := byName[current.ParentName]
		if !ok || seen[current.ParentName] || entries[parentIndex].record.Generation != current.ParentGeneration {
			return resolutionLeaf{}, errors.New("naming materialization lineage is discontinuous")
		}
		seen[current.ParentName] = true
		parent := entries[parentIndex]
		lineage = append(lineage, parent.signed)
		parentState, parentNotAfter, parentAvailable := effectiveLease(parent.record)
		available = available && parentAvailable
		if parentState == 2 {
			state = 2
		}
		if notAfter == 0 || parentNotAfter < notAfter {
			notAfter = parentNotAfter
		}
		current = parent.record
	}
	if current.ParentName != "" || head.record.Target == [32]byte{} {
		available = false
	}
	if !available {
		state, notAfter = 0, 0
	}
	return resolutionLeaf{schema: leafSchema, signedRecord: head.signed, lineageRoot: namespaceCommitmentRoot(lineage, emptyLineageTag),
		lineageCount: uint8(len(lineage)), state: state, notAfter: notAfter}, nil
}

func effectiveLease(record Record) (byte, int64, bool) {
	if record.Consistency != "current" || record.Recovery != "stable" {
		return 0, 0, false
	}
	switch record.Lease {
	case "active":
		if record.LeaseExpiresAt <= 0 {
			return 0, 0, false
		}
		return effectiveRecordNotAfter(record, 1, record.LeaseExpiresAt*1_000)
	case "grace":
		if record.GraceExpiresAt <= 0 {
			return 0, 0, false
		}
		return effectiveRecordNotAfter(record, 2, record.GraceExpiresAt*1_000)
	default:
		return 0, 0, false
	}
}

func effectiveRecordNotAfter(record Record, state byte, leaseNotAfter int64) (byte, int64, bool) {
	if record.Target == [32]byte{} {
		return state, leaseNotAfter, true
	}
	if record.RecordNotAfter <= 0 {
		return 0, 0, false
	}
	return state, min(leaseNotAfter, record.RecordNotAfter), true
}

func recordRoot(leaves [][]byte) [32]byte { return namespaceCommitmentRoot(leaves, emptyRecordTag) }

func sameInputs(left, right [][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !bytes.Equal(left[index], right[index]) {
			return false
		}
	}
	return true
}

func sameRecord(left, right Record) bool {
	leftWire, leftErr := EncodeRecord(left)
	rightWire, rightErr := EncodeRecord(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftWire, rightWire)
}
