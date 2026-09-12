//go:build linux

package textdocument

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

type readerServiceEvent struct {
	body     []byte
	eof      bool
	terminal bool
	outcome  connection.Outcome
	err      error
}

type readerWriteCompletion struct {
	size uint32
	eof  bool
	err  error
}

// readerConnectionIO owns only the already-admitted Service stream and the
// initialized local attachment. It cannot select a destination or open a Route.
type readerConnectionIO struct {
	ctx           context.Context
	cancel        context.CancelFunc
	attachment    io.ReadWriteCloser
	service       connection.Stream
	frames        chan workerReadResult
	serviceEvents chan readerServiceEvent
	permits       chan int
	writes        chan workerFrame
	written       chan readerWriteCompletion
	workers       sync.WaitGroup
	closeOnce     sync.Once
	closeErr      error
}

func startReaderConnectionIO(parent context.Context, attachment io.ReadWriteCloser, service connection.Stream) *readerConnectionIO {
	ctx, cancel := context.WithCancel(parent)
	owner := &readerConnectionIO{ctx: ctx, cancel: cancel, attachment: attachment, service: service,
		frames: make(chan workerReadResult, 1), serviceEvents: make(chan readerServiceEvent, 1), permits: make(chan int, 1),
		writes: make(chan workerFrame, 8), written: make(chan readerWriteCompletion, 8)}
	owner.workers.Add(3)
	go owner.readWorker()
	go owner.readService()
	go owner.writeService()
	return owner
}

func (owner *readerConnectionIO) close() error {
	owner.closeOnce.Do(func() {
		owner.cancel()
		// Either Close may join I/O that needs the other attachment interrupted.
		// Start both, then join both closers and all three transfer goroutines.
		var closers sync.WaitGroup
		var attachmentErr, serviceErr error
		closers.Add(2)
		go func() { defer closers.Done(); attachmentErr = owner.attachment.Close() }()
		go func() { defer closers.Done(); serviceErr = owner.service.Close() }()
		closers.Wait()
		owner.workers.Wait()
		owner.closeErr = errors.Join(attachmentErr, serviceErr)
	})
	return owner.closeErr
}

func (owner *readerConnectionIO) readWorker() {
	defer owner.workers.Done()
	for {
		frame, err := readWorkerOutput(owner.attachment)
		select {
		case owner.frames <- workerReadResult{frame: frame, err: err}:
		case <-owner.ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func (owner *readerConnectionIO) readService() {
	defer owner.workers.Done()
	for {
		var limit int
		select {
		case limit = <-owner.permits:
		case <-owner.ctx.Done():
			return
		}
		body := make([]byte, limit)
		n, err := owner.service.Read(body)
		if n < 0 || n > len(body) {
			owner.sendService(readerServiceEvent{err: errors.New("service returned an invalid read")})
			return
		}
		if n > 0 && !owner.sendService(readerServiceEvent{body: body[:n]}) {
			return
		}
		if err != nil {
			if err != io.EOF {
				owner.sendService(readerServiceEvent{err: errors.New("service read was interrupted")})
				return
			}
			if !owner.sendService(readerServiceEvent{eof: true}) {
				return
			}
			select {
			case outcome, ok := <-owner.service.Done():
				if !ok {
					owner.sendService(readerServiceEvent{err: errors.New("service terminal is absent")})
					return
				}
				owner.sendService(readerServiceEvent{terminal: true, outcome: outcome})
			case <-owner.ctx.Done():
			}
			return
		}
		if n == 0 {
			owner.sendService(readerServiceEvent{err: io.ErrNoProgress})
			return
		}
	}
}

func (owner *readerConnectionIO) sendService(event readerServiceEvent) bool {
	select {
	case owner.serviceEvents <- event:
		return true
	case <-owner.ctx.Done():
		return false
	}
}

func (owner *readerConnectionIO) writeService() {
	defer owner.workers.Done()
	for {
		var frame workerFrame
		select {
		case frame = <-owner.writes:
		case <-owner.ctx.Done():
			return
		}
		completion := readerWriteCompletion{size: uint32(len(frame.body)), eof: frame.kind == 4}
		if completion.eof {
			completion.err = owner.service.CloseInput()
		} else {
			completion.err = writeWorkerBytes(owner.service, frame.body)
		}
		select {
		case owner.written <- completion:
		case <-owner.ctx.Done():
			return
		}
		if completion.err != nil || completion.eof {
			return
		}
	}
}
