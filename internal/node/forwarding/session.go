package forwarding

import (
	"context"
	"crypto/rand"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"sync"
	"time"
)

// sessionSet gives every retained Carrier one reader and one
// serialized writer. Its child IDs are local to that Carrier, so source lane
// IDs from different admitted channels can never collide on a reused leg.
type sessionSet struct {
	mu         sync.Mutex
	sessions   map[routecarrier.ClosedCarrierKey]*session
	pending    map[routecarrier.ClosedCarrierKey]*pendingSession
	readers    sync.WaitGroup
	cleanupErr error
}

// pendingSession gives one caller ownership of a new outer
// handshake. It publishes its terminal result before waking same-key waiters.
type pendingSession struct {
	done    chan struct{}
	session *session
	err     error
}

type session struct {
	owner       *sessionSet
	key         routecarrier.ClosedCarrierKey
	carrier     routecarrier.Carrier
	binding     *routecarrier.ClosedCarrierLease
	invalidate  func() error
	mu          sync.Mutex
	writer      chan struct{}
	done        chan struct{}
	children    map[uint32]*frameQueue
	queues      map[uint32]func(ardp.Frame) error
	retirements map[uint32]func() bool
	retired     map[uint32]struct{}
	lastOdd     uint32
	closed      bool
}

func newSessionSet() *sessionSet {
	return &sessionSet{sessions: make(map[routecarrier.ClosedCarrierKey]*session), pending: make(map[routecarrier.ClosedCarrierKey]*pendingSession)}
}

func (sessions *sessionSet) acquire(ctx context.Context, key routecarrier.ClosedCarrierKey, binding *routecarrier.ClosedCarrierLease, deadline time.Time, hello func() (ardp.Hello, error)) (*session, error) {
	if sessions == nil || ctx == nil || binding == nil || hello == nil {
		return nil, errors.New("closed forwarding Carrier session is unavailable")
	}
	carrier, err := binding.Carrier()
	if err != nil {
		return nil, err
	}
	for {
		sessions.mu.Lock()
		existing := sessions.sessions[key]
		if existing == nil {
			if pending := sessions.pending[key]; pending != nil {
				sessions.mu.Unlock()
				select {
				case <-pending.done:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				if pending.session != nil {
					pending.session.mu.Lock()
					current := !pending.session.closed && pending.session.binding.SameCarrier(binding)
					pending.session.mu.Unlock()
					if current {
						return pending.session, nil
					}
				}
				if pending.err != nil {
					return nil, pending.err
				}
				continue
			}
			pending := &pendingSession{done: make(chan struct{})}
			sessions.pending[key] = pending
			sessions.mu.Unlock()
			return sessions.open(ctx, key, binding, carrier, deadline, hello, pending)
		}
		existing.mu.Lock()
		current := !existing.closed && existing.binding.SameCarrier(binding)
		existing.mu.Unlock()
		if current {
			sessions.mu.Unlock()
			return existing, nil
		}
		delete(sessions.sessions, key)
		sessions.mu.Unlock()
		existing.fail()
	}

	// The new-session owner performs all outer HELLO I/O without sessions.mu.
	// Only exact-key waiters join its published terminal result.
}

func (sessions *sessionSet) open(ctx context.Context, key routecarrier.ClosedCarrierKey, binding *routecarrier.ClosedCarrierLease, carrier routecarrier.Carrier, deadline time.Time, hello func() (ardp.Hello, error), pending *pendingSession) (returned *session, returnedErr error) {
	var result *session
	var resultErr error
	var cancelErr error
	cancelDone := make(chan struct{})
	stopCancel := context.AfterFunc(ctx, func() {
		defer close(cancelDone)
		cancelErr = carrier.Close()
	})
	defer func() {
		if !stopCancel() {
			<-cancelDone
		}
		if cancelErr != nil || ctx.Err() != nil {
			if result != nil && ctx.Err() != nil {
				resultErr = errors.Join(ctx.Err(), resultErr, cancelErr, result.invalidate())
				result = nil
			} else if result == nil && ctx.Err() != nil {
				resultErr = errors.Join(ctx.Err(), resultErr, cancelErr)
			}
		}
		if result != nil {
			if _, err := binding.Carrier(); err != nil {
				resultErr = errors.Join(err, result.carrier.Close(), result.invalidate())
				result = nil
			}
		}
		sessions.mu.Lock()
		if result != nil {
			sessions.sessions[key] = result
		}
		pending.session, pending.err = result, resultErr
		delete(sessions.pending, key)
		close(pending.done)
		sessions.mu.Unlock()
		returned, returnedErr = result, resultErr
		if result != nil {
			sessions.readers.Add(1)
			go func() {
				defer sessions.readers.Done()
				result.copyReverse()
			}()
		}
	}()
	if _, err := binding.Carrier(); err != nil {
		resultErr = err
		return nil, resultErr
	}
	value, err := hello()
	if err != nil {
		resultErr = err
		return nil, resultErr
	}
	body, err := ardp.EncodeHello(value)
	if err != nil {
		resultErr = err
		return nil, resultErr
	}
	if value.Deadline.Before(deadline) {
		deadline = value.Deadline
	}
	if err := carrier.SetDeadline(deadline); err != nil {
		resultErr = err
		return nil, resultErr
	}
	if err := ardp.WriteFrame(carrier, ardp.Frame{Kind: 1, Lane: 0, Body: body}); err != nil {
		if ctx.Err() != nil {
			resultErr = ctx.Err()
			return nil, resultErr
		}
		resultErr = err
		return nil, resultErr
	}
	accepted, err := ardp.ReadFrame(carrier)
	if err != nil {
		if ctx.Err() != nil {
			resultErr = ctx.Err()
			return nil, resultErr
		}
		resultErr = err
		return nil, resultErr
	}
	status, _, err := ardp.DecodeAcceptFrame(accepted)
	if err != nil || status != 0 {
		resultErr = errors.New("closed forwarding outer HELLO is unavailable")
		return nil, resultErr
	}
	if err := carrier.SetDeadline(time.Time{}); err != nil {
		resultErr = err
		return nil, resultErr
	}
	session := &session{owner: sessions, key: key, carrier: carrier, binding: binding, invalidate: binding.Invalidate, children: make(map[uint32]*frameQueue), retired: make(map[uint32]struct{})}
	result = session
	return result, nil
}

// joinedResult waits for every reader owned by this session set and returns
// their retained physical cleanup result. The caller must first join every
// producer that can publish a session, so no reader can be added after Wait.
func (sessions *sessionSet) joinedResult() error {
	if sessions == nil {
		return nil
	}
	sessions.readers.Wait()
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	return sessions.cleanupErr
}

func (session *session) attach(ctx context.Context, open route.ClosedOpen, restriction route.ClosedChildRestriction, queue func(ardp.Frame) error, retired func() bool) (uint32, *frameQueue, error) {
	if session == nil || ctx == nil {
		return 0, nil, errors.New("closed forwarding Carrier session is unavailable")
	}
	body, err := route.EncodeClosedNodeOpen(open, restriction)
	if err != nil {
		return 0, nil, err
	}
	// The writer owns allocation order as well as complete OPEN frames. Two
	// prefix callers must not allocate1/3 and emit3 before1 on a shared Carrier.
	deadline := closedForwardingHandshakeDeadline(open.Deadline, time.Now().UTC())
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	acquired, err := session.acquireWriter(ctx, deadline, nil)
	if err != nil || !acquired {
		return 0, nil, err
	}
	defer session.releaseWriter()
	session.mu.Lock()
	if session.closed || len(session.children)+len(session.retired) >= 256 || session.lastOdd > ^uint32(0)-2 {
		session.mu.Unlock()
		return 0, nil, errors.New("closed forwarding Carrier lanes are unavailable")
	}
	if session.lastOdd == 0 {
		session.lastOdd = 1
	} else {
		session.lastOdd += 2
	}
	lane := session.lastOdd
	// The prefix byte reservation bounds frames, including control overhead.
	reverse := newFrameQueue(4 << 20)
	session.children[lane] = reverse
	if session.queues == nil {
		session.queues = make(map[uint32]func(ardp.Frame) error)
	}
	session.queues[lane] = queue
	if session.retirements == nil {
		session.retirements = make(map[uint32]func() bool)
	}
	session.retirements[lane] = retired
	session.mu.Unlock()
	err = closedForwardingWriteDeadline(session.carrier, deadline)
	if err == nil {
		err = ardp.WriteFrame(session.carrier, ardp.Frame{Kind: 4, Lane: lane, Body: body})
	}
	if err != nil {
		_ = session.carrier.Close()
		session.retire(lane)
		return 0, nil, err
	}
	return lane, reverse, nil
}

// A received child terminal retires queued upstream work even when its reverse
// copier is blocked. Recheck under the reader's lock after writer acquisition:
// it may receive CLOSE while this writer is waiting. Generic Carrier EOF never
// supplies this evidence. Once physical emission starts, every error remains
// an error and retires the Carrier: a partial frame cannot be reused by siblings.
func (session *session) writeChildFrame(frame ardp.Frame, deadline time.Time, reverse *frameQueue) (bool, error) {
	acquired, err := session.acquireWriter(context.Background(), deadline, reverse)
	if err != nil || !acquired {
		return false, err
	}
	defer session.releaseWriter()
	if err := closedForwardingWriteDeadline(session.carrier, deadline); err != nil {
		return false, err
	}
	err = ardp.WriteFrame(session.carrier, frame)
	if err != nil {
		_ = session.carrier.Close()
	}
	return err == nil, err
}

// acquireWriter waits within this request's authority without changing the
// active frame's physical deadline. No waiter owns a background writer: after
// return it cannot emit a frame. Allocation and OPEN emission use the same
// reservation so cancellation cannot reorder wire IDs or allocate unknown lanes.
func (session *session) acquireWriter(ctx context.Context, deadline time.Time, reverse *frameQueue) (bool, error) {
	if reverse.peerClosed() {
		return false, nil
	}
	if session == nil {
		return false, errors.New("closed forwarding Carrier session is unavailable")
	}
	if deadline.IsZero() {
		return false, errors.New("closed forwarding write deadline is unavailable")
	}
	session.mu.Lock()
	if session.writer == nil {
		session.writer = make(chan struct{}, 1)
		session.writer <- struct{}{}
		session.done = make(chan struct{})
		if session.closed {
			close(session.done)
		}
	}
	writer, done := session.writer, session.done
	session.mu.Unlock()
	var retired <-chan struct{}
	if reverse != nil {
		retired = reverse.retirement
	}
	// Recheck before and after the reservation. A ready writer, timer or local
	// retirement can win the same select; none grants expired physical output.
	check := func() (bool, error) {
		if reverse.peerClosed() {
			return false, nil
		}
		session.mu.Lock()
		closed := session.closed
		session.mu.Unlock()
		if closed {
			return false, errors.New("closed forwarding Carrier session is unavailable")
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if !time.Now().Before(deadline) {
			return false, context.DeadlineExceeded
		}
		if reverse != nil {
			reverse.mu.Lock()
			closed = reverse.closed
			reverse.mu.Unlock()
			if closed {
				return false, context.Canceled
			}
		}
		return true, nil
	}
	if ready, err := check(); !ready {
		return false, err
	}
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-writer:
		if ready, err := check(); !ready {
			session.releaseWriter()
			return false, err
		}
		return true, nil
	case <-timer.C:
	case <-ctx.Done():
	case <-retired:
	case <-done:
	}
	if ready, err := check(); !ready {
		return false, err
	}
	return false, context.DeadlineExceeded
}

func (session *session) releaseWriter() { session.writer <- struct{}{} }
func (session *session) retire(lane uint32) {
	if session == nil {
		return
	}
	session.mu.Lock()
	channel := session.children[lane]
	delete(session.children, lane)
	delete(session.queues, lane)
	delete(session.retirements, lane)
	// A peer CLOSE delivered while the child copier was still live is already
	// the terminal witness. Creating a tombstone after that CLOSE would leave
	// it permanently resident: no later frame exists to remove it.
	peerClosed := channel != nil && channel.peerClosed()
	if !session.closed && !peerClosed {
		session.retired[lane] = struct{}{}
	}
	if channel != nil {
		channel.close()
	}
	session.mu.Unlock()
}

func (session *session) copyReverse() {
	for {
		frame, err := ardp.ReadFrame(session.carrier)
		if err != nil || frame.Lane == 0 || (frame.Kind != 5 && frame.Kind != 6 && frame.Kind != 7 && frame.Kind != 8 && frame.Kind != 9) {
			session.fail()
			return
		}
		if !session.deliverReverse(frame) {
			session.fail()
			return
		}
	}
}

// Delivery and retirement hold the same lock through the channel operation.
// A full bounded queue refuses the Carrier; its reader never waits behind one
// slow child while other children need control or cancellation frames.
func (session *session) deliverReverse(frame ardp.Frame) bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return false
	}
	if _, retired := session.retired[frame.Lane]; retired {
		if frame.Kind == 9 {
			delete(session.retired, frame.Lane)
		}
		return true
	}
	if retired := session.retirements[frame.Lane]; retired != nil && retired() {
		return true
	}
	channel := session.children[frame.Lane]
	if channel == nil {
		return false
	}
	err := channel.push(frame, session.queues[frame.Lane])
	if errors.Is(err, errClosedForwardingQueueFull) {
		// Expiry can occur after the precheck while push waits for the queue.
		// A full local queue for retired work cannot indict the shared peer.
		if retired := session.retirements[frame.Lane]; retired != nil && retired() {
			return true
		}
	}
	return err == nil || errors.Is(err, route.ErrClosedForwardingChildRetired)
}

func (session *session) fail() {
	if session == nil {
		return
	}
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return
	}
	session.closed = true
	if session.done != nil {
		close(session.done)
	}
	children := session.children
	session.children = make(map[uint32]*frameQueue)
	clear(session.retired)
	clear(session.queues)
	clear(session.retirements)
	session.mu.Unlock()
	cleanupErr := errors.Join(session.carrier.Close(), session.invalidate())
	for _, channel := range children {
		channel.close()
	}
	session.owner.mu.Lock()
	session.owner.cleanupErr = errors.Join(session.owner.cleanupErr, cleanupErr)
	if session.owner.sessions[session.key] == session {
		delete(session.owner.sessions, session.key)
	}
	session.owner.mu.Unlock()
}

func (server *forwardServer) outerHello(snapshot state.NodeDuty, open route.ClosedOpen) (ardp.Hello, error) {
	if server.dependencies.authority.CurrentProfile == nil {
		return ardp.Hello{}, errors.New("closed forwarding profile is unavailable")
	}
	profile, available := server.dependencies.authority.CurrentProfile()
	if !available || !authority.ProfileMatchesSnapshot(profile, snapshot, server.clock()) {
		return ardp.Hello{}, errors.New("closed forwarding profile is unavailable")
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil || nonce == [32]byte{} {
		return ardp.Hello{}, errors.New("draw closed forwarding channel nonce")
	}
	return ardp.Hello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest, ProfileDigest: profile.Digest,
		RecipientNodeID: open.NextNodeID, RecipientDutyGeneration: open.NextDutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: nonce,
		Deadline: profile.NotAfter}, nil
}

// Every maintained selected Carrier supports independent write deadlines.
// Refuse an incompatible transport instead of resetting the shared reader's
// lifetime whenever one child writes.
func closedForwardingWriteDeadline(carrier routecarrier.Carrier, deadline time.Time) error {
	writer, ok := carrier.(interface{ SetWriteDeadline(time.Time) error })
	if !ok {
		return errors.New("closed forwarding write deadline is unsupported")
	}
	return writer.SetWriteDeadline(deadline)
}

// Opening a child never spends its admitted lifetime waiting for a new
// Carrier or for the peer to consume OPEN. Its receiving admission is separate.
func closedForwardingHandshakeDeadline(end, now time.Time) time.Time {
	pending := now.Add(10 * time.Second)
	if end.Before(pending) {
		return end
	}
	return pending
}

// The shared reader never waits for a child writer. Queued complete frames
// consume the same finite byte budget regardless of TLS record fragmentation.
// Allocation grows only with queued work, rather than reserving thousands of
// frame slots for every idle child. Route separately charges the prefix/duty.
var errClosedForwardingQueueFull = errors.New("closed forwarding reverse queue is full")

type frameQueue struct {
	mu             sync.Mutex
	changed        *sync.Cond
	frames         []ardp.Frame
	bytes, maximum int
	closed         bool
	retirement     chan struct{}
	terminal       bool // Complete, reserved peer CLOSE; transport EOF alone is not terminal.
}

func newFrameQueue(maximum int) *frameQueue {
	queue := &frameQueue{maximum: maximum, retirement: make(chan struct{})}
	queue.changed = sync.NewCond(&queue.mu)
	return queue
}

func (queue *frameQueue) push(frame ardp.Frame, reserve func(ardp.Frame) error) error {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	size := 16 + len(frame.Body)
	if queue.closed || size > queue.maximum-queue.bytes {
		return errClosedForwardingQueueFull
	}
	if reserve != nil {
		if err := reserve(frame); err != nil {
			return err
		}
	}
	queue.frames = append(queue.frames, frame)
	queue.bytes += size
	if frame.Kind == 9 {
		if !queue.terminal && !queue.closed {
			close(queue.retirement)
		}
		queue.terminal = true
	}
	queue.changed.Signal()
	return nil
}

func (queue *frameQueue) next() (ardp.Frame, bool) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	for len(queue.frames) == 0 && !queue.closed {
		queue.changed.Wait()
	}
	if len(queue.frames) == 0 {
		return ardp.Frame{}, false
	}
	frame := queue.frames[0]
	queue.frames[0] = ardp.Frame{}
	queue.frames = queue.frames[1:]
	queue.bytes -= 16 + len(frame.Body)
	if len(queue.frames) == 0 {
		queue.frames = nil
	}
	return frame, true
}

func (queue *frameQueue) close() {
	queue.mu.Lock()
	if !queue.closed && !queue.terminal {
		close(queue.retirement)
	}
	queue.closed = true
	queue.changed.Broadcast()
	queue.mu.Unlock()
}

// peerClosed remains true after draining or retiring the queue. It records the
// peer's complete terminal frame independently of the child copier's schedule.
func (queue *frameQueue) peerClosed() bool {
	if queue == nil {
		return false
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.terminal
}
