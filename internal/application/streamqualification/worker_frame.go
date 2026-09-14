package streamqualification

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	frameOpen   = 1
	frameBytes  = 2
	frameCredit = 3
	frameEOF    = 4
	frameClose  = 5

	frameLimit               = 16 << 10
	frameCreditWindow uint32 = 64 << 10
)

type workerFrame struct {
	kind byte
	id   uint32
	body []byte
}

func readWorkerFrame(input io.Reader) (workerFrame, error) {
	if input == nil {
		return workerFrame{}, errors.New("qualification worker frame input is unavailable")
	}
	var header [9]byte
	if _, err := io.ReadFull(input, header[:]); err != nil {
		return workerFrame{}, err
	}
	frame := workerFrame{kind: header[0], id: binary.BigEndian.Uint32(header[1:5])}
	size := binary.BigEndian.Uint32(header[5:])
	valid := frame.id != 0 && ((frame.kind == frameOpen || frame.kind == frameEOF) && size == 0 ||
		frame.kind == frameBytes && size > 0 && size <= frameLimit || frame.kind == frameCredit && size == 4 ||
		frame.kind == frameClose && size == 1)
	if !valid {
		return workerFrame{}, errors.New("qualification worker frame is invalid")
	}
	frame.body = make([]byte, size)
	if _, err := io.ReadFull(input, frame.body); err != nil {
		return workerFrame{}, err
	}
	return frame, nil
}

func writeWorkerFrame(output io.Writer, frame workerFrame) error {
	if output == nil || frame.id == 0 || len(frame.body) > frameLimit ||
		(frame.kind != frameOpen && frame.kind != frameBytes && frame.kind != frameCredit && frame.kind != frameEOF && frame.kind != frameClose) ||
		((frame.kind == frameOpen || frame.kind == frameEOF) && len(frame.body) != 0) ||
		(frame.kind == frameBytes && len(frame.body) == 0) || (frame.kind == frameCredit && len(frame.body) != 4) ||
		(frame.kind == frameClose && len(frame.body) != 1) {
		return errors.New("qualification worker frame is invalid")
	}
	var header [9]byte
	header[0] = frame.kind
	binary.BigEndian.PutUint32(header[1:5], frame.id)
	binary.BigEndian.PutUint32(header[5:], uint32(len(frame.body)))
	if err := writeWorkerBytes(output, header[:]); err != nil {
		return err
	}
	return writeWorkerBytes(output, frame.body)
}

func writeWorkerBytes(output io.Writer, body []byte) error {
	for len(body) > 0 {
		n, err := output.Write(body)
		if n < 0 || n > len(body) {
			return io.ErrShortWrite
		}
		body = body[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
