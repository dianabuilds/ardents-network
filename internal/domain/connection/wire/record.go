package wire

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

// ErrInvalidRecord marks a completely observed structural contradiction.
// Physical EOF, truncation and other reader failures retain their original
// causes instead; they do not prove that a complete malformed record arrived.
var ErrInvalidRecord = errors.New("service Connection record is invalid")

const (
	connectionPrefix = "ardents-service-connection-v3\x00"
	version          = uint16(3)

	kindChallenge       = byte(1)
	kindProof           = byte(2)
	kindContinuity      = byte(3)
	kindData            = byte(4)
	kindAcknowledgement = byte(5)
	kindTerminal        = byte(6)

	commonHeaderSize = 2 + 1 + 1 + len(Profile)
	maximumBodySize  = commonHeaderSize + 8 + 8 + 2 + MaximumDataBytes
)

// Write emits exactly one closed native record. It accepts no generic type,
// unknown kind, profile, optional field, or caller-selected version.
func Write(writer io.Writer, record Record) error {
	envelope, err := encodeEnvelope(record)
	if err != nil {
		return err
	}
	if writer == nil {
		return errors.New("native connection record writer is absent")
	}
	n, err := writer.Write(envelope)
	if err != nil {
		return err
	}
	if n != len(envelope) {
		return io.ErrShortWrite
	}
	return nil
}

// Read consumes one complete closed native record and rejects every malformed,
// oversized, unknown-kind, version, profile, or trailing-body encoding.
func Read(reader io.Reader) (Record, error) {
	if reader == nil {
		return Record{}, errors.New("native connection record reader is absent")
	}
	size, err := readRecordHeader(reader)
	if err != nil {
		return Record{}, err
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(reader, body); err != nil {
		return Record{}, err
	}
	record, err := decodeRecord(body)
	if err != nil {
		return Record{}, errors.Join(ErrInvalidRecord, err)
	}
	return record, nil
}

// ReadStream consumes one native application-stream record. It rejects the
// Challenge, Proof, and Continuity record kinds, which belong only to the
// fixed authentication and attachment phases. Segment controls also require
// a separate phase-aware consumer.
func ReadStream(reader io.Reader) (StreamRecord, error) {
	return readStream(reader, false)
}

// ReadSegmentStream reads the same stream grammar with the selected
// continuation controls. The original Stream must check their phase.
func ReadSegmentStream(reader io.Reader) (StreamRecord, error) {
	return readStream(reader, true)
}

func readStream(reader io.Reader, continuation bool) (StreamRecord, error) {
	record, err := Read(reader)
	if err != nil {
		return StreamRecord{}, err
	}
	switch {
	case record.Data != nil:
		return StreamRecord{Data: record.Data}, nil
	case record.Acknowledgement != nil:
		return StreamRecord{Acknowledgement: record.Acknowledgement}, nil
	case record.Terminal != nil:
		return StreamRecord{Terminal: record.Terminal}, nil
	case continuation && record.Continuation != nil:
		return StreamRecord{Continuation: record.Continuation}, nil
	default:
		return StreamRecord{}, errors.Join(ErrInvalidRecord, errors.New("native connection stream received a non-stream record"))
	}
}

// ChallengeDigest hashes the exact canonical Challenge envelope used by the
// matching InstanceProof. It has no predecessor interpretation.
func ChallengeDigest(challenge Challenge) ([32]byte, error) {
	envelope, err := encodeEnvelope(Record{Challenge: &challenge})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(envelope), nil
}

// encodeEnvelope builds the complete canonical envelope in one allocation:
// prefix, length, common header and payload are written directly into the final
// buffer, so the payload is copied once rather than through a separate body and
// envelope (audit SW-1). The refused grammar and canonical bytes are unchanged.
func encodeEnvelope(record Record) ([]byte, error) {
	kind, size, err := recordKindSize(record)
	if err != nil {
		return nil, err
	}
	bodySize := commonHeaderSize + size
	if bodySize > maximumBodySize {
		return nil, errors.New("native connection record body is oversized")
	}
	envelope := make([]byte, len(connectionPrefix)+2+bodySize)
	copy(envelope, connectionPrefix)
	envelope[len(connectionPrefix)] = byte(bodySize >> 8)
	envelope[len(connectionPrefix)+1] = byte(bodySize)
	body := envelope[len(connectionPrefix)+2:]
	binary.BigEndian.PutUint16(body[:2], version)
	body[2] = kind
	body[3] = byte(len(Profile))
	copy(body[4:4+len(Profile)], Profile)
	appendRecordPayload(record, kind, body[commonHeaderSize:])
	return envelope, nil
}

// recordKindSize validates the closed one-kind grammar and returns the exact
// payload size without allocating. It owns the same refusal order and messages
// as the former re-encoding validator.
func recordKindSize(record Record) (byte, int, error) {
	count := 0
	if record.Challenge != nil {
		count++
	}
	if record.Proof != nil {
		count++
	}
	if record.Continuity != nil {
		count++
	}
	if record.Data != nil {
		count++
	}
	if record.Acknowledgement != nil {
		count++
	}
	if record.Terminal != nil {
		count++
	}
	if record.Continuation != nil {
		count++
	}
	if count != 1 {
		return 0, 0, errors.New("native connection record is not one closed kind")
	}
	switch {
	case record.Continuation != nil:
		size, err := record.Continuation.payloadSize()
		return byte(record.Continuation.Kind), size, err
	case record.Challenge != nil:
		value := record.Challenge
		if value.Network == [32]byte{} || value.Target == [32]byte{} || value.Context == [32]byte{} ||
			value.Nonce == [32]byte{} || value.InstanceGeneration == 0 {
			return 0, 0, errors.New("native InstanceChallenge is invalid")
		}
		return kindChallenge, 136, nil
	case record.Proof != nil:
		value := record.Proof
		if value.ChallengeDigest == [32]byte{} || value.Signature == [64]byte{} {
			return 0, 0, errors.New("native InstanceProof is invalid")
		}
		return kindProof, 96, nil
	case record.Continuity != nil:
		value := record.Continuity
		if !validRole(value.Role) || value.AttachmentGeneration == 0 || value.SendBase > value.SendEnd ||
			value.Nonce == [32]byte{} || value.Context == [32]byte{} || value.ExporterCommitment == [32]byte{} || value.MAC == [32]byte{} {
			return 0, 0, errors.New("native Continuity is invalid")
		}
		return kindContinuity, 1 + 8*4 + 32*4, nil
	case record.Data != nil:
		value := record.Data
		if value.AttachmentGeneration == 0 || len(value.Payload) == 0 || len(value.Payload) > MaximumDataBytes || uint64(len(value.Payload)) > ^uint64(0)-value.Offset {
			return 0, 0, errors.New("native Data is invalid")
		}
		return kindData, 18 + len(value.Payload), nil
	case record.Acknowledgement != nil:
		value := record.Acknowledgement
		if value.AttachmentGeneration == 0 {
			return 0, 0, errors.New("native Acknowledgement is invalid")
		}
		if value.Terminal {
			return kindAcknowledgement, 17, nil
		}
		if value.TerminalConfirmation {
			return 0, 0, errors.New("native Terminal Acknowledgement confirmation is invalid")
		}
		return kindAcknowledgement, 16, nil
	case record.Terminal != nil:
		value := record.Terminal
		if value.AttachmentGeneration == 0 {
			return 0, 0, errors.New("native Terminal is invalid")
		}
		return kindTerminal, 16, nil
	}
	return 0, 0, errors.New("native connection record kind is invalid")
}

// appendRecordPayload writes the validated payload fields directly into dst,
// which encodeEnvelope sized exactly. It performs no validation or allocation.
func appendRecordPayload(record Record, kind byte, dst []byte) {
	switch kind {
	case byte(DrainSegment), byte(ProposeBoundary), byte(OfferContinuation), byte(ConfirmBoundary), byte(ResumeTransfer), byte(AcceptResume), byte(AcceptBoundary), byte(StartTransfer), byte(AcceptTransfer):
		record.Continuation.encodePayload(dst)
	case kindChallenge:
		value := record.Challenge
		copy(dst[0:32], value.Network[:])
		copy(dst[32:64], value.Target[:])
		binary.BigEndian.PutUint64(dst[64:72], value.InstanceGeneration)
		copy(dst[72:104], value.Context[:])
		copy(dst[104:136], value.Nonce[:])
	case kindProof:
		value := record.Proof
		copy(dst[0:32], value.ChallengeDigest[:])
		copy(dst[32:96], value.Signature[:])
	case kindContinuity:
		value := record.Continuity
		dst[0] = byte(value.Role)
		binary.BigEndian.PutUint64(dst[1:9], value.AttachmentGeneration)
		binary.BigEndian.PutUint64(dst[9:17], value.SendBase)
		binary.BigEndian.PutUint64(dst[17:25], value.SendEnd)
		binary.BigEndian.PutUint64(dst[25:33], value.ReceiveNext)
		copy(dst[33:65], value.Nonce[:])
		copy(dst[65:97], value.Context[:])
		copy(dst[97:129], value.ExporterCommitment[:])
		copy(dst[129:161], value.MAC[:])
	case kindData:
		value := record.Data
		binary.BigEndian.PutUint64(dst[0:8], value.AttachmentGeneration)
		binary.BigEndian.PutUint64(dst[8:16], value.Offset)
		binary.BigEndian.PutUint16(dst[16:18], uint16(len(value.Payload)))
		copy(dst[18:], value.Payload)
	case kindAcknowledgement:
		value := record.Acknowledgement
		binary.BigEndian.PutUint64(dst[0:8], value.AttachmentGeneration)
		binary.BigEndian.PutUint64(dst[8:16], value.Offset)
		if value.Terminal {
			marker := byte(1)
			if value.TerminalConfirmation {
				marker = 2
			}
			dst[16] = marker
		}
	case kindTerminal:
		value := record.Terminal
		binary.BigEndian.PutUint64(dst[0:8], value.AttachmentGeneration)
		binary.BigEndian.PutUint64(dst[8:16], value.Offset)
	}
}

func decodeRecord(body []byte) (Record, error) {
	if len(body) < commonHeaderSize || binary.BigEndian.Uint16(body[:2]) != version || body[3] != byte(len(Profile)) ||
		string(body[4:4+len(Profile)]) != Profile {
		return Record{}, errors.New("native connection record version or profile is invalid")
	}
	kind, payload := body[2], body[commonHeaderSize:]
	switch kind {
	case byte(DrainSegment), byte(ProposeBoundary), byte(OfferContinuation), byte(ConfirmBoundary), byte(ResumeTransfer), byte(AcceptResume), byte(AcceptBoundary), byte(StartTransfer), byte(AcceptTransfer):
		return decodeContinuation(ContinuationKind(kind), payload)
	case kindChallenge:
		if len(payload) != 136 {
			return Record{}, errors.New("native InstanceChallenge length is invalid")
		}
		value := &Challenge{InstanceGeneration: binary.BigEndian.Uint64(payload[64:72])}
		copy(value.Network[:], payload[:32])
		copy(value.Target[:], payload[32:64])
		copy(value.Context[:], payload[72:104])
		copy(value.Nonce[:], payload[104:136])
		// Validate the decoded fields directly rather than re-encoding the whole
		// payload only to discard it (audit SW-1). This matches recordKindSize's
		// Challenge refusal grammar exactly.
		if value.Network == [32]byte{} || value.Target == [32]byte{} || value.Context == [32]byte{} ||
			value.Nonce == [32]byte{} || value.InstanceGeneration == 0 {
			return Record{}, errors.New("native InstanceChallenge is invalid")
		}
		return Record{Challenge: value}, nil
	case kindProof:
		if len(payload) != 96 {
			return Record{}, errors.New("native InstanceProof length is invalid")
		}
		value := &Proof{}
		copy(value.ChallengeDigest[:], payload[:32])
		copy(value.Signature[:], payload[32:])
		if value.ChallengeDigest == [32]byte{} || value.Signature == [64]byte{} {
			return Record{}, errors.New("native InstanceProof is invalid")
		}
		return Record{Proof: value}, nil
	case kindContinuity:
		if len(payload) != 1+8*4+32*4 {
			return Record{}, errors.New("native Continuity length is invalid")
		}
		value := &Continuity{Role: Role(payload[0]), AttachmentGeneration: binary.BigEndian.Uint64(payload[1:9]),
			SendBase: binary.BigEndian.Uint64(payload[9:17]), SendEnd: binary.BigEndian.Uint64(payload[17:25]), ReceiveNext: binary.BigEndian.Uint64(payload[25:33])}
		copy(value.Nonce[:], payload[33:65])
		copy(value.Context[:], payload[65:97])
		copy(value.ExporterCommitment[:], payload[97:129])
		copy(value.MAC[:], payload[129:161])
		if !validRole(value.Role) || value.AttachmentGeneration == 0 || value.SendBase > value.SendEnd ||
			value.Nonce == [32]byte{} || value.Context == [32]byte{} || value.ExporterCommitment == [32]byte{} || value.MAC == [32]byte{} {
			return Record{}, errors.New("native Continuity is invalid")
		}
		return Record{Continuity: value}, nil
	case kindData:
		if len(payload) < 18 {
			return Record{}, errors.New("native Data length is invalid")
		}
		length := int(binary.BigEndian.Uint16(payload[16:18]))
		if length == 0 || length > MaximumDataBytes || len(payload) != 18+length {
			return Record{}, errors.New("native Data length is invalid")
		}
		// The body is a fresh per-Read allocation owned only by this record, so the
		// payload can reference it directly instead of copying it again (audit SW-1).
		// The consumer retains accepted bytes in its own bounded ledger buffer.
		value := &Data{AttachmentGeneration: binary.BigEndian.Uint64(payload[:8]), Offset: binary.BigEndian.Uint64(payload[8:16]), Payload: payload[18:]}
		// Validate the decoded fields directly rather than re-encoding the whole
		// payload only to discard it (audit SW-1). The length bounds are checked
		// above; this matches recordKindSize's remaining Data refusal grammar.
		if value.AttachmentGeneration == 0 || uint64(len(value.Payload)) > ^uint64(0)-value.Offset {
			return Record{}, errors.New("native Data is invalid")
		}
		return Record{Data: value}, nil
	case kindAcknowledgement:
		terminal, confirmation := false, false
		if len(payload) == 17 {
			switch payload[16] {
			case 1:
				terminal = true
			case 2:
				terminal, confirmation = true, true
			default:
				return Record{}, errors.New("native Terminal Acknowledgement marker is invalid")
			}
			payload = payload[:16]
		}
		value, err := decodeOffset(payload, "Acknowledgement")
		if err != nil {
			return Record{}, err
		}
		return Record{Acknowledgement: &Acknowledgement{AttachmentGeneration: value[0], Offset: value[1], Terminal: terminal, TerminalConfirmation: confirmation}}, nil
	case kindTerminal:
		value, err := decodeOffset(payload, "Terminal")
		if err != nil {
			return Record{}, err
		}
		return Record{Terminal: &Terminal{AttachmentGeneration: value[0], Offset: value[1]}}, nil
	default:
		return Record{}, errors.New("native connection record kind is unknown")
	}
}

func decodeOffset(payload []byte, name string) ([2]uint64, error) {
	if len(payload) != 16 || binary.BigEndian.Uint64(payload[:8]) == 0 {
		return [2]uint64{}, errors.New("native " + name + " length is invalid")
	}
	return [2]uint64{binary.BigEndian.Uint64(payload[:8]), binary.BigEndian.Uint64(payload[8:])}, nil
}
