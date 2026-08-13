package serviceconn

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	dataFrameType     = byte(1)
	ackFrameType      = byte(2)
	terminalFrameType = byte(3)
	maximumFrameData  = 16 << 10
	frameHeaderSize   = 4 + 1 + 1 + 8 + 8 + 4
)

type connectionFrame struct {
	kind       byte
	generation uint64
	offset     uint64
	data       []byte
}

func writeConnectionFrame(writer io.Writer, frame connectionFrame) error {
	if frame.generation == 0 || len(frame.data) > maximumFrameData ||
		(frame.kind != dataFrameType && frame.kind != ackFrameType && frame.kind != terminalFrameType) ||
		(frame.kind != dataFrameType && len(frame.data) != 0) {
		return errors.New("service Connection frame is outside its bound")
	}
	header := make([]byte, frameHeaderSize)
	copy(header[:4], "ASCF")
	header[4], header[5] = 1, frame.kind
	binary.BigEndian.PutUint64(header[6:14], frame.generation)
	binary.BigEndian.PutUint64(header[14:22], frame.offset)
	binary.BigEndian.PutUint32(header[22:26], uint32(len(frame.data)))
	if err := writeAll(writer, header); err != nil {
		return err
	}
	return writeAll(writer, frame.data)
}

func readConnectionFrame(reader io.Reader) (connectionFrame, error) {
	header := make([]byte, frameHeaderSize)
	if _, err := io.ReadFull(reader, header); err != nil {
		return connectionFrame{}, err
	}
	length := binary.BigEndian.Uint32(header[22:26])
	frame := connectionFrame{kind: header[5], generation: binary.BigEndian.Uint64(header[6:14]),
		offset: binary.BigEndian.Uint64(header[14:22])}
	if string(header[:4]) != "ASCF" || header[4] != 1 || frame.generation == 0 ||
		length > maximumFrameData ||
		(frame.kind != dataFrameType && frame.kind != ackFrameType && frame.kind != terminalFrameType) ||
		(frame.kind != dataFrameType && length != 0) {
		return connectionFrame{}, errors.New("service Connection frame is malformed or oversized")
	}
	frame.data = make([]byte, length)
	if _, err := io.ReadFull(reader, frame.data); err != nil {
		return connectionFrame{}, err
	}
	return frame, nil
}
