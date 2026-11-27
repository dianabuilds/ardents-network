package endpointstream

import (
	"encoding/binary"
	"errors"
	"io"
)

// WriteData writes one bounded nonempty Application frame.
func WriteData(writer io.Writer, data []byte) error {
	if len(data) == 0 || len(data) > MaximumFrame {
		return errors.New("local Application data frame is invalid")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data)))
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

// MaximumFrame bounds one data frame; TerminalMarker marks a terminal outcome.
const (
	MaximumFrame   = 16 << 10
	TerminalMarker = ^uint32(0)
)

// ReadRefusal reads a terminal refusal from the peer.
func ReadRefusal(reader io.Reader) (Outcome, error) {
	var marker [4]byte
	if _, err := io.ReadFull(reader, marker[:]); err != nil || binary.BigEndian.Uint32(marker[:]) != TerminalMarker {
		return Outcome{}, errors.New("local Application refusal is invalid")
	}
	return ReadTerminal(reader)
}

// ReadTerminal reads and validates a bounded terminal outcome.
func ReadTerminal(reader io.Reader) (Outcome, error) {
	var lengths [4]byte
	if _, err := io.ReadFull(reader, lengths[:]); err != nil {
		return Outcome{}, err
	}
	classLength, reasonLength := int(binary.BigEndian.Uint16(lengths[:2])), int(binary.BigEndian.Uint16(lengths[2:]))
	if classLength == 0 || classLength > maximumOutcomeClassBytes || reasonLength > maximumOutcomeReasonBytes {
		return Outcome{}, errors.New("local Application terminal frame is invalid")
	}
	raw := make([]byte, classLength+reasonLength)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return Outcome{}, err
	}
	outcome := Outcome{Class: OutcomeClass(raw[:classLength]), Reason: string(raw[classLength:])}
	if err := validOutcome(outcome); err != nil {
		return Outcome{}, err
	}
	return outcome, nil
}
