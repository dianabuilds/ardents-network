package route

import (
	"errors"
	"io"
)

const (
	credentialRelayEnvelopeKind = 16
	credentialRelayResponseKind = 17

	// CredentialOHTTPResponse is the only OHTTP response framing accepted by
	// the first bounded credential exchange.
	CredentialOHTTPResponse = byte(1)
)

// CredentialRelayEnvelope is one opaque OHTTP request or response. Route
// cannot decode it and does not expose this finite exchange as a data relay.
type CredentialRelayEnvelope struct{ OHTTP []byte }

// CredentialRelayResponse returns the sole opaque issuer response before the
// Entry attachment closes.
type CredentialRelayResponse struct {
	OHTTP   []byte
	Framing byte
}

func ReadCredentialRelayReady(reader io.Reader) (CredentialRelayReady, error) {
	raw, err := readRouteRecord(reader)
	if err != nil {
		return CredentialRelayReady{}, err
	}
	return DecodeCredentialRelayReady(raw)
}

func WriteCredentialRelayEnvelope(writer io.Writer, input CredentialRelayEnvelope) error {
	raw, err := encodeCredentialRelayEnvelope(credentialRelayEnvelopeKind, input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

func ReadCredentialRelayResponse(reader io.Reader) (CredentialRelayResponse, error) {
	raw, err := readCredentialRouteRecord(reader)
	if err != nil {
		return CredentialRelayResponse{}, err
	}
	return decodeCredentialRelayResponse(raw)
}

func encodeCredentialRelayEnvelope(kind byte, input CredentialRelayEnvelope) ([]byte, error) {
	if len(input.OHTTP) == 0 || len(input.OHTTP) > CredentialEnvelopeCapacity {
		return nil, errors.New("credential relay envelope is outside its capacity")
	}
	body := make([]byte, 0, 2+1+1+len(Profile)+2+len(input.OHTTP))
	body = appendUint16(body, routeWireVersion)
	body = append(body, kind)
	body = appendProfile(body)
	body = appendUint16(body, uint16(len(input.OHTTP)))
	body = append(body, input.OHTTP...)
	return credentialRouteEnvelope(body)
}

func decodeCredentialRelayResponse(raw []byte) (CredentialRelayResponse, error) {
	reader, err := credentialRouteBody(raw, credentialRelayResponseKind)
	if err != nil {
		return CredentialRelayResponse{}, err
	}
	result := CredentialRelayResponse{Framing: reader.uint8()}
	length := int(reader.uint16())
	result.OHTTP = append([]byte(nil), reader.take(length)...)
	if reader.off != len(reader.raw) || result.Framing != CredentialOHTTPResponse || len(result.OHTTP) != length ||
		length == 0 || length > CredentialEnvelopeCapacity {
		return CredentialRelayResponse{}, errors.New("credential relay response is invalid")
	}
	return result, nil
}

func credentialRouteEnvelope(body []byte) ([]byte, error) {
	if len(body) == 0 || len(body) > CredentialEnvelopeCapacity+64 {
		return nil, errors.New("credential relay wire body is outside its bound")
	}
	result := make([]byte, 0, len(routeWireMagic)+2+len(body))
	result = append(result, routeWireMagic...)
	result = appendUint16(result, uint16(len(body)))
	return append(result, body...), nil
}

func credentialRouteBody(raw []byte, kind byte) (*wireReader, error) {
	if len(raw) < len(routeWireMagic)+2 || string(raw[:len(routeWireMagic)]) != routeWireMagic {
		return nil, errors.New("route wire magic is invalid")
	}
	reader := &wireReader{raw: raw[len(routeWireMagic):]}
	length := int(reader.uint16())
	body := reader.take(length)
	if reader.off != len(reader.raw) || len(body) != length || length == 0 || length > CredentialEnvelopeCapacity+64 {
		return nil, errors.New("credential relay wire length is invalid")
	}
	reader = &wireReader{raw: body}
	if reader.uint16() != routeWireVersion || reader.uint8() != kind {
		return nil, errors.New("credential relay wire kind or version is invalid")
	}
	profileLength := int(reader.uint8())
	if !validRouteProfile(string(reader.take(profileLength))) {
		return nil, errors.New("credential relay wire profile is invalid")
	}
	return reader, nil
}

func readCredentialRouteRecord(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, errors.New("route record reader is unavailable")
	}
	header := make([]byte, len(routeWireMagic)+2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	if string(header[:len(routeWireMagic)]) != routeWireMagic {
		return nil, errors.New("route record magic is invalid")
	}
	length := int(header[len(routeWireMagic)])<<8 | int(header[len(routeWireMagic)+1])
	if length == 0 || length > CredentialEnvelopeCapacity+64 {
		return nil, errors.New("credential relay wire length is invalid")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return append(header, body...), nil
}
