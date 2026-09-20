//go:build linux

package route

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// Reserve bounded per-JOIN bookkeeping before handshake. Actual receive and
// write queues additionally debit the original Source/Responder as they grow.
const closedJoinedQueue = 4 << 10

// ErrClosedJoinPeerCleanupDeadline reports that the bounded wait for the
// peer's outer JOIN retirement expired after local inner cleanup completed.
var ErrClosedJoinPeerCleanupDeadline = errors.New("closed JOIN peer cleanup deadline exceeded")

// ClosedJoinedStream is the admitted framed stream returned only after JOIN.
// Endpoint may carry its independent Service TLS here; this is not Service
// authentication. The retained Source/Responder prefix owns its parent route.
type ClosedJoinedStream struct {
	*closedSourceLane
	hello               ClosedHello
	refillMu            sync.Mutex
	channels            *closedSourceChannels
	outer               *closedSourceLane
	context             context.Context
	finished            chan struct{}
	once                sync.Once
	closeErr, finishErr error
	retireOnce          sync.Once
	retireErr           error
}

// AuthenticatedPeerRetired reports only the clean CLOSE(0) decoded for this
// exact admitted child. A TLS owner can preserve that distinction after TLS
// maps its lower EOF to truncation; local close and raw Carrier loss are false.
func (stream *ClosedJoinedStream) AuthenticatedPeerRetired() bool {
	if stream == nil || stream.closedSourceLane == nil {
		return false
	}
	owner := stream.closedSourceLane.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return stream.closedSourceLane.remoteClosed && stream.closedSourceLane.failure == io.EOF
}

func newClosedJoinedStream(ctx context.Context, parent net.Conn, lane *closedSourceLane, release func()) *ClosedJoinedStream {
	// A successful JOIN result accepted the outer Source operation. From this
	// point its terminal status describes retirement of an admitted stream, not
	// refusal of an unopened child. Commit that transition before cancellation
	// can start the nested owner and retire the outer lane.
	lane.owner.mu.Lock()
	lane.closeStatus = 0
	lane.owner.mu.Unlock()
	owner := newClosedSourceChannelOwner(parent, lane.end, nil)
	owner.queueParent = lane.owner
	owner.last = 1
	owner.retainClosedRead = true
	owner.framedParent = lane
	owner.transferred = 3*closedLaneHeaderSize + 209 + 355 + 5 + closedLaneHeaderSize + 4096 + closedLaneHeaderSize + closedTerminalOperationSize
	joined := &closedSourceLane{owner: owner, id: 1, end: lane.end, readEnd: lane.end, writeEnd: lane.end, credit: 64 << 10, receiveCredit: 64 << 10, opened: true, active: true, closeStatus: 0}
	owner.lanes[1] = joined
	stream := &ClosedJoinedStream{closedSourceLane: joined, channels: owner, outer: lane, context: ctx, finished: make(chan struct{})}
	// A clean transport EOF must leave the outer lane live until the admitted
	// child's terminal cleanup is joined below. Every actual failure still
	// retires the outer lane immediately so it interrupts blocked physical I/O.
	owner.retire = stream.retireOuterOnFailure
	owner.start()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); owner.stop() })
	go func() {
		defer close(stream.finished)
		<-owner.done
		// Accepted bytes precede a transport EOF, even without inner CLOSE. Keep their
		// bounded reservation until consumed, caller close/cancel, or original expiry.
		for {
			owner.mu.Lock()
			retainPayload := len(joined.buffer) != 0 || joined.remoteClosed && joined.receivedData && !joined.terminalRead
			retain := owner.terminal == io.EOF && !joined.closed && (joined.failure == nil || joined.failure == io.EOF) &&
				retainPayload && ctx.Err() == nil && time.Now().Before(joined.end)
			changed := owner.changed
			owner.mu.Unlock()
			if !retain {
				break
			}
			timer := time.NewTimer(time.Until(joined.end))
			select {
			case <-changed:
			case <-ctx.Done():
			case <-timer.C:
			}
			timer.Stop()
		}
		// Join the admitted lane before retiring its outer transport. A clean
		// peer CLOSE can finish the reader concurrently with the caller's Close;
		// both paths share lane.closeOnce, so neither can tear down the parent
		// underneath the other's terminal cleanup.
		stream.finishErr = errors.Join(joined.Close(), owner.Close(), stream.retireOuter())
		if !stop() {
			<-interrupted
		}
		release()
	}()
	return stream
}

// CloseWrite ends only this direction. The opposite direction can still send
// bytes and return credit until the complete stream is closed or expires.
func (stream *ClosedJoinedStream) CloseWrite() error {
	lane := stream.closedSourceLane
	lane.writing.Lock()
	defer lane.writing.Unlock()
	lane.owner.mu.Lock()
	if lane.writeEOF {
		lane.owner.mu.Unlock()
		return nil
	}
	if err := lane.writeErrorLocked(false); err != nil {
		lane.owner.mu.Unlock()
		return err
	}
	request, err := lane.enqueueLocked(ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1}, time.Time{})
	if err == nil {
		lane.writeEOF = true
	}
	lane.owner.mu.Unlock()
	if err != nil {
		return err
	}
	return lane.owner.awaitWrite(request)
}

func (stream *ClosedJoinedStream) Close() error {
	if stream == nil {
		return nil
	}
	stream.once.Do(func() {
		stream.closeErr = stream.closedSourceLane.Close()
		if stream.closeErr == nil && stream.context.Err() == nil {
			stream.closeErr = stream.waitPeerClose()
		}
		stream.closeErr = errors.Join(stream.closeErr, stream.channels.Close())
		<-stream.finished
		stream.closeErr = errors.Join(stream.closeErr, stream.finishErr)
	})
	return stream.closeErr
}

// Completing the inner CLOSE write is not acknowledgement that Rendezvous has
// forwarded the final bytes and joined the pair. Let its role TLS/outer closure
// arrive before retiring our outer lane; otherwise cleanup cancels that work.
func (stream *ClosedJoinedStream) waitPeerClose() error {
	deadline := time.Now().Add(time.Second)
	if stream.outer.end.Before(deadline) {
		deadline = stream.outer.end
	}
	for {
		owner := stream.outer.owner
		owner.mu.Lock()
		ended := stream.outer.remoteClosed
		failure := owner.terminal
		if ended && stream.outer.failure != io.EOF {
			failure = errors.Join(failure, stream.outer.failure)
		}
		changed := owner.changed
		owner.mu.Unlock()
		if stream.context.Err() != nil {
			return nil
		}
		if failure != nil {
			return failure
		}
		if ended {
			return nil
		}
		timer := time.NewTimer(time.Until(deadline))
		select {
		case <-changed:
		case <-stream.context.Done():
			timer.Stop()
			return nil
		case <-timer.C:
			return ErrClosedJoinPeerCleanupDeadline
		}
		timer.Stop()
	}
}

func (stream *ClosedJoinedStream) retireOuter() error {
	stream.retireOnce.Do(func() {
		stream.channels.mu.Lock()
		cleanTLS := stream.channels.terminal == io.EOF
		stream.channels.mu.Unlock()
		if cleanTLS && stream.context.Err() == nil {
			stream.retireErr = stream.waitPeerClose()
		}
		stream.retireErr = errors.Join(stream.retireErr, stream.outer.Close())
	})
	return stream.retireErr
}

func (stream *ClosedJoinedStream) retireOuterOnFailure() error {
	stream.channels.mu.Lock()
	cause := stream.channels.terminal
	cleanPeerClose := cause == io.EOF && stream.closedSourceLane.remoteClosed && stream.closedSourceLane.failure == io.EOF
	stream.channels.mu.Unlock()
	if cause == nil || cleanPeerClose {
		return nil
	}
	return stream.retireOuter()
}
