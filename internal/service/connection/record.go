package connection

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

const (
	connectionPrefix = "ardents-service-connection-v1\x00"
	version          = uint16(1)

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
	body, err := encodeRecord(record)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(writer, connectionPrefix); err != nil {
		return err
	}
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(body)))
	if _, err := writer.Write(length[:]); err != nil {
		return err
	}
	_, err = writer.Write(body)
	return err
}

// Read consumes one complete closed native record and rejects every malformed,
// oversized, H3, unknown-kind, version, profile, or trailing-body encoding.
func Read(reader io.Reader) (Record, error) {
	prefix := make([]byte, len(connectionPrefix))
	if _, err := io.ReadFull(reader, prefix); err != nil {
		return Record{}, err
	}
	if string(prefix) != connectionPrefix {
		return Record{}, errors.New("native connection record prefix is invalid")
	}
	var length [2]byte
	if _, err := io.ReadFull(reader, length[:]); err != nil {
		return Record{}, err
	}
	size := int(binary.BigEndian.Uint16(length[:]))
	if size < commonHeaderSize || size > maximumBodySize {
		return Record{}, errors.New("native connection record body is outside its bound")
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(reader, body); err != nil {
		return Record{}, err
	}
	return decodeRecord(body)
}

// ReadStream consumes one native application-stream record. It rejects the
// Challenge, Proof, and Continuity record kinds, which belong only to the
// fixed authentication and attachment phases.
func ReadStream(reader io.Reader) (StreamRecord, error) {
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
	default:
		return StreamRecord{}, errors.New("native connection stream received a non-stream record")
	}
}

// ChallengeDigest hashes the exact canonical Challenge envelope used by the
// matching InstanceProof. It has no H3 predecessor interpretation.
func ChallengeDigest(challenge Challenge) ([32]byte, error) {
	body, err := encodeRecord(Record{Challenge: &challenge})
	if err != nil {
		return [32]byte{}, err
	}
	envelope := append([]byte(connectionPrefix), byte(len(body)>>8), byte(len(body)))
	envelope = append(envelope, body...)
	return sha256.Sum256(envelope), nil
}

func encodeRecord(record Record) ([]byte, error) {
	kind, payload, err := recordPayload(record)
	if err != nil {
		return nil, err
	}
	body := make([]byte, 0, commonHeaderSize+len(payload))
	var raw [2]byte
	binary.BigEndian.PutUint16(raw[:], version)
	body = append(body, raw[:]...)
	body = append(body, kind, byte(len(Profile)))
	body = append(body, Profile...)
	body = append(body, payload...)
	if len(body) > maximumBodySize {
		return nil, errors.New("native connection record body is oversized")
	}
	return body, nil
}

func recordPayload(record Record) (byte, []byte, error) {
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
	if count != 1 {
		return 0, nil, errors.New("native connection record is not one closed kind")
	}
	switch {
	case record.Challenge != nil:
		value := record.Challenge
		if value.Network == [32]byte{} || value.Target == [32]byte{} || value.Context == [32]byte{} ||
			value.Nonce == [32]byte{} || value.InstanceGeneration == 0 {
			return 0, nil, errors.New("native InstanceChallenge is invalid")
		}
		payload := make([]byte, 0, 136)
		payload = append(payload, value.Network[:]...)
		payload = append(payload, value.Target[:]...)
		var raw [8]byte
		binary.BigEndian.PutUint64(raw[:], value.InstanceGeneration)
		payload = append(payload, raw[:]...)
		payload = append(payload, value.Context[:]...)
		payload = append(payload, value.Nonce[:]...)
		return kindChallenge, payload, nil
	case record.Proof != nil:
		value := record.Proof
		if value.ChallengeDigest == [32]byte{} || value.Signature == [64]byte{} {
			return 0, nil, errors.New("native InstanceProof is invalid")
		}
		payload := append(append([]byte(nil), value.ChallengeDigest[:]...), value.Signature[:]...)
		return kindProof, payload, nil
	case record.Continuity != nil:
		value := record.Continuity
		if !validRole(value.Role) || value.AttachmentGeneration == 0 || value.SendBase > value.SendEnd ||
			value.Nonce == [32]byte{} || value.Context == [32]byte{} || value.ExporterCommitment == [32]byte{} || value.MAC == [32]byte{} {
			return 0, nil, errors.New("native Continuity is invalid")
		}
		payload := continuityTranscript(*value)
		payload = append(payload, value.MAC[:]...)
		return kindContinuity, payload, nil
	case record.Data != nil:
		value := record.Data
		if value.AttachmentGeneration == 0 || len(value.Payload) == 0 || len(value.Payload) > MaximumDataBytes {
			return 0, nil, errors.New("native Data is invalid")
		}
		payload := connectionOffset(value.AttachmentGeneration, value.Offset)
		var length [2]byte
		binary.BigEndian.PutUint16(length[:], uint16(len(value.Payload)))
		payload = append(payload, length[:]...)
		payload = append(payload, value.Payload...)
		return kindData, payload, nil
	case record.Acknowledgement != nil:
		value := record.Acknowledgement
		if value.AttachmentGeneration == 0 {
			return 0, nil, errors.New("native Acknowledgement is invalid")
		}
		return kindAcknowledgement, connectionOffset(value.AttachmentGeneration, value.Offset), nil
	case record.Terminal != nil:
		value := record.Terminal
		if value.AttachmentGeneration == 0 {
			return 0, nil, errors.New("native Terminal is invalid")
		}
		return kindTerminal, connectionOffset(value.AttachmentGeneration, value.Offset), nil
	}
	return 0, nil, errors.New("native connection record kind is invalid")
}

func decodeRecord(body []byte) (Record, error) {
	if len(body) < commonHeaderSize || binary.BigEndian.Uint16(body[:2]) != version || body[3] != byte(len(Profile)) ||
		string(body[4:4+len(Profile)]) != Profile {
		return Record{}, errors.New("native connection record version or profile is invalid")
	}
	kind, payload := body[2], body[commonHeaderSize:]
	switch kind {
	case kindChallenge:
		if len(payload) != 136 {
			return Record{}, errors.New("native InstanceChallenge length is invalid")
		}
		value := &Challenge{InstanceGeneration: binary.BigEndian.Uint64(payload[64:72])}
		copy(value.Network[:], payload[:32])
		copy(value.Target[:], payload[32:64])
		copy(value.Context[:], payload[72:104])
		copy(value.Nonce[:], payload[104:136])
		if _, _, err := recordPayload(Record{Challenge: value}); err != nil {
			return Record{}, err
		}
		return Record{Challenge: value}, nil
	case kindProof:
		if len(payload) != 96 {
			return Record{}, errors.New("native InstanceProof length is invalid")
		}
		value := &Proof{}
		copy(value.ChallengeDigest[:], payload[:32])
		copy(value.Signature[:], payload[32:])
		if _, _, err := recordPayload(Record{Proof: value}); err != nil {
			return Record{}, err
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
		if _, _, err := recordPayload(Record{Continuity: value}); err != nil {
			return Record{}, err
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
		value := &Data{AttachmentGeneration: binary.BigEndian.Uint64(payload[:8]), Offset: binary.BigEndian.Uint64(payload[8:16]), Payload: append([]byte(nil), payload[18:]...)}
		if _, _, err := recordPayload(Record{Data: value}); err != nil {
			return Record{}, err
		}
		return Record{Data: value}, nil
	case kindAcknowledgement:
		value, err := decodeOffset(payload, "Acknowledgement")
		if err != nil {
			return Record{}, err
		}
		return Record{Acknowledgement: &Acknowledgement{AttachmentGeneration: value[0], Offset: value[1]}}, nil
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

func connectionOffset(generation, offset uint64) []byte {
	encoded := make([]byte, 16)
	binary.BigEndian.PutUint64(encoded[:8], generation)
	binary.BigEndian.PutUint64(encoded[8:], offset)
	return encoded
}

func decodeOffset(payload []byte, name string) ([2]uint64, error) {
	if len(payload) != 16 || binary.BigEndian.Uint64(payload[:8]) == 0 {
		return [2]uint64{}, errors.New("native " + name + " length is invalid")
	}
	return [2]uint64{binary.BigEndian.Uint64(payload[:8]), binary.BigEndian.Uint64(payload[8:])}, nil
}
