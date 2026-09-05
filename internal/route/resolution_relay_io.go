package route

import (
	"errors"
	"io"
)

// WriteResolutionRelaySetup writes one lookup authorization over an admitted
// Entry TLS connection.
func WriteResolutionRelaySetup(writer io.Writer, input ResolutionRelaySetup) error {
	raw, err := EncodeResolutionRelaySetup(input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

// ReadResolutionRelaySetup reads one bounded lookup authorization.
func ReadResolutionRelaySetup(reader io.Reader) (ResolutionRelaySetup, error) {
	raw, err := readRouteRecord(reader)
	if err != nil {
		return ResolutionRelaySetup{}, err
	}
	return DecodeResolutionRelaySetup(raw)
}

// WriteResolutionRelayReady writes one exact lookup confirmation.
func WriteResolutionRelayReady(writer io.Writer, input ResolutionRelayReady) error {
	raw, err := EncodeResolutionRelayReady(input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

// ReadResolutionRelayReady reads one bounded lookup confirmation.
func ReadResolutionRelayReady(reader io.Reader) (ResolutionRelayReady, error) {
	raw, err := readRouteRecord(reader)
	if err != nil {
		return ResolutionRelayReady{}, err
	}
	return DecodeResolutionRelayReady(raw)
}

// WriteResolutionRelayEnvelope writes the sole opaque OHTTP request.
func WriteResolutionRelayEnvelope(writer io.Writer, input ResolutionRelayEnvelope) error {
	raw, err := EncodeResolutionRelayEnvelope(input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

// ReadResolutionRelayEnvelope reads the sole opaque OHTTP request.
func ReadResolutionRelayEnvelope(reader io.Reader) (ResolutionRelayEnvelope, error) {
	raw, err := readResolutionRouteRecord(reader)
	if err != nil {
		return ResolutionRelayEnvelope{}, err
	}
	return DecodeResolutionRelayEnvelope(raw)
}

// WriteResolutionRelayResponse writes the sole opaque OHTTP response.
func WriteResolutionRelayResponse(writer io.Writer, input ResolutionRelayResponse) error {
	raw, err := EncodeResolutionRelayResponse(input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

// ReadResolutionRelayResponse reads the sole opaque OHTTP response.
func ReadResolutionRelayResponse(reader io.Reader) (ResolutionRelayResponse, error) {
	raw, err := readResolutionRouteRecord(reader)
	if err != nil {
		return ResolutionRelayResponse{}, err
	}
	return DecodeResolutionRelayResponse(raw)
}

func resolutionRouteEnvelope(body []byte) ([]byte, error) {
	if len(body) == 0 || len(body) > ResolutionEnvelopeCapacity+64 {
		return nil, errors.New("resolution relay wire body is outside its bound")
	}
	result := make([]byte, 0, len(routeWireMagic)+2+len(body))
	result = append(result, routeWireMagic...)
	result = appendUint16(result, uint16(len(body)))
	return append(result, body...), nil
}

func resolutionRouteBody(raw []byte, kind byte) (*wireReader, error) {
	if len(raw) < len(routeWireMagic)+2 || string(raw[:len(routeWireMagic)]) != routeWireMagic {
		return nil, errors.New("route wire magic is invalid")
	}
	reader := &wireReader{raw: raw[len(routeWireMagic):]}
	length := int(reader.uint16())
	body := reader.take(length)
	if reader.off != len(reader.raw) || len(body) != length || length == 0 || length > ResolutionEnvelopeCapacity+64 {
		return nil, errors.New("resolution relay wire length is invalid")
	}
	reader = &wireReader{raw: body}
	if reader.uint16() != routeWireVersion || reader.uint8() != kind {
		return nil, errors.New("route wire kind or version is invalid")
	}
	profileLength := int(reader.uint8())
	if !validRouteProfile(string(reader.take(profileLength))) {
		return nil, errors.New("route wire profile is invalid")
	}
	return reader, nil
}

func readResolutionRouteRecord(reader io.Reader) ([]byte, error) {
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
	if length == 0 || length > ResolutionEnvelopeCapacity+64 {
		return nil, errors.New("resolution relay wire length is invalid")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return append(header, body...), nil
}
