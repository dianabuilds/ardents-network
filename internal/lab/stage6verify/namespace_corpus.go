package stage6verify

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

type namespaceRecordEntry struct {
	record decodedRecord
	signed []byte
}

func verifyNamespaceRecordCorpus(signed [][]byte, statement namespaceStatement, ordinal uint32, proofLeaf []byte) bool {
	if len(signed) == 0 || uint32(len(signed)) != statement.records || ordinal >= statement.records {
		return false
	}
	entries := make([]namespaceRecordEntry, len(signed))
	byName := make(map[string]int, len(signed))
	for index, raw := range signed {
		record, err := verifySignedNamespaceRecord(raw, statement.network)
		if err != nil || index > 0 && entries[index-1].record.Name >= record.Name {
			return false
		}
		entries[index] = namespaceRecordEntry{record: record, signed: raw}
		byName[record.Name] = index
	}
	leaves := make([][]byte, len(entries))
	for index := range entries {
		leaf, ok := namespaceCorpusLeaf(index, entries, byName)
		if !ok {
			return false
		}
		leaves[index] = leaf
	}
	if !bytes.Equal(leaves[ordinal], proofLeaf) {
		return false
	}
	if namespaceRawRoot(leaves, 0x61) != statement.recordRoot {
		return false
	}
	return true
}

func namespaceCorpusLeaf(index int, entries []namespaceRecordEntry, byName map[string]int) ([]byte, bool) {
	head := entries[index]
	lineage, current := [][]byte{}, head.record
	state, notAfter, available := namespaceLease(current)
	seen := map[string]bool{current.Name: true}
	for depth := 0; current.Parent != ""; depth++ {
		parentIndex, ok := byName[current.Parent]
		if !ok || depth >= 126 || seen[current.Parent] ||
			entries[parentIndex].record.Generation != current.ParentGeneration {
			return nil, false
		}
		seen[current.Parent] = true
		parent := entries[parentIndex]
		lineage = append(lineage, parent.signed)
		parentState, parentNotAfter, parentAvailable := namespaceLease(parent.record)
		available = available && parentAvailable
		if parentState == 2 {
			state = 2
		}
		if notAfter == 0 || parentNotAfter < notAfter {
			notAfter = parentNotAfter
		}
		current = parent.record
	}
	if head.record.Target == [32]byte{} || !available {
		state, notAfter = 0, 0
	}
	out := binary.BigEndian.AppendUint16(nil, namespaceLeafSchema)
	out = binary.BigEndian.AppendUint32(out, uint32(len(head.signed)))
	out = append(out, head.signed...)
	root := namespaceRawRoot(lineage, 0x62)
	out = append(out, root[:]...)
	out = append(out, byte(len(lineage)), state)
	return binary.BigEndian.AppendUint64(out, notAfter), true
}

func namespaceLease(record decodedRecord) (byte, uint64, bool) {
	if record.Consistency != "current" || record.Recovery != "stable" {
		return 0, 0, false
	}
	limit := int64(0)
	if record.Target != [32]byte{} {
		if record.RecordNotAfter <= 0 {
			return 0, 0, false
		}
		limit = record.RecordNotAfter
	}
	switch record.Lease {
	case "active":
		if record.LeaseExpires <= 0 {
			return 0, 0, false
		}
		return 1, namespaceNotAfter(record.LeaseExpires*1_000, limit), true
	case "grace":
		if record.GraceExpires <= 0 {
			return 0, 0, false
		}
		return 2, namespaceNotAfter(record.GraceExpires*1_000, limit), true
	default:
		return 0, 0, false
	}
}

func namespaceNotAfter(lease, record int64) uint64 {
	if record > 0 && record < lease {
		return uint64(record)
	}
	return uint64(lease)
}

func namespaceRawRoot(values [][]byte, emptyTag byte) [32]byte {
	if len(values) == 0 {
		return sha256.Sum256([]byte{emptyTag})
	}
	if len(values) == 1 {
		return namespaceRawLeaf(values[0])
	}
	split := 1
	for split<<1 < len(values) {
		split <<= 1
	}
	return namespaceBranch(namespaceRawRoot(values[:split], emptyTag), namespaceRawRoot(values[split:], emptyTag))
}

func namespaceRawLeaf(value []byte) [32]byte {
	out := make([]byte, 5+len(value))
	binary.BigEndian.PutUint32(out[1:5], uint32(len(value)))
	copy(out[5:], value)
	return sha256.Sum256(out)
}
