package serviceconn

import (
	"errors"
	"io"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

const (
	dataFrameType     = byte(1)
	ackFrameType      = byte(2)
	terminalFrameType = byte(3)
	maximumFrameData  = nativeconnection.MaximumDataBytes
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
	var record nativeconnection.Record
	switch frame.kind {
	case dataFrameType:
		record.Data = &nativeconnection.Data{AttachmentGeneration: frame.generation, Offset: frame.offset, Payload: frame.data}
	case ackFrameType:
		record.Acknowledgement = &nativeconnection.Acknowledgement{AttachmentGeneration: frame.generation, Offset: frame.offset}
	case terminalFrameType:
		record.Terminal = &nativeconnection.Terminal{AttachmentGeneration: frame.generation, Offset: frame.offset}
	}
	return nativeconnection.Write(writer, record)
}

func readConnectionFrame(reader io.Reader) (connectionFrame, error) {
	record, err := nativeconnection.Read(reader)
	if err != nil {
		return connectionFrame{}, err
	}
	switch {
	case record.Data != nil:
		return connectionFrame{kind: dataFrameType, generation: record.Data.AttachmentGeneration, offset: record.Data.Offset, data: record.Data.Payload}, nil
	case record.Acknowledgement != nil:
		return connectionFrame{kind: ackFrameType, generation: record.Acknowledgement.AttachmentGeneration, offset: record.Acknowledgement.Offset}, nil
	case record.Terminal != nil:
		return connectionFrame{kind: terminalFrameType, generation: record.Terminal.AttachmentGeneration, offset: record.Terminal.Offset}, nil
	default:
		return connectionFrame{}, errors.New("native connection stream received a non-stream record")
	}
}
