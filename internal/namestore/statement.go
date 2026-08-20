package namestore

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
)

const (
	attestedSchema   uint16 = 1
	maximumRuleBytes int    = 64
)

func statementTranscript(value statement) []byte {
	out := appendText(nil, "ardents-namespace-epoch-materialization-v1")
	out = append(out, value.network[:]...)
	out = appendUint64(out, value.epoch)
	out = append(out, value.epochDigest[:]...)
	out = appendText(out, value.rule)
	out = appendUint64(out, uint64(value.cutoff))
	out = append(out, value.recordRoot[:]...)
	out = appendUint32(out, value.recordLength)
	out = append(out, value.transitionRoot[:]...)
	out = appendUint32(out, value.transitionLength)
	out = append(out, value.rejectionRoot[:]...)
	return appendUint32(out, value.rejectionLength)
}

func verifyAttestation(policy Policy, value attestedStatement) bool {
	statement := value.statement
	if statement.network != policy.Network || statement.rule != policy.Rule || statement.epoch == 0 ||
		statement.epochDigest == [32]byte{} ||
		statement.cutoff < 0 || statement.recordRoot == [32]byte{} || statement.recordLength == 0 ||
		statement.recordLength > maximumRecords || statement.transitionRoot == [32]byte{} || statement.rejectionRoot == [32]byte{} ||
		len(value.signerIDs) < policy.Threshold || len(value.signerIDs) != len(value.signatures) ||
		len(value.signerIDs) > len(policy.Authorities) {
		return false
	}
	transcript := statementTranscript(statement)
	for index, id := range value.signerIDs {
		if index > 0 && bytes.Compare(value.signerIDs[index-1][:], id[:]) >= 0 {
			return false
		}
		public, ok := policy.Authorities[id]
		if !ok || sha256.Sum256(public) != id || len(value.signatures[index]) != ed25519.SignatureSize ||
			!ed25519.Verify(public, transcript, value.signatures[index]) {
			return false
		}
	}
	return true
}

func encodeAttested(value attestedStatement) []byte {
	transcript := statementTranscript(value.statement)
	out := appendUint16(nil, attestedSchema)
	out = appendUint32(out, uint32(len(transcript)))
	out = append(out, transcript...)
	out = append(out, byte(len(value.signerIDs)))
	for index, id := range value.signerIDs {
		out = append(out, id[:]...)
		out = append(out, value.signatures[index]...)
	}
	return out
}

func decodeAttested(raw []byte) (attestedStatement, error) {
	cursor := byteCursor{raw: raw}
	schema, err := cursor.uint16()
	size, sizeErr := cursor.uint32()
	if err != nil || sizeErr != nil || schema != attestedSchema || size == 0 || int(size) > cursor.remaining() {
		return attestedStatement{}, errors.New("naming materialization statement is malformed")
	}
	transcript, _ := cursor.bytes(int(size))
	statement, err := decodeStatement(transcript)
	count, countErr := cursor.byte()
	if err != nil || countErr != nil || count == 0 || count > 16 {
		return attestedStatement{}, errors.New("naming materialization signatures are malformed")
	}
	value := attestedStatement{statement: statement, signerIDs: make([][32]byte, count), signatures: make([][]byte, count)}
	for index := range value.signerIDs {
		id, idErr := cursor.bytes(32)
		signature, signatureErr := cursor.bytes(ed25519.SignatureSize)
		if idErr != nil || signatureErr != nil {
			return attestedStatement{}, errors.New("naming materialization signature is truncated")
		}
		copy(value.signerIDs[index][:], id)
		value.signatures[index] = append([]byte(nil), signature...)
	}
	if !cursor.done() || !bytes.Equal(encodeAttested(value), raw) {
		return attestedStatement{}, errors.New("naming materialization statement is non-canonical")
	}
	return value, nil
}

func decodeStatement(raw []byte) (statement, error) {
	cursor := byteCursor{raw: raw}
	domain, err := cursor.text(maximumRuleBytes)
	network, networkErr := cursor.array32()
	epoch, epochErr := cursor.uint64()
	epochDigest, epochDigestErr := cursor.array32()
	rule, ruleErr := cursor.text(maximumRuleBytes)
	cutoff, cutoffErr := cursor.uint64()
	recordRoot, recordRootErr := cursor.array32()
	recordLength, recordLengthErr := cursor.uint32()
	transitionRoot, transitionRootErr := cursor.array32()
	transitionLength, transitionLengthErr := cursor.uint32()
	rejectionRoot, rejectionRootErr := cursor.array32()
	rejectionLength, rejectionLengthErr := cursor.uint32()
	if err != nil || networkErr != nil || epochErr != nil || epochDigestErr != nil || ruleErr != nil || cutoffErr != nil ||
		recordRootErr != nil || recordLengthErr != nil || transitionRootErr != nil || transitionLengthErr != nil ||
		rejectionRootErr != nil || rejectionLengthErr != nil || !cursor.done() ||
		domain != "ardents-namespace-epoch-materialization-v1" || cutoff > uint64(^uint64(0)>>1) {
		return statement{}, errors.New("naming materialization transcript is invalid")
	}
	return statement{network: network, epoch: epoch, epochDigest: epochDigest, rule: rule, cutoff: int64(cutoff),
		recordRoot: recordRoot, recordLength: recordLength, transitionRoot: transitionRoot,
		transitionLength: transitionLength, rejectionRoot: rejectionRoot, rejectionLength: rejectionLength}, nil
}
