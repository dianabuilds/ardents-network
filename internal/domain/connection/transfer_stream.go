package connection

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/transfer"
)

// SegmentOpener performs one fresh admission under the calling Endpoint's
// original transfer. The release closes the exact segment and its Source.
type SegmentOpener func(context.Context, *Stream) (*Stream, func() error, error)

// Transfer retains one Application interface while the independently admitted
// Service Connections it uses join and succeed each other.
type Transfer struct {
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	current  *Stream
	changed  chan struct{}
	done     chan endpointstream.Outcome
	joined   chan struct{}
	failure  error
	readMu   sync.Mutex
	readEOF  bool
	writeMu  sync.Mutex
	writeEOF bool
}

// OpenTransfer takes one active segment and its release on success.
func OpenTransfer(ctx context.Context, first *Stream, release func() error, open SegmentOpener) (*Transfer, error) {
	if ctx == nil || ctx.Err() != nil || first == nil || first.self != first || first.segment == nil ||
		first.segment.base.Next != 1 || release == nil || open == nil {
		return nil, errContinuationRefused
	}
	lifetime, cancel := context.WithCancel(ctx)
	owner := &Transfer{
		ctx: lifetime, cancel: cancel, current: first, changed: make(chan struct{}),
		done: make(chan endpointstream.Outcome, 1), joined: make(chan struct{}),
	}
	go owner.run(first, release, open)
	return owner, nil
}

func (owner *Transfer) run(current *Stream, release func() error, open SegmentOpener) {
	original := current.owner
	var result error
	for {
		var outcome endpointstream.Outcome
		var final bool
		select {
		case <-owner.ctx.Done():
			result = owner.ctx.Err()
			final = true
		case outcome, final = <-current.Done():
			if final && outcome.Class == endpointstream.CleanClose {
				select {
				case <-current.Joined():
				case <-owner.ctx.Done():
					result = owner.ctx.Err()
				}
			}
		}
		result = errors.Join(result, release(), current.JoinedCleanup(), owner.ctx.Err())
		if result != nil || final {
			if result == nil && outcome.Class != endpointstream.CleanClose {
				result = errIntegrity
			}
			break
		}
		if current.segmentBoundary == nil {
			result = errContinuationRefused
			break
		}
		previous := current
		current, release, result = open(owner.ctx, previous)
		if result != nil {
			break
		}
		if current == nil || current.self != current || current.owner == nil || current.segment == nil ||
			current.owner.operation != original.operation || current.owner.role != original.role ||
			!sameContinuationService(original.identity, current.owner.identity) ||
			current.segment.base != previous.segmentBoundary.boundary || release == nil {
			result = errContinuationRefused
			if release != nil {
				result = errors.Join(result, release())
			}
			break
		}
		owner.mu.Lock()
		owner.current = current
		close(owner.changed)
		owner.changed = make(chan struct{})
		owner.mu.Unlock()
	}
	owner.mu.Lock()
	owner.failure = result
	owner.current = nil
	close(owner.changed)
	owner.mu.Unlock()
	if result == nil {
		owner.done <- endpointstream.Outcome{Class: endpointstream.CleanClose}
	} else {
		owner.done <- endpointstream.Outcome{Class: endpointstream.IndeterminateFailure, Reason: "Service transfer did not complete"}
	}
	close(owner.done)
	close(owner.joined)
	owner.cancel()
}

func (owner *Transfer) segment() (*Stream, <-chan struct{}, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.current == nil {
		if owner.failure != nil {
			return nil, nil, owner.failure
		}
		return nil, nil, io.EOF
	}
	return owner.current, owner.changed, nil
}

func (owner *Transfer) awaitSegment(changed <-chan struct{}) error {
	select {
	case <-changed:
		return nil
	case <-owner.ctx.Done():
		// A simultaneous final publish decides the transfer result.
		select {
		case <-changed:
			return nil
		default:
			return owner.ctx.Err()
		}
	}
}

func (owner *Transfer) Read(destination []byte) (int, error) {
	owner.readMu.Lock()
	defer owner.readMu.Unlock()
	if owner.readEOF {
		return 0, io.EOF
	}
	for {
		segment, changed, err := owner.segment()
		if err != nil {
			return 0, err
		}
		n, err := segment.Read(destination)
		if !errors.Is(err, transfer.ErrSegmentDraining) {
			owner.readEOF = err == io.EOF
			return n, err
		}
		if err := owner.awaitSegment(changed); err != nil {
			return 0, err
		}
	}
}

func (owner *Transfer) Write(body []byte) (int, error) {
	owner.writeMu.Lock()
	defer owner.writeMu.Unlock()
	if owner.writeEOF {
		return 0, io.ErrClosedPipe
	}
	accepted := 0
	for accepted < len(body) {
		segment, changed, err := owner.segment()
		if err != nil {
			if err == io.EOF {
				err = io.ErrClosedPipe
			}
			return accepted, err
		}
		n, err := segment.Write(body[accepted:])
		accepted += n
		if !errors.Is(err, transfer.ErrSegmentDraining) {
			return accepted, err
		}
		if accepted == len(body) {
			return accepted, nil
		}
		if err := owner.awaitSegment(changed); err != nil {
			return accepted, err
		}
	}
	return accepted, nil
}

func (owner *Transfer) CloseInput() error {
	owner.writeMu.Lock()
	defer owner.writeMu.Unlock()
	if owner.writeEOF {
		return nil
	}
	for {
		segment, changed, err := owner.segment()
		if err != nil {
			if err == io.EOF {
				err = io.ErrClosedPipe
			}
			return err
		}
		err = segment.CloseInput()
		if !errors.Is(err, transfer.ErrSegmentDraining) {
			owner.writeEOF = err == nil
			return err
		}
		if err := owner.awaitSegment(changed); err != nil {
			return err
		}
	}
}

func (owner *Transfer) Done() <-chan endpointstream.Outcome { return owner.done }

func (owner *Transfer) Close() error {
	owner.cancel()
	<-owner.joined
	return owner.failure
}
