package document

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

var (
	errWorkerRelayProtocol = errors.New("byte-stream worker framing did not complete")
	errWorkerRelayClosed   = errors.New("byte-stream worker attachment closed before completion")
	errWorkerRelayRead     = errors.New("byte-stream source read failed")
	errWorkerRelayWrite    = errors.New("byte-stream destination write failed")
)

type byteStreamFirstFailure struct{ cause error }

func (failure byteStreamFirstFailure) Error() string { return "original byte-stream exchange failed" }
func (failure byteStreamFirstFailure) Unwrap() error { return failure.cause }

// ByteStreamFailure projects fixed categories without exposing peer, path or bytes.
func ByteStreamFailure(err error) string {
	if first, ok := errors.AsType[byteStreamFirstFailure](err); ok {
		err = first.cause
	}
	switch {
	case errors.Is(err, errWorkerRelayClosed):
		return errWorkerRelayClosed.Error()
	case errors.Is(err, errWorkerRelayProtocol):
		return errWorkerRelayProtocol.Error()
	case errors.Is(err, errWorkerRelayRead):
		return errWorkerRelayRead.Error()
	case errors.Is(err, errWorkerRelayWrite):
		return errWorkerRelayWrite.Error()
	case errors.Is(err, context.DeadlineExceeded):
		return "byte-stream exchange exceeded its deadline"
	default:
		return "byte-stream worker handoff or exchange failed"
	}
}

// ByteStreamPort is a local or authenticated byte attachment with directional EOF.
// Its Close interrupts both pending reads and writes.
type ByteStreamPort interface {
	io.ReadWriteCloser
	CloseInput() error
}

// BridgeByteStreamWorker runs the fixed two-port exchange after verified worker
// initialization. Both ports and the worker attachment are borrowed for the
// exchange. The caller still owns authentication and terminal policy.
func BridgeByteStreamWorker(ctx context.Context, attachment io.ReadWriteCloser, first, second ByteStreamPort) (result error) {
	if ctx == nil || attachment == nil || first == nil || second == nil {
		return errors.New("byte-stream worker attachments are unavailable")
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	retired := make(chan struct{})
	stop := context.AfterFunc(child, func() {
		defer close(retired)
		_ = attachment.Close()
		_ = first.Close()
		_ = second.Close()
	})
	defer func() {
		if result == nil && stop() {
			result = attachment.Close()
			return
		}
		cancel()
		<-retired
	}()
	for _, id := range []uint32{1, 3} {
		if err := writeWorkerFrame(attachment, workerFrame{kind: 1, id: id}); err != nil {
			return err
		}
	}
	ports := [2]ByteStreamPort{first, second}
	incoming := [2]chan workerFrame{make(chan workerFrame, 2), make(chan workerFrame, 2)}
	credits := [2]chan uint32{make(chan uint32, 1), make(chan uint32, 1)}
	var writeMu sync.Mutex
	write := func(frame workerFrame) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return writeWorkerFrame(attachment, frame)
	}
	var firstFailure error
	var retainFailure sync.Once
	fail := func(err error) {
		if err != nil {
			retainFailure.Do(func() { firstFailure = err })
			cancel()
		}
	}
	results := make(chan error, 4)
	receiveDone := make(chan error, 1)
	finishing := make(chan struct{})
	go func() {
		for {
			frame, err := readWorkerFrame(attachment)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					err = errors.Join(errWorkerRelayProtocol, err)
				} else {
					err = errors.Join(errWorkerRelayClosed, err)
				}
				fail(err)
				receiveDone <- err
				return
			}
			if frame.kind == 5 {
				select {
				case <-finishing:
					if frame.id == 1 && frame.body[0] == 0 {
						receiveDone <- nil
						return
					}
				default:
				}
				fail(errWorkerRelayProtocol)
				receiveDone <- errWorkerRelayProtocol
				return
			}
			if frame.id != 1 && frame.id != 3 {
				fail(errWorkerRelayProtocol)
				receiveDone <- errWorkerRelayProtocol
				return
			}
			index := (frame.id - 1) / 2
			switch frame.kind {
			case 2, 4:
				select {
				case incoming[index] <- frame:
					continue
				default:
				}
			case 3:
				select {
				case credits[index] <- binary.BigEndian.Uint32(frame.body):
					continue
				default:
				}
			}
			fail(errWorkerRelayProtocol)
			receiveDone <- errWorkerRelayProtocol
			return
		}
	}()
	for index, port := range ports {
		id := uint32(2*index + 1)
		go func() {
			err := sendWorkerBytes(child, port, id, credits[index], write)
			fail(err)
			results <- err
		}()
		go func() {
			err := receiveWorkerBytes(child, port, incoming[index], write)
			fail(err)
			results <- err
		}()
	}
	for range 4 {
		result = errors.Join(result, <-results)
	}
	if result == nil && child.Err() == nil {
		close(finishing)
		result = write(workerFrame{kind: 5, id: 1, body: []byte{0}})
		if result != nil {
			cancel()
		}
		// Join the authenticated worker receipt before closing its socket.
		return errors.Join(result, <-receiveDone, ctx.Err())
	}
	cancel()
	result = errors.Join(result, <-receiveDone, ctx.Err())
	if firstFailure != nil {
		return errors.Join(byteStreamFirstFailure{cause: firstFailure}, result)
	}
	return result
}

func sendWorkerBytes(ctx context.Context, port io.Reader, id uint32, credit <-chan uint32, write func(workerFrame) error) error {
	var buffer [workerFrameLimit]byte
	for {
		n, err := port.Read(buffer[:])
		if n > 0 {
			if writeErr := write(workerFrame{kind: 2, id: id, body: buffer[:n]}); writeErr != nil {
				return errors.Join(errWorkerRelayProtocol, writeErr)
			}
			select {
			case acknowledged := <-credit:
				if acknowledged != uint32(n) {
					return errWorkerRelayProtocol
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err == io.EOF {
			return write(workerFrame{kind: 4, id: id})
		}
		if err != nil {
			return errors.Join(errWorkerRelayRead, err)
		}
	}
}

func receiveWorkerBytes(ctx context.Context, port ByteStreamPort, incoming <-chan workerFrame, write func(workerFrame) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame := <-incoming:
			if frame.kind == 4 {
				return port.CloseInput()
			}
			if err := writeWorkerBytes(port, frame.body); err != nil {
				return errors.Join(errWorkerRelayWrite, err)
			}
			credit := binary.BigEndian.AppendUint32(nil, uint32(len(frame.body)))
			if err := write(workerFrame{kind: 3, id: frame.id, body: credit}); err != nil {
				return err
			}
		}
	}
}
