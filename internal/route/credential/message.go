package credential

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const (
	requestDomain  = "ATI3"
	responseDomain = "ATO2"
)

func encodeRequest(input Request) ([]byte, error) {
	if !validRequest(input) {
		return nil, errors.New("transit issuance request is invalid")
	}
	result := make([]byte, 0, 4+32*6+8+1+8)
	result = append(result, requestDomain...)
	for _, value := range [][32]byte{input.RequestID, input.NetworkID, input.Digest, input.TransitNodeID, input.AttachmentID, input.ClientKeyDigest} {
		result = append(result, value[:]...)
	}
	result = binary.BigEndian.AppendUint64(result, input.Epoch)
	result = append(result, input.TransitRole)
	return binary.BigEndian.AppendUint64(result, uint64(input.NotAfter.Unix())), nil
}

func decodeRequest(raw []byte) (Request, error) {
	if len(raw) != 4+32*6+8+1+8 || string(raw[:4]) != requestDomain {
		return Request{}, errors.New("transit issuance request encoding is invalid")
	}
	offset := 4
	result := Request{}
	for _, destination := range []*[32]byte{&result.RequestID, &result.NetworkID, &result.Digest, &result.TransitNodeID, &result.AttachmentID, &result.ClientKeyDigest} {
		copy(destination[:], raw[offset:offset+32])
		offset += 32
	}
	result.Epoch = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	result.TransitRole = raw[offset]
	offset++
	result.NotAfter = time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	if !validRequest(result) {
		return Request{}, errors.New("transit issuance request content is invalid")
	}
	return result, nil
}

func encodeResponse(result Result) ([]byte, error) {
	class, valid := outcomeClass(result)
	if !valid {
		return nil, errors.New("transit issuance response is invalid")
	}
	raw := make([]byte, 0, 4+1+2+len(result.Grant))
	raw = append(raw, responseDomain...)
	raw = append(raw, class)
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(result.Grant)))
	return append(raw, result.Grant...), nil
}

func decodeResponse(raw []byte) (Result, error) {
	if len(raw) < 7 || string(raw[:4]) != responseDomain {
		return Result{}, errors.New("transit issuance response encoding is invalid")
	}
	result := Result{Outcome: classOutcome(raw[4])}
	length := int(binary.BigEndian.Uint16(raw[5:7]))
	if result.Outcome == "" || length > 512 || len(raw) != 7+length || result.Outcome == Issued && length == 0 ||
		result.Outcome != Issued && length != 0 {
		return Result{}, errors.New("transit issuance response length is invalid")
	}
	result.Grant = append([]byte(nil), raw[7:]...)
	return result, nil
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
	return input.RequestID != [32]byte{} && input.NetworkID != [32]byte{} && input.Digest != [32]byte{} && input.TransitNodeID != [32]byte{} &&
		input.AttachmentID != [32]byte{} && input.ClientKeyDigest != [32]byte{} && input.Epoch != 0 && !input.NotAfter.IsZero() &&
		(input.TransitRole == route.IntroductionRole || input.TransitRole == route.ResponderRole) &&
		input.NotAfter.Unix() > 0 && input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second))
}

func outcomeClass(result Result) (byte, bool) {
	switch result.Outcome {
	case Issued:
		return 1, len(result.Grant) > 0 && len(result.Grant) <= 512
	case Exhausted:
		return 2, len(result.Grant) == 0
	case Withdrawn:
		return 3, len(result.Grant) == 0
	case Unavailable:
		return 4, len(result.Grant) == 0
	default:
		return 0, false
	}
}

func classOutcome(class byte) Outcome {
	switch class {
	case 1:
		return Issued
	case 2:
		return Exhausted
	case 3:
		return Withdrawn
	case 4:
		return Unavailable
	default:
		return ""
	}
}
