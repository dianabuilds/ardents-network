//go:build linux

package textdocument

import (
	"context"
	"errors"
	"io"
	"math"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

const publisherOpenLimit = 256
const publisherActiveLimit = 64

// ServeWorkerConnections transfers already-admitted Service Connections to one
// initialized Publisher worker. It never opens a Route or selects a destination.
// Ownership of a stream transfers only when it is received from incoming; the
// sender retains streams still waiting there. Closing incoming drains received
// streams. Cancellation interrupts and joins all received stream I/O.
// Endpoint still owns current-job/Grant validation and joined cgroup cleanup.
func ServeWorkerConnections(ctx context.Context, attachment io.ReadWriteCloser, incoming <-chan connection.Stream) (resultErr error) {
	if ctx == nil || attachment == nil || incoming == nil {
		return errors.New("text Publisher Connections are unavailable")
	}
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), attachment.Close())
	}
	owner := newPublisherConnections(ctx, attachment)
	callbackDone := make(chan struct{})
	stop := context.AfterFunc(owner.ctx, func() { defer close(callbackDone); _ = owner.closeAttachment() })
	defer func() {
		resultErr = errors.Join(resultErr, owner.close(), ctx.Err())
		if !stop() {
			<-callbackDone
		}
	}()
	for {
		if err := owner.advance(); err != nil {
			return err
		}
		if incoming == nil && len(owner.streams) == 0 {
			return nil
		}
		var admissions <-chan connection.Stream
		if len(owner.streams) < publisherOpenLimit {
			admissions = incoming
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case stream, ok := <-admissions:
			if !ok {
				incoming = nil
				continue
			}
			if stream == nil {
				return errors.New("text Publisher received an absent Connection")
			}
			owner.streams = append(owner.streams, owner.startStream(stream))
		case event := <-owner.events:
			if err := owner.fromService(event); err != nil {
				return err
			}
		case event := <-owner.frames:
			if event.err != nil {
				return errors.New("text Publisher worker exchange was interrupted")
			}
			if err := owner.fromWorker(event.frame); err != nil {
				return err
			}
		}
	}
}

type publisherStream struct {
	io            *publisherStreamIO
	id            uint32
	request       []byte
	requestReady  bool
	failed        bool
	ioClosed      bool
	workerClosed  bool
	workerEOF     bool
	remoteClean   bool
	responseEOF   bool
	writePending  bool
	writeSize     uint32
	receiveCredit uint32
	sendCredit    uint32
	responseBytes uint32
	queue         []byte
	head          int
	queued        int
}

func (owner *publisherConnections) advance() error {
	if owner.ctx.Err() != nil {
		return owner.ctx.Err()
	}
	for index := 0; index < len(owner.streams); {
		stream := owner.streams[index]
		if stream.ioClosed && (stream.id == 0 || stream.workerClosed) {
			if stream.id != 0 {
				delete(owner.byID, stream.id)
				owner.active--
			}
			clear(stream.queue)
			owner.streams = append(owner.streams[:index], owner.streams[index+1:]...)
			continue
		}
		if stream.id == 0 && stream.requestReady && !stream.failed && owner.active < publisherActiveLimit {
			if owner.lastID > math.MaxUint32-2 {
				return errors.New("text Publisher stream IDs are exhausted")
			}
			if owner.lastID == 0 {
				owner.lastID = 1
			} else {
				owner.lastID += 2
			}
			stream.id = owner.lastID
			stream.receiveCredit, stream.sendCredit = workerCredit, workerCredit-uint32(len(stream.request))
			stream.queue = make([]byte, workerCredit)
			owner.byID[stream.id] = stream
			owner.active++
			if err := owner.frame(workerFrame{kind: 1, id: stream.id}); err != nil {
				return err
			}
			if len(stream.request) > 0 {
				if err := owner.frame(workerFrame{kind: 2, id: stream.id, body: stream.request}); err != nil {
					return err
				}
			}
			stream.request = nil
			if err := owner.frame(workerFrame{kind: 4, id: stream.id}); err != nil {
				return err
			}
		}
		if stream.id != 0 && !stream.failed && !stream.writePending && !stream.responseEOF {
			if stream.queued > 0 {
				size := min(stream.queued, workerFrameLimit)
				body := make([]byte, size)
				first := copy(body, stream.queue[stream.head:min(stream.head+size, len(stream.queue))])
				copy(body[first:], stream.queue[:size-first])
				stream.head = (stream.head + size) % len(stream.queue)
				stream.queued -= size
				stream.writePending, stream.writeSize = true, uint32(size)
				stream.io.writes <- publisherWrite{body: body}
			} else if stream.workerEOF {
				stream.writePending = true
				stream.io.writes <- publisherWrite{eof: true}
			}
		}
		if stream.workerEOF && !stream.workerClosed && (stream.failed || stream.responseEOF && stream.remoteClean) {
			terminal := byte(0)
			if stream.failed {
				terminal = 1
			}
			if err := owner.frame(workerFrame{kind: 5, id: stream.id, body: []byte{terminal}}); err != nil {
				return err
			}
			stream.workerClosed = true
			stream.io.close()
		}
		index++
	}
	return nil
}

func (owner *publisherConnections) frame(frame workerFrame) error {
	if owner.ctx.Err() != nil {
		return owner.ctx.Err()
	}
	if err := writeWorkerFrame(owner.attachment, frame); err != nil {
		return errors.New("text Publisher worker forwarding was interrupted")
	}
	return nil
}
