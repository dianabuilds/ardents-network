package credential

import (
	"encoding/binary"
	"errors"
	"time"
)

const (
	requestDomain  = "ATIR"
	responseDomain = "ATIS"
)

func encodeRequest(input Request) ([]byte, error) {
	if !validRequest(input) {
		return nil, errors.New("transit issuance request is invalid")
	}
	result := make([]byte, 0, 4+32*5+8+8)
	result = append(result, requestDomain...)
	for _, value := range [][32]byte{input.NetworkID, input.Digest, input.IntroductionNodeID, input.AttachmentID, input.ClientKeyDigest} {
		result = append(result, value[:]...)
	}
	result = binary.BigEndian.AppendUint64(result, input.Epoch)
	return binary.BigEndian.AppendUint64(result, uint64(input.NotAfter.Unix())), nil
}

func decodeRequest(raw []byte) (Request, error) {
	if len(raw) != 4+32*5+8+8 || string(raw[:4]) != requestDomain {
		return Request{}, errors.New("transit issuance request encoding is invalid")
	}
	offset := 4
	result := Request{}
	for _, destination := range []*[32]byte{&result.NetworkID, &result.Digest, &result.IntroductionNodeID, &result.AttachmentID, &result.ClientKeyDigest} {
		copy(destination[:], raw[offset:offset+32])
		offset += 32
	}
	result.Epoch = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	result.NotAfter = time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	if !validRequest(result) {
		return Request{}, errors.New("transit issuance request content is invalid")
	}
	return result, nil
}

func encodeResponse(grant []byte) ([]byte, error) {
	if len(grant) == 0 || len(grant) > 512 {
		return nil, errors.New("transit issuance response is invalid")
	}
	result := make([]byte, 0, 4+2+len(grant))
	result = append(result, responseDomain...)
	result = binary.BigEndian.AppendUint16(result, uint16(len(grant)))
	return append(result, grant...), nil
}

func decodeResponse(raw []byte) ([]byte, error) {
	if len(raw) < 6 || string(raw[:4]) != responseDomain {
		return nil, errors.New("transit issuance response encoding is invalid")
	}
	length := int(binary.BigEndian.Uint16(raw[4:6]))
	if length == 0 || length > 512 || len(raw) != 6+length {
		return nil, errors.New("transit issuance response length is invalid")
	}
	return append([]byte(nil), raw[6:]...), nil
}

func pad(raw []byte) ([]byte, error) {
	if len(raw) == 0 || len(raw) > messageSize-2 {
		return nil, errors.New("transit issuance message exceeds capacity")
	}
	result := make([]byte, messageSize)
	binary.BigEndian.PutUint16(result[:2], uint16(len(raw)))
	copy(result[2:], raw)
	return result, nil
}

func unpad(raw []byte) ([]byte, error) {
	if len(raw) != messageSize {
		return nil, errors.New("transit issuance message size is invalid")
	}
	length := int(binary.BigEndian.Uint16(raw[:2]))
	if length == 0 || length > len(raw)-2 {
		return nil, errors.New("transit issuance message padding is invalid")
	}
	return append([]byte(nil), raw[2:2+length]...), nil
}

func validRequest(input Request) bool {
	return input.NetworkID != [32]byte{} && input.Digest != [32]byte{} && input.IntroductionNodeID != [32]byte{} &&
		input.AttachmentID != [32]byte{} && input.ClientKeyDigest != [32]byte{} && input.Epoch != 0 && !input.NotAfter.IsZero() &&
		input.NotAfter.Unix() > 0 && input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second))
}
