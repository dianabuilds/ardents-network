package namestore

import (
	"crypto/sha256"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/naming"
)

const (
	proofSchema       uint16 = 1
	maximumProofBytes int    = 4096
)

// Lookup returns one compact threshold-authenticated membership proof for the
// exact current Service Name.
func (store *Store) Lookup(rawName string, minimumEpoch uint64) ([]byte, error) {
	name, err := naming.Parse(rawName)
	if err != nil || string(name) != rawName {
		return nil, errors.New("naming proof name is invalid")
	}
	current, err := store.load(minimumEpoch)
	if err != nil {
		return nil, err
	}
	index := -1
	for candidate, raw := range current.records {
		record, verifyErr := nameauthority.VerifyRecord(store.policy.Network, raw)
		if verifyErr != nil {
			return nil, errors.New("naming state is tampered")
		}
		if record.Name == rawName {
			index = candidate
			break
		}
	}
	if index < 0 {
		return nil, errors.New("name is unavailable")
	}
	proof := encodeProof(current.attested, uint32(index), current.leaves[index],
		namespaceProof(current.leaves, index, emptyRecordTag))
	if len(proof) > maximumProofBytes {
		return nil, errors.New("naming proof exceeds the fixed response bound")
	}
	return proof, nil
}

// Verify authenticates one current Namespace proof and returns the exact
// immutable binding asserted by its threshold-signed materialization.
func Verify(input Policy, proof []byte, minimumEpoch uint64, expectedEpochDigest [32]byte, at int64) (
	namelease.Record, namelease.Binding, string, uint64, error,
) {
	policy, err := validPolicy(input)
	if err != nil || at < 0 || expectedEpochDigest == [32]byte{} {
		return namelease.Record{}, namelease.Binding{}, "", 0, errors.New("naming proof policy is invalid")
	}
	attested, ordinal, leafRaw, siblings, err := decodeProof(proof)
	statement := attested.statement
	if err != nil || statement.epoch < minimumEpoch || statement.epochDigest != expectedEpochDigest ||
		!verifyAttestation(policy, attested) ||
		!verifyNamespaceProof(leafRaw, ordinal, statement.recordLength, siblings, statement.recordRoot) {
		return namelease.Record{}, namelease.Binding{}, "", 0, errors.New("naming proof is invalid or stale")
	}
	leaf, err := decodeLeaf(leafRaw)
	if err != nil || leaf.state == 0 || at > leaf.notAfter {
		return namelease.Record{}, namelease.Binding{}, "", 0, errors.New("name is unavailable")
	}
	record, err := nameauthority.VerifyRecord(policy.Network, leaf.signedRecord)
	if err != nil || record.Target == [32]byte{} {
		return namelease.Record{}, namelease.Binding{}, "", 0, errors.New("naming proof Record is invalid")
	}
	recordWire, err := namelease.EncodeRecord(record)
	if err != nil {
		return namelease.Record{}, namelease.Binding{}, "", 0, err
	}
	recordDigest, leafDigest := sha256.Sum256(recordWire), sha256.Sum256(leafRaw)
	commitment := sha256.Sum256(append([]byte("ardents-h3-name-materialized-binding-v1\x00"), leafDigest[:]...))
	binding := namelease.Binding{Name: record.Name, Generation: record.Generation, Revision: record.Revision,
		Authority: record.Authority, Target: record.Target, ParentName: record.ParentName,
		ParentGeneration: record.ParentGeneration, RecordDigest: recordDigest, Commitment: commitment}
	warning := ""
	if leaf.state == 2 {
		warning = "name lineage is in grace and should be treated as volatile"
	}
	return record, binding, warning, statement.epoch, nil
}

func encodeProof(attested attestedStatement, ordinal uint32, leaf []byte, siblings [][32]byte) []byte {
	statementBytes := encodeAttested(attested)
	out := appendUint16(nil, proofSchema)
	out = appendUint32(out, uint32(len(statementBytes)))
	out = append(out, statementBytes...)
	out = appendUint32(out, ordinal)
	out = appendUint32(out, uint32(len(leaf)))
	out = append(out, leaf...)
	out = append(out, byte(len(siblings)))
	for _, sibling := range siblings {
		out = append(out, sibling[:]...)
	}
	return out
}

func decodeProof(raw []byte) (attestedStatement, uint32, []byte, [][32]byte, error) {
	if len(raw) == 0 || len(raw) > maximumProofBytes {
		return attestedStatement{}, 0, nil, nil, errors.New("naming proof size is invalid")
	}
	cursor := byteCursor{raw: raw}
	schema, schemaErr := cursor.uint16()
	statementSize, sizeErr := cursor.uint32()
	statementRaw, statementErr := cursor.bytes(int(statementSize))
	attested, attestedErr := decodeAttested(statementRaw)
	ordinal, ordinalErr := cursor.uint32()
	leafSize, leafSizeErr := cursor.uint32()
	leaf, leafErr := cursor.bytes(int(leafSize))
	count, countErr := cursor.byte()
	if schemaErr != nil || sizeErr != nil || statementErr != nil || attestedErr != nil || ordinalErr != nil ||
		leafSizeErr != nil || leafErr != nil || countErr != nil || schema != proofSchema || count > 32 {
		return attestedStatement{}, 0, nil, nil, errors.New("naming proof is malformed")
	}
	siblings := make([][32]byte, count)
	for index := range siblings {
		value, err := cursor.bytes(32)
		if err != nil {
			return attestedStatement{}, 0, nil, nil, errors.New("naming proof path is truncated")
		}
		copy(siblings[index][:], value)
	}
	if !cursor.done() || string(encodeProof(attested, ordinal, leaf, siblings)) != string(raw) {
		return attestedStatement{}, 0, nil, nil, errors.New("naming proof is non-canonical")
	}
	return attested, ordinal, append([]byte(nil), leaf...), siblings, nil
}
