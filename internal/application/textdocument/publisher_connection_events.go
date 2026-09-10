//go:build linux

package textdocument

import (
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

func (owner *publisherConnections) fromService(event publisherEvent) error {
	stream := event.stream
	switch event.kind {
	case 1:
		if event.err != nil {
			return owner.failStream(stream)
		}
		stream.request, stream.requestReady = event.body, true
	case 2:
		if event.err != nil || event.outcome.Class != connection.CleanClose {
			return owner.failStream(stream)
		}
		stream.remoteClean = true
	case 3:
		if !stream.writePending {
			return errors.New("text Publisher write completion is unsolicited")
		}
		released := stream.writeSize
		stream.writePending, stream.writeSize = false, 0
		if err := owner.releaseCredit(stream, released); err != nil {
			return err
		}
		if event.err != nil {
			return owner.failStream(stream)
		}
		if event.eof && !stream.failed {
			stream.responseEOF = true
		}
	case 4:
		stream.ioClosed = true
		if event.err != nil {
			owner.cleanupErr = event.err
			return errors.New("text Publisher Service cleanup failed")
		}
		if stream.writePending {
			released := stream.writeSize
			stream.writePending, stream.writeSize = false, 0
			return owner.releaseCredit(stream, released)
		}
	default:
		return errors.New("text Publisher Service event is invalid")
	}
	return nil
}

func (owner *publisherConnections) failStream(stream *publisherStream) error {
	stream.failed = true
	stream.request = nil
	stream.io.close()
	released := uint32(stream.queued)
	stream.queued = 0
	clear(stream.queue)
	return owner.releaseCredit(stream, released)
}

func (owner *publisherConnections) fromWorker(frame workerFrame) error {
	stream := owner.byID[frame.id]
	if stream == nil || stream.workerEOF || stream.workerClosed {
		return errors.New("text Publisher worker emitted an unsolicited stream")
	}
	switch frame.kind {
	case 2:
		size := uint32(len(frame.body))
		if size > stream.receiveCredit || size > MaximumBytes+13-stream.responseBytes {
			return errors.New("text Publisher response exceeds its bound")
		}
		stream.receiveCredit -= size
		stream.responseBytes += size
		if stream.failed {
			// The failed Service no longer consumes output. Drain only this
			// already-authorized bounded response to its worker terminal barrier.
			return owner.releaseCredit(stream, size)
		}
		if len(frame.body) > len(stream.queue)-stream.queued {
			return errors.New("text Publisher response queue exceeds its bound")
		}
		tail := (stream.head + stream.queued) % len(stream.queue)
		first := copy(stream.queue[tail:], frame.body)
		copy(stream.queue, frame.body[first:])
		stream.queued += len(frame.body)
	case 3:
		credit := binary.BigEndian.Uint32(frame.body)
		if credit == 0 || credit > workerCredit || stream.sendCredit > workerCredit-credit {
			return errors.New("text Publisher request credit is invalid")
		}
		stream.sendCredit += credit
	case 4:
		if stream.responseBytes < 13 {
			return errors.New("text Publisher response is incomplete")
		}
		stream.workerEOF = true
	case 5:
		if frame.body[0] > 6 {
			return errors.New("text Publisher worker terminal is invalid")
		}
		// Worker CLOSE is a per-stream refusal, never semantic success. It is
		// an ordered barrier after earlier output; no CLOSE acknowledgement or
		// new wire authority is invented for cancellation.
		stream.workerEOF = true
		return owner.failStream(stream)
	default:
		return errors.New("text Publisher worker frame direction is invalid")
	}
	return nil
}

func (owner *publisherConnections) releaseCredit(stream *publisherStream, released uint32) error {
	if released == 0 || stream.id == 0 || stream.workerEOF || stream.workerClosed {
		return nil
	}
	if released > workerCredit || stream.receiveCredit > workerCredit-released {
		return errors.New("text Publisher released credit exceeds its bound")
	}
	stream.receiveCredit += released
	var body [4]byte
	binary.BigEndian.PutUint32(body[:], released)
	return owner.frame(workerFrame{kind: 3, id: stream.id, body: body[:]})
}
