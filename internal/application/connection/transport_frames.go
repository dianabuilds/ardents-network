package connection

import (
	"encoding/binary"
	"errors"
	"io"
)

func writeData(writer io.Writer, data []byte) error {
	if len(data) == 0 || len(data) > maximumFrame {
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

const (
	maximumFrame   = 16 << 10
	terminalMarker = ^uint32(0)
)

func readRefusal(reader io.Reader) (Outcome, error) {
	var marker [4]byte
	if _, err := io.ReadFull(reader, marker[:]); err != nil || binary.BigEndian.Uint32(marker[:]) != terminalMarker {
		return Outcome{}, errors.New("local Application refusal is invalid")
	}
	return readTerminal(reader)
}

func readTerminal(reader io.Reader) (Outcome, error) {
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
