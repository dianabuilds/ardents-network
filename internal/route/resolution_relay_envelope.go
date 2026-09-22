package route

import (
	"errors"
)

const (
	resolutionRelayEnvelopeKind = 12
	resolutionRelayResponseKind = 13

	// ResolutionOHTTPResponse and ResolutionOHTTPChunkedResponse are the only
	// opaque response framings accepted by the v1 OHTTP adapter.
	ResolutionOHTTPResponse        = byte(1)
	ResolutionOHTTPChunkedResponse = byte(2)
)

// ResolutionRelayEnvelope is one opaque OHTTP request or response. Route
// never interprets its bytes; its separate record capacity keeps a lookup from
// becoming a general data carrier.
type ResolutionRelayEnvelope struct{ OHTTP []byte }

// ResolutionRelayResponse returns one opaque Gateway response and its closed
// OHTTP framing. The framing is needed for decapsulation but reveals neither a
// Target nor an application response.
type ResolutionRelayResponse struct {
	OHTTP   []byte
	Framing byte
}

// EncodeResolutionRelayEnvelope encodes one Endpoint-to-Initiator OHTTP
// envelope after ResolutionRelayReady.
func EncodeResolutionRelayEnvelope(input ResolutionRelayEnvelope) ([]byte, error) {
	return resolutionEnvelopeRecord(resolutionRelayEnvelopeKind, input)
}

// DecodeResolutionRelayResponse rejects a malformed lookup response envelope.
func DecodeResolutionRelayResponse(raw []byte) (ResolutionRelayResponse, error) {
	reader, err := resolutionRouteBody(raw, resolutionRelayResponseKind)
	if err != nil {
		return ResolutionRelayResponse{}, err
	}
	result := ResolutionRelayResponse{Framing: reader.uint8()}
	length := int(reader.uint16())
	result.OHTTP = append([]byte(nil), reader.take(length)...)
	if reader.off != len(reader.raw) || len(result.OHTTP) != length || length == 0 || len(result.OHTTP) > ResolutionEnvelopeCapacity ||
		(result.Framing != ResolutionOHTTPResponse && result.Framing != ResolutionOHTTPChunkedResponse) {
		return ResolutionRelayResponse{}, errors.New("resolution relay response is invalid")
	}
	return result, nil
}

func resolutionEnvelopeRecord(kind byte, input ResolutionRelayEnvelope) ([]byte, error) {
	if len(input.OHTTP) == 0 || len(input.OHTTP) > ResolutionEnvelopeCapacity {
		return nil, errors.New("resolution relay envelope is outside its capacity")
	}
	body := make([]byte, 0, 2+1+1+len(Profile)+2+len(input.OHTTP))
	body = appendUint16(body, routeWireVersion)
	body = append(body, kind)
	body = appendProfile(body)
	body = appendUint16(body, uint16(len(input.OHTTP)))
	body = append(body, input.OHTTP...)
	return resolutionRouteEnvelope(body)
}
