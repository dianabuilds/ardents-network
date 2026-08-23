package stage6verify

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

type namespaceStatement struct {
	raw                                                    []byte
	network, epochDigest, recordRoot, transition, rejected [32]byte
	epoch, cutoff                                          uint64
	rule                                                   string
	records, transitions, rejections                       uint32
	signerIDs                                              [][32]byte
	signatures                                             [][]byte
}

const namespaceLeafSchema uint16 = 2

func verifyNamespaceMaterialization(evidence resolutionCellEvidence) bool {
	if evidence.EpochThreshold != 2 || len(evidence.EpochSignerIDs) != 3 ||
		len(evidence.EpochPublicKeys) != len(evidence.EpochSignerIDs) {
		return false
	}
	authorities := make(map[[32]byte]ed25519.PublicKey, len(evidence.EpochSignerIDs))
	for index, id := range evidence.EpochSignerIDs {
		if index > 0 && bytes.Compare(evidence.EpochSignerIDs[index-1][:], id[:]) >= 0 ||
			sha256.Sum256(evidence.EpochPublicKeys[index][:]) != id {
			return false
		}
		authorities[id] = append(ed25519.PublicKey(nil), evidence.EpochPublicKeys[index][:]...)
	}
	statement, ordinal, leaf, path, err := decodeNamespaceProof(evidence.NamespaceProof)
	if err != nil || statement.network != [32]byte{9} || statement.epoch != 1 || statement.epochDigest != [32]byte{1} ||
		statement.cutoff != 10_000 ||
		statement.rule != "ardents-namespace-materialization-v1" || statement.records != 1 || ordinal != 0 ||
		statement.transitions != 2 || statement.rejections != 1 ||
		len(statement.signerIDs) < int(evidence.EpochThreshold) || len(statement.signerIDs) != len(statement.signatures) {
		return false
	}
	for index, id := range statement.signerIDs {
		public, ok := authorities[id]
		if !ok || index > 0 && bytes.Compare(statement.signerIDs[index-1][:], id[:]) >= 0 ||
			len(statement.signatures[index]) != ed25519.SignatureSize ||
			!ed25519.Verify(public, statement.raw, statement.signatures[index]) {
			return false
		}
	}
	if !verifyNamespaceInclusion(leaf, ordinal, statement.records, path, statement.recordRoot) {
		return false
	}
	if !verifyNamespaceRecordCorpus(evidence.NamespaceRecords, statement, ordinal, leaf) {
		return false
	}
	if !verifyNamespaceTransitionCorpus(evidence, statement) {
		return false
	}
	record, state, notAfter, err := verifyNamespaceLeaf(leaf, statement.network)
	return err == nil && record.Name == "alice" && record.Generation == 1 && record.Revision == 2 &&
		record.Target == [32]byte{1} && record.RecordNotAfter == 1_800_001_800_000 &&
		state == 1 && notAfter == 1_800_001_800_000
}

func decodeNamespaceProof(raw []byte) (namespaceStatement, uint32, []byte, [][32]byte, error) {
	cursor := namespaceCursor{raw: raw}
	schema, schemaErr := cursor.u16()
	attestedSize, sizeErr := cursor.u32()
	attestedRaw, attestedErr := cursor.take(int(attestedSize))
	statement, statementErr := decodeNamespaceStatement(attestedRaw)
	ordinal, ordinalErr := cursor.u32()
	leafSize, leafSizeErr := cursor.u32()
	leaf, leafErr := cursor.take(int(leafSize))
	count, countErr := cursor.u8()
	if schemaErr != nil || sizeErr != nil || attestedErr != nil || statementErr != nil || ordinalErr != nil ||
		leafSizeErr != nil || leafErr != nil || countErr != nil || schema != 1 || count > 32 {
		return namespaceStatement{}, 0, nil, nil, errors.New("namespace proof is malformed")
	}
	path := make([][32]byte, count)
	for index := range path {
		value, err := cursor.a32()
		if err != nil {
			return namespaceStatement{}, 0, nil, nil, err
		}
		path[index] = value
	}
	if !cursor.done() {
		return namespaceStatement{}, 0, nil, nil, errors.New("namespace proof has trailing bytes")
	}
	return statement, ordinal, append([]byte(nil), leaf...), path, nil
}

func decodeNamespaceStatement(raw []byte) (namespaceStatement, error) {
	cursor := namespaceCursor{raw: raw}
	schema, schemaErr := cursor.u16()
	transcriptSize, sizeErr := cursor.u32()
	transcript, transcriptErr := cursor.take(int(transcriptSize))
	statement, statementErr := decodeNamespaceTranscript(transcript)
	count, countErr := cursor.u8()
	if schemaErr != nil || sizeErr != nil || transcriptErr != nil || statementErr != nil || countErr != nil ||
		schema != 1 || count == 0 || count > 16 {
		return namespaceStatement{}, errors.New("namespace statement is malformed")
	}
	statement.signerIDs, statement.signatures = make([][32]byte, count), make([][]byte, count)
	for index := range statement.signerIDs {
		id, idErr := cursor.a32()
		signature, signatureErr := cursor.take(ed25519.SignatureSize)
		if idErr != nil || signatureErr != nil {
			return namespaceStatement{}, errors.New("namespace signature is truncated")
		}
		statement.signerIDs[index], statement.signatures[index] = id, append([]byte(nil), signature...)
	}
	if !cursor.done() {
		return namespaceStatement{}, errors.New("namespace statement has trailing bytes")
	}
	statement.raw = append([]byte(nil), transcript...)
	return statement, nil
}

func decodeNamespaceTranscript(raw []byte) (namespaceStatement, error) {
	cursor := namespaceCursor{raw: raw}
	domain, domainErr := cursor.text()
	network, networkErr := cursor.a32()
	epoch, epochErr := cursor.u64()
	epochDigest, epochDigestErr := cursor.a32()
	rule, ruleErr := cursor.text()
	cutoff, cutoffErr := cursor.u64()
	recordRoot, recordRootErr := cursor.a32()
	records, recordsErr := cursor.u32()
	transition, transitionErr := cursor.a32()
	transitions, transitionsErr := cursor.u32()
	rejected, rejectedErr := cursor.a32()
	rejections, rejectionsErr := cursor.u32()
	if domainErr != nil || networkErr != nil || epochErr != nil || epochDigestErr != nil || ruleErr != nil || cutoffErr != nil ||
		recordRootErr != nil || recordsErr != nil || transitionErr != nil || transitionsErr != nil ||
		rejectedErr != nil || rejectionsErr != nil || !cursor.done() ||
		domain != "ardents-namespace-epoch-materialization-v1" {
		return namespaceStatement{}, errors.New("namespace transcript is invalid")
	}
	return namespaceStatement{network: network, epoch: epoch, epochDigest: epochDigest, rule: rule, cutoff: cutoff,
		recordRoot: recordRoot, records: records, transition: transition, transitions: transitions,
		rejected: rejected, rejections: rejections}, nil
}

func verifyNamespaceLeaf(raw []byte, network [32]byte) (decodedRecord, byte, uint64, error) {
	cursor := namespaceCursor{raw: raw}
	schema, schemaErr := cursor.u16()
	signedSize, sizeErr := cursor.u32()
	signed, signedErr := cursor.take(int(signedSize))
	lineageRoot, rootErr := cursor.a32()
	lineageCount, countErr := cursor.u8()
	state, stateErr := cursor.u8()
	notAfter, timeErr := cursor.u64()
	if schemaErr != nil || sizeErr != nil || signedErr != nil || rootErr != nil || countErr != nil ||
		stateErr != nil || timeErr != nil || !cursor.done() || schema != namespaceLeafSchema || lineageCount != 0 ||
		lineageRoot != sha256.Sum256([]byte{0x62}) {
		return decodedRecord{}, 0, 0, errors.New("namespace leaf is invalid")
	}
	record, err := verifySignedNamespaceRecord(signed, network)
	return record, state, notAfter, err
}

func verifySignedNamespaceRecord(raw []byte, network [32]byte) (decodedRecord, error) {
	if len(raw) < 74 || binary.BigEndian.Uint16(raw) != 1 {
		return decodedRecord{}, errors.New("signed Record is malformed")
	}
	size := binary.BigEndian.Uint64(raw[2:10])
	if size == 0 || size != uint64(len(raw)-10-ed25519.SignatureSize) {
		return decodedRecord{}, errors.New("signed Record length is invalid")
	}
	recordWire, signature := raw[10:10+size], raw[10+size:]
	record, err := decodeRecord(recordWire)
	public, publicErr := decodeFixedHex(record.Authority, ed25519.PublicKeySize)
	transcript := appendText16(nil, "ardents-name-record-v1")
	transcript = append(transcript, network[:]...)
	transcript = binary.BigEndian.AppendUint64(transcript, uint64(len(recordWire)))
	transcript = append(transcript, recordWire...)
	if err != nil || publicErr != nil || !ed25519.Verify(ed25519.PublicKey(public), transcript, signature) {
		return decodedRecord{}, errors.New("signed Record is invalid")
	}
	return record, nil
}

func verifyNamespaceInclusion(record []byte, index, length uint32, path [][32]byte, expected [32]byte) bool {
	if length == 0 || index >= length {
		return false
	}
	encoded := append([]byte{0}, binary.BigEndian.AppendUint32(nil, uint32(len(record)))...)
	current, leaf, last := sha256.Sum256(append(encoded, record...)), index, length-1
	for _, sibling := range path {
		if leaf&1 == 1 || leaf == last {
			current = namespaceBranch(sibling, current)
			for leaf&1 == 0 && leaf != 0 {
				leaf, last = leaf>>1, last>>1
			}
		} else {
			current = namespaceBranch(current, sibling)
		}
		leaf, last = leaf>>1, last>>1
	}
	return last == 0 && current == expected
}

func namespaceBranch(left, right [32]byte) [32]byte {
	return sha256.Sum256(append(append([]byte{1}, left[:]...), right[:]...))
}
