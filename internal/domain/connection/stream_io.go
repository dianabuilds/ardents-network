package connection

import (
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/transfer"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

func (stream *Stream) Read(destination []byte) (int, error) {
	defer measurement.Begin(runtimeevent.ConnectionRead).End()

	if stream == nil || stream.self != stream {
		return 0, errIntegrity
	}
	// Caller memory is an Application handoff. Keep bytes private until the
	// original owners' final local fences accept this exact read transition.
	staged := make([]byte, min(len(destination), wire.MaximumDataBytes))
	defer clear(staged)
	for {
		var n int
		var readErr error
		var changed <-chan struct{}
		done := false
		err := stream.commitHandoff(func() error {
			changed = stream.changeLocked()
			n, readErr = stream.transfer.Peek(staged)
			if n == 0 && readErr == nil && stream.segment != nil && stream.segment.complete(stream.transfer.Generation()) {
				return transfer.ErrSegmentDraining
			}
			done = n != 0 || readErr != nil || len(destination) == 0
			return nil
		}, func() {
			n, readErr = stream.transfer.Read(staged)
			n, readErr = deliverRead(destination, staged, n, readErr, nil)
			if done {
				stream.signalLocked()
			}
		})
		if err != nil {
			return deliverRead(destination, staged, n, readErr, err)
		}
		if done {
			// Preserve the exact authenticated EOF sentinel for io.Reader.
			return n, readErr
		}
		stream.waitChange(changed)
	}
}

// deliverRead cannot grant authority; only Read supplies the actual commit
// result. A failed final fence exposes neither a positive count nor plaintext.
func deliverRead(destination, staged []byte, n int, readErr, handoffErr error) (int, error) {
	if handoffErr != nil {
		return 0, errors.Join(readErr, handoffErr)
	}
	if n < 0 || n > len(staged) || n > len(destination) {
		return 0, errIntegrity
	}
	copy(destination, staged[:n])
	return n, readErr
}

func (stream *Stream) Write(body []byte) (int, error) {
	defer measurement.Begin(runtimeevent.ConnectionWrite).End()

	if stream == nil || stream.self != stream {
		return 0, errIntegrity
	}
	stream.writeMu.Lock()
	defer stream.writeMu.Unlock()
	accepted := 0
	for {
		var changed <-chan struct{}
		var stagedSize int
		var deliveryErr error
		err := stream.commitHandoff(func() error {
			if stream.settled || stream.transfer.InputClosed() {
				return io.ErrClosedPipe
			}
			if accepted == len(body) {
				return nil
			}
			size := min(len(body)-accepted, wire.MaximumDataBytes)
			err := stream.transfer.CheckWrite(body[accepted : accepted+size])
			if errors.Is(err, transfer.ErrBackpressure) {
				changed = stream.changeLocked()
				return nil
			}
			if err != nil {
				if !errors.Is(err, transfer.ErrSegmentDraining) {
					stream.failLocked(err)
				}
				return err
			}
			stagedSize = size
			return nil
		}, func() {
			if stagedSize == 0 {
				return
			}
			deliveryErr = stream.transfer.Write(body[accepted : accepted+stagedSize])
			if deliveryErr != nil {
				stream.failLocked(deliveryErr)
				return
			}
			accepted += stagedSize
			stream.signalLocked()
		})
		err = errors.Join(err, deliveryErr)
		if err != nil || accepted == len(body) {
			return accepted, err
		}
		if changed != nil {
			stream.waitChange(changed)
		}
	}
}

func (stream *Stream) CloseInput() error {
	if stream == nil || stream.self != stream {
		return errIntegrity
	}
	stream.writeMu.Lock()
	defer stream.writeMu.Unlock()
	return stream.commit(func() error {
		if err := stream.transfer.CloseInput(); err != nil {
			if !errors.Is(err, transfer.ErrSegmentDraining) {
				stream.failLocked(err)
			}
			return err
		}
		stream.signalLocked()
		return nil
	})
}
