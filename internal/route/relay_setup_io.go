package route

import (
	"errors"
	"io"
)

// WriteRelaySetup writes one canonical RelaySetup over an already admitted
// Entry TLS connection.
func WriteRelaySetup(writer io.Writer, input RelaySetup) error {
	raw, err := EncodeRelaySetup(input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

// ReadRelaySetup reads one bounded canonical RelaySetup record.
func ReadRelaySetup(reader io.Reader) (RelaySetup, error) {
	raw, err := readRouteRecord(reader)
	if err != nil {
		return RelaySetup{}, err
	}
	return DecodeRelaySetup(raw)
}

// WriteRelayReady writes one canonical transit confirmation.
func WriteRelayReady(writer io.Writer, input RelayReady) error {
	raw, err := EncodeRelayReady(input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

// ReadRelayReady reads one bounded canonical transit confirmation.
func ReadRelayReady(reader io.Reader) (RelayReady, error) {
	raw, err := readRouteRecord(reader)
	if err != nil {
		return RelayReady{}, err
	}
	return DecodeRelayReady(raw)
}

func readRouteRecord(reader io.Reader) ([]byte, error) {
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
	if length == 0 || length > maximumWireBody {
		return nil, errors.New("route record length is invalid")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return append(header, body...), nil
}
