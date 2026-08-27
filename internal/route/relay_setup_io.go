package route

import (
	"errors"
	"io"
)

// EntryOperation is the closed set of operations an Initiator may accept
// immediately after an admitted Entry attachment. Exactly one pointer is set.
// It is not an extensible proxy or application framing surface.
type EntryOperation struct {
	Relay           *RelaySetup
	ResolutionRelay *ResolutionRelaySetup
	CredentialRelay *CredentialRelaySetup
}

// ReadEntryOperation reads one closed post-Entry operation. A C-2 relay,
// private resolution relay, and membership Credential Relay remain distinct
// authorizations despite sharing the preceding Entry admission.
func ReadEntryOperation(reader io.Reader) (EntryOperation, error) {
	raw, err := readRouteRecord(reader)
	if err != nil {
		return EntryOperation{}, err
	}
	if len(raw) < len(routeWireMagic)+2+3 {
		return EntryOperation{}, errors.New("route entry operation is truncated")
	}
	kind := raw[len(routeWireMagic)+2+2]
	switch kind {
	case relaySetupKind:
		setup, err := DecodeRelaySetup(raw)
		return EntryOperation{Relay: &setup}, err
	case resolutionRelaySetupKind:
		setup, err := DecodeResolutionRelaySetup(raw)
		return EntryOperation{ResolutionRelay: &setup}, err
	case credentialRelaySetupKind:
		setup, err := DecodeCredentialRelaySetup(raw)
		return EntryOperation{CredentialRelay: &setup}, err
	default:
		return EntryOperation{}, errors.New("route entry operation is not supported")
	}
}

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
