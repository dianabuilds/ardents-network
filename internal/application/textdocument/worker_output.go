//go:build linux

package textdocument

import (
	"encoding/binary"
	"errors"
	"io"
)

type workerReadResult struct {
	frame workerFrame
	err   error
}

// readWorkerOutput is the opposite direction from readWorkerFrame: the worker
// cannot OPEN a Service stream, and only it can announce a bounded RESULT.
func readWorkerOutput(input io.Reader) (workerFrame, error) {
	var header [9]byte
	if _, err := io.ReadFull(input, header[:]); err != nil {
		return workerFrame{}, err
	}
	kind, id, size := header[0], binary.BigEndian.Uint32(header[1:5]), binary.BigEndian.Uint32(header[5:])
	valid := id != 0 && (kind == 2 && size > 0 && size <= workerFrameLimit || kind == 3 && size == 4 || kind == 4 && size == 0 || kind == 5 && size == 1 || kind == 6 && size == 5)
	if !valid {
		return workerFrame{}, errors.New("text worker output frame is invalid")
	}
	frame := workerFrame{kind: kind, id: id, body: make([]byte, size)}
	if _, err := io.ReadFull(input, frame.body); err != nil {
		return workerFrame{}, err
	}
	return frame, nil
}
