package stage6verify

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"strings"
)

func verifyDeepNamespaceMeasurement(evidence resolutionCellEvidence) bool {
	if evidence.DeepName != strings.Repeat("a.", 126)+"a" || len(evidence.DeepName) != 253 ||
		evidence.DeepProofBytes != uint32(len(evidence.DeepNamespaceProof)) ||
		evidence.DeepProofBytes == 0 || evidence.DeepProofBytes > 4096 ||
		len(evidence.DeepNamespaceRecords) != 127 {
		return false
	}
	statement, ordinal, leaf, path, err := decodeNamespaceProof(evidence.DeepNamespaceProof)
	if err != nil || statement.network != [32]byte{9} || statement.epoch != 1 ||
		statement.epochDigest != [32]byte{1} || statement.cutoff != 10_000 ||
		statement.rule != "ardents-namespace-materialization-v1" || statement.records != 127 ||
		ordinal != 126 || statement.transitions != 127 || statement.rejections != 0 ||
		statement.transition != namespaceRawRoot(evidence.DeepNamespaceRecords, 0x63) ||
		statement.rejected != sha256.Sum256([]byte("ardents-stage6-deep-no-rejections")) {
		return false
	}
	if !verifyDeepNamespaceSignatures(evidence, statement) {
		return false
	}
	if !verifyNamespaceInclusion(leaf, ordinal, statement.records, path, statement.recordRoot) {
		return false
	}
	if !verifyNamespaceRecordCorpus(evidence.DeepNamespaceRecords, statement, ordinal, leaf) {
		return false
	}
	cursor := namespaceCursor{raw: leaf}
	schema, schemaErr := cursor.u16()
	signedSize, sizeErr := cursor.u32()
	signed, signedErr := cursor.take(int(signedSize))
	_, rootErr := cursor.a32()
	lineage, lineageErr := cursor.u8()
	state, stateErr := cursor.u8()
	notAfter, timeErr := cursor.u64()
	record, recordErr := verifySignedNamespaceRecord(signed, statement.network)
	return schemaErr == nil && sizeErr == nil && signedErr == nil && rootErr == nil && lineageErr == nil &&
		stateErr == nil && timeErr == nil && recordErr == nil && cursor.done() && schema == namespaceLeafSchema && lineage == 126 &&
		state == 1 && notAfter == 1_800_001_800_000 && record.Name == evidence.DeepName && record.Generation == 1 &&
		record.Revision == 1 && record.Target == [32]byte{1} && record.RecordNotAfter == 1_800_001_800_000
}

func verifyDeepNamespaceSignatures(evidence resolutionCellEvidence, statement namespaceStatement) bool {
	if evidence.EpochThreshold != 2 || len(evidence.EpochSignerIDs) != 3 ||
		len(evidence.EpochPublicKeys) != len(evidence.EpochSignerIDs) ||
		len(statement.signerIDs) < int(evidence.EpochThreshold) ||
		len(statement.signerIDs) != len(statement.signatures) {
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
	for index, id := range statement.signerIDs {
		public, ok := authorities[id]
		if !ok || index > 0 && bytes.Compare(statement.signerIDs[index-1][:], id[:]) >= 0 ||
			len(statement.signatures[index]) != ed25519.SignatureSize ||
			!ed25519.Verify(public, statement.raw, statement.signatures[index]) {
			return false
		}
	}
	return true
}
