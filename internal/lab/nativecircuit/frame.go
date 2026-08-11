package nativecircuit

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	maximumFrameLength        = 65_536
	maximumApplicationPayload = 16_384
)

type frameType byte

const (
	frameRouteExtend frameType = iota + 1
	frameRouteResult
	frameIntroductionRegister
	frameIntroductionDeliver
	frameIntroductionAcknowledge
	frameRendezvousRegister
	frameRendezvousAttach
	frameRendezvousResult
	frameProtectedData
	frameClose
	frameTerminalError
)

type frame struct {
	Type    frameType
	Payload []byte
}

func writeFrame(writer io.Writer, value frame) error {
	if !validFrameType(value.Type) {
		return errors.New("native circuit frame type is not defined")
	}
	length := 1 + len(value.Payload)
	if length > maximumFrameLength {
		return errors.New("native circuit frame exceeds 65,536 bytes")
	}
	if value.Type == frameProtectedData && len(value.Payload) > maximumApplicationPayload {
		return errors.New("native circuit Application payload exceeds 16,384 bytes")
	}
	header := [5]byte{}
	binary.BigEndian.PutUint32(header[:4], uint32(length))
	header[4] = byte(value.Type)
	if err := writeAll(writer, header[:]); err != nil {
		return err
	}
	return writeAll(writer, value.Payload)
}

func readFrame(reader io.Reader) (frame, error) {
	header := [4]byte{}
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return frame{}, err
	}
	length := binary.BigEndian.Uint32(header[:])
	if length < 1 || length > maximumFrameLength {
		return frame{}, errors.New("native circuit frame length is outside the fixed bound")
	}
	content := make([]byte, int(length))
	if _, err := io.ReadFull(reader, content); err != nil {
		return frame{}, err
	}
	typeValue := frameType(content[0])
	if !validFrameType(typeValue) {
		return frame{}, errors.New("native circuit frame type is not defined")
	}
	payload := content[1:]
	if typeValue == frameProtectedData && len(payload) > maximumApplicationPayload {
		return frame{}, errors.New("native circuit Application payload exceeds 16,384 bytes")
	}
	return frame{Type: typeValue, Payload: payload}, nil
}

func validFrameType(value frameType) bool {
	return value >= frameRouteExtend && value <= frameTerminalError
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
