//go:build linux

package textdocument

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
)

type publisherEvent struct {
	stream  *publisherStream
	kind    byte // request=1, terminal=2, write=3, joined close=4
	body    []byte
	outcome connection.Outcome
	eof     bool
	err     error
}

type publisherWrite struct {
	body []byte
	eof  bool
}

type publisherConnections struct {
	ctx            context.Context
	cancel         context.CancelFunc
	attachment     io.ReadWriteCloser
	attachmentOnce sync.Once
	attachmentErr  error
	frames         chan workerReadResult
	workerDone     chan struct{}
	events         chan publisherEvent
	streams        []*publisherStream
	byID           map[uint32]*publisherStream
	lastID         uint32
	active         int
	cleanupErr     error
	closers        sync.WaitGroup
}

func newPublisherConnections(parent context.Context, attachment io.ReadWriteCloser) *publisherConnections {
	ctx, cancel := context.WithCancel(parent)
	owner := &publisherConnections{ctx: ctx, cancel: cancel, attachment: attachment,
		frames: make(chan workerReadResult, 1), events: make(chan publisherEvent), workerDone: make(chan struct{}),
		byID: make(map[uint32]*publisherStream)}
	go func() {
		defer close(owner.workerDone)
		for {
			frame, err := readWorkerOutput(attachment)
			select {
			case owner.frames <- workerReadResult{frame: frame, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return owner
}

func (owner *publisherConnections) closeAttachment() error {
	owner.attachmentOnce.Do(func() { owner.attachmentErr = owner.attachment.Close() })
	return owner.attachmentErr
}

func (owner *publisherConnections) close() error {
	owner.cancel()
	for _, stream := range owner.streams {
		stream.io.close()
	}
	attachmentErr := owner.closeAttachment()
	for _, stream := range owner.streams {
		<-stream.io.closed
		owner.cleanupErr = errors.Join(owner.cleanupErr, stream.io.cleanupErr)
		clear(stream.queue)
	}
	owner.closers.Wait()
	<-owner.workerDone
	return errors.Join(attachmentErr, owner.cleanupErr)
}

type publisherStreamIO struct {
	ctx        context.Context
	cancel     context.CancelFunc
	owner      *publisherConnections
	state      *publisherStream
	service    connection.Stream
	writes     chan publisherWrite
	workers    sync.WaitGroup
	closeOnce  sync.Once
	closed     chan struct{}
	closeErr   error // Retained complete Service result, including logical refusal.
	cleanupErr error // Joined physical result reported to worker cleanup.
}

func (owner *publisherConnections) startStream(service connection.Stream) *publisherStream {
	ctx, cancel := context.WithCancel(owner.ctx)
	state := &publisherStream{}
	stream := &publisherStreamIO{ctx: ctx, cancel: cancel, owner: owner, state: state, service: service,
		writes: make(chan publisherWrite, 1), closed: make(chan struct{})}
	state.io = stream
	stream.workers.Add(2)
	go stream.read()
	go stream.write()
	return state
}

func (stream *publisherStreamIO) send(event publisherEvent) bool {
	event.stream = stream.state
	select {
	case stream.owner.events <- event:
		return true
	case <-stream.ctx.Done():
		return false
	}
}

func (stream *publisherStreamIO) read() {
	defer stream.workers.Done()
	// One extra byte distinguishes an oversized request without reading or
	// queuing arbitrary Service input. Idle Connections hold no response queue.
	var request [requestBytes + 1]byte
	n, err := io.ReadFull(stream.service, request[:])
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	if !stream.send(publisherEvent{kind: 1, body: request[:n], err: err}) || err != nil {
		return
	}
	select {
	case outcome, ok := <-stream.service.Done():
		if !ok {
			stream.send(publisherEvent{kind: 2, err: errors.New("text Publisher Service terminal is absent")})
			return
		}
		stream.send(publisherEvent{kind: 2, outcome: outcome})
	case <-stream.ctx.Done():
	}
}

func (stream *publisherStreamIO) write() {
	defer stream.workers.Done()
	for {
		select {
		case output := <-stream.writes:
			var err error
			if output.eof {
				err = stream.service.CloseInput()
			} else {
				err = writeWorkerBytes(stream.service, output.body)
			}
			if !stream.send(publisherEvent{kind: 3, eof: output.eof, err: err}) || err != nil || output.eof {
				return
			}
		case <-stream.ctx.Done():
			return
		}
	}
}

func (stream *publisherStreamIO) close() {
	// Refusal and owner cancellation interrupt even a completed stream already
	// waiting for its original control tail. Waiting never delays revoke.
	stream.cancel()
	stream.startClose(false)
}

func (stream *publisherStreamIO) finish() {
	stream.startClose(true)
}

func (stream *publisherStreamIO) startClose(completed bool) {
	stream.closeOnce.Do(func() {
		stream.owner.closers.Add(1)
		go func() {
			defer stream.owner.closers.Done()
			if completed {
				// Application Done is distinct from original physical completion.
				// A recovery-enabled Service retains its final control proofs until
				// the same peer closes or its original bound ends. Keep this borrow
				// and worker cleanup capacity; a notification grants no authority.
				if tail, ok := stream.service.(interface{ Joined() <-chan struct{} }); ok {
					select {
					case <-tail.Joined():
					case <-stream.ctx.Done():
					}
				}
			}
			stream.cancel()
			stream.closeErr = stream.service.Close()
			stream.workers.Wait()
			stream.cleanupErr = stream.closeErr
			if joined, ok := stream.service.(interface {
				Joined() <-chan struct{}
				JoinedCleanup() error
			}); ok {
				// Close has joined the same original Service. Its logical result
				// remains retained above; worker cleanup receives only actual
				// physical failure, not ordinary control-tail cancellation.
				stream.cleanupErr = joined.JoinedCleanup()
			}
			// Publish cleanup before notification; the owner joins this channel
			// during global cancellation, when no actor receives events anymore.
			close(stream.closed)
			select {
			case stream.owner.events <- publisherEvent{stream: stream.state, kind: 4, err: stream.cleanupErr}:
			case <-stream.owner.ctx.Done():
			}
		}()
	})
}
