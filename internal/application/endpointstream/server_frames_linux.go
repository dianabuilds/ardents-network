//go:build linux

package endpointstream

import (
	"encoding/binary"
	"errors"
	"io"
)

// DecodeRequest accepts exactly one canonical local request. Name is returned
// as a typed request; its receiver must refuse it before any network effect.
func DecodeRequest(raw []byte) (Request, error) {
	const magicSize = len(localMagic)
	if len(raw) < magicSize+3 {
		return Request{}, errors.New("local Application Connection request is invalid")
	}
	if string(raw[:magicSize]) != localMagic {
		return Request{}, errors.New("local Application Connection request is invalid")
	}
	length := int(binary.BigEndian.Uint16(raw[magicSize+1:]))
	if length == 0 || length > maximumTarget || len(raw) != magicSize+3+length {
		return Request{}, errors.New("local Application Connection destination is invalid")
	}
	request := Request{Destination: DestinationTag(raw[magicSize]), Value: string(raw[magicSize+3:])}
	if err := ValidateRequest(request); err != nil {
		return Request{}, err
	}
	return request, nil
}

// ReadRequest reads one request and refuses the unselected Name before admission.
func ReadRequest(reader io.Reader) (Request, error) {
	var header [7]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return Request{}, errors.New("local Application request is invalid")
	}
	if magic := string(header[:4]); magic != localMagic {
		return Request{}, errors.New("local Application request is invalid")
	}
	length := int(binary.BigEndian.Uint16(header[5:]))
	if length == 0 || length > maximumTarget {
		return Request{}, errors.New("local Application destination is invalid")
	}
	raw := make([]byte, len(header)+length)
	copy(raw, header[:])
	if _, err := io.ReadFull(reader, raw[len(header):]); err != nil {
		return Request{}, err
	}
	request, err := DecodeRequest(raw)
	if err != nil {
		return Request{}, err
	}
	if request.Destination == Name {
		return Request{}, Refuse(Outcome{Class: "not-selected", Reason: "canonical Names are not selected"})
	}
	return request, nil
}

// WriteTerminal writes one bounded terminal outcome in the shared frame grammar.
func WriteTerminal(writer io.Writer, outcome Outcome) error {
	if err := validOutcome(outcome); err != nil {
		return err
	}
	var header [8]byte
	binary.BigEndian.PutUint32(header[:4], TerminalMarker)
	binary.BigEndian.PutUint16(header[4:6], uint16(len(outcome.Class)))
	binary.BigEndian.PutUint16(header[6:8], uint16(len(outcome.Reason)))
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, string(outcome.Class)); err != nil {
		return err
	}
	_, err := io.WriteString(writer, outcome.Reason)
	return err
}
