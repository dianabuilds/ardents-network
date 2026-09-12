//go:build linux

package textdocument

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"unicode/utf8"
)

const workerFrameLimit = 16 << 10

// workerInitialization is volatile, destination-free input. Its nonce is a
// job correlation guard, never evidence that a launch is confined.
type workerInitialization struct {
	mode     WorkerMode
	nonce    [32]byte
	digest   [32]byte
	snapshot []byte
}

func readWorkerInitialization(input io.Reader, expected WorkerMode) (workerInitialization, error) {
	var header [77]byte
	if input == nil || (expected != ReaderWorker && expected != PublisherWorker) {
		return workerInitialization{}, errors.New("text worker initialization is invalid")
	}
	if _, err := io.ReadFull(input, header[:]); err != nil {
		return workerInitialization{}, err
	}
	length := binary.BigEndian.Uint32(header[41:45])
	if string(header[:8]) != "ARDTWP01" || WorkerMode(header[8]) != expected || length > MaximumBytes || (expected == ReaderWorker && length != 0) {
		return workerInitialization{}, errors.New("text worker initialization is invalid")
	}
	initial := workerInitialization{mode: expected}
	copy(initial.nonce[:], header[9:41])
	copy(initial.digest[:], header[45:77])
	if initial.nonce == [32]byte{} {
		return workerInitialization{}, errors.New("text worker identity is invalid")
	}
	initial.snapshot = make([]byte, length)
	if _, err := io.ReadFull(input, initial.snapshot); err != nil {
		return workerInitialization{}, err
	}
	if !utf8.Valid(initial.snapshot) || sha256.Sum256(initial.snapshot) != initial.digest {
		return workerInitialization{}, errors.New("text worker snapshot is invalid")
	}
	return initial, nil
}

func writeWorkerReadiness(output io.Writer, initial workerInitialization) error {
	var ready [72]byte
	copy(ready[:], "ARDTWR01")
	copy(ready[8:40], initial.nonce[:])
	copy(ready[40:], initial.digest[:])
	return writeWorkerBytes(output, ready[:])
}

type workerFrame struct {
	current *workerStream
	kind    byte
	id      uint32
	body    []byte
}

func readWorkerFrame(input io.Reader) (workerFrame, error) {
	var header [9]byte
	if _, err := io.ReadFull(input, header[:]); err != nil {
		return workerFrame{}, err
	}
	kind, id, size := header[0], binary.BigEndian.Uint32(header[1:5]), binary.BigEndian.Uint32(header[5:])
	valid := id != 0 && ((kind == 1 || kind == 4) && size == 0 || kind == 2 && size > 0 && size <= workerFrameLimit || kind == 3 && size == 4 || kind == 5 && size == 1)
	if !valid {
		return workerFrame{}, errors.New("text worker frame is invalid")
	}
	frame := workerFrame{kind: kind, id: id, body: make([]byte, size)}
	if _, err := io.ReadFull(input, frame.body); err != nil {
		return workerFrame{}, err
	}
	return frame, nil
}

func writeWorkerFrame(output io.Writer, frame workerFrame) error {
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
