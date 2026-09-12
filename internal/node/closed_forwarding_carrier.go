package node

import (
	"crypto/rand"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// closedForwardingSessions gives every retained Carrier one reader and one
// serialized writer. Its child IDs are local to that Carrier, so source lane
// IDs from different admitted channels can never collide on a reused leg.
type closedForwardingSessions struct {
	mu         sync.Mutex
	sessions   map[route.ClosedCarrierKey]*closedForwardingSession
	workers    *sync.WaitGroup
	cleanupErr error
}

type closedForwardingSession struct {
	owner       *closedForwardingSessions
	key         route.ClosedCarrierKey
	carrier     route.Carrier
	binding     *route.ClosedCarrierLease
	invalidate  func() error
	mu          sync.Mutex
	writer      sync.Mutex
	children    map[uint32]*closedForwardingQueue
	queues      map[uint32]func(route.ClosedLaneFrame) error
	retirements map[uint32]func() bool
	retired     map[uint32]struct{}
	lastOdd     uint32
	closed      bool
}

func newClosedForwardingSessions(workers *sync.WaitGroup) *closedForwardingSessions {
	return &closedForwardingSessions{sessions: make(map[route.ClosedCarrierKey]*closedForwardingSession), workers: workers}
}

func (sessions *closedForwardingSessions) acquire(key route.ClosedCarrierKey, binding *route.ClosedCarrierLease, deadline time.Time, hello func() (route.ClosedHello, error)) (*closedForwardingSession, error) {
	if sessions == nil || sessions.workers == nil || binding == nil || hello == nil {
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
			break
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
	defer sessions.mu.Unlock()
	if _, err := binding.Carrier(); err != nil {
		return nil, err
	}
	value, err := hello()
	if err != nil {
		return nil, err
	}
	body, err := route.EncodeClosedHello(value)
	if err != nil {
		return nil, err
	}
	if value.Deadline.Before(deadline) {
		deadline = value.Deadline
	}
	if err := carrier.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if err := route.WriteClosedLaneFrame(carrier, route.ClosedLaneFrame{Kind: 1, Lane: 0, Body: body}); err != nil {
		return nil, err
	}
	accepted, err := route.ReadClosedLaneFrame(carrier)
	if err != nil {
		return nil, err
	}
	status, _, err := route.DecodeClosedAcceptFrame(accepted)
	if err != nil || status != 0 {
		return nil, errors.New("closed forwarding outer HELLO is unavailable")
	}
	if err := carrier.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	session := &closedForwardingSession{owner: sessions, key: key, carrier: carrier, binding: binding, invalidate: binding.Invalidate, children: make(map[uint32]*closedForwardingQueue), retired: make(map[uint32]struct{})}
	sessions.sessions[key] = session
	sessions.workers.Add(1)
	go func() {
		defer sessions.workers.Done()
		session.copyReverse()
	}()
	return session, nil
}

func (session *closedForwardingSession) attach(open route.ClosedOpen, restriction route.ClosedChildRestriction, queue func(route.ClosedLaneFrame) error, retired func() bool) (uint32, *closedForwardingQueue, error) {
	if session == nil {
		return 0, nil, errors.New("closed forwarding Carrier session is unavailable")
	}
	body, err := route.EncodeClosedNodeOpen(open, restriction)
	if err != nil {
		return 0, nil, err
	}
	// The writer owns allocation order as well as complete OPEN frames. Two
	// prefix callers must not allocate1/3 and emit3 before1 on a shared Carrier.
	session.writer.Lock()
	defer session.writer.Unlock()
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
	reverse := newClosedForwardingQueue(4 << 20)
	session.children[lane] = reverse
	if session.queues == nil {
		session.queues = make(map[uint32]func(route.ClosedLaneFrame) error)
	}
	session.queues[lane] = queue
	if session.retirements == nil {
		session.retirements = make(map[uint32]func() bool)
	}
	session.retirements[lane] = retired
	session.mu.Unlock()
	err = closedForwardingWriteDeadline(session.carrier, closedForwardingHandshakeDeadline(open.Deadline, time.Now().UTC()))
	if err == nil {
		err = route.WriteClosedLaneFrame(session.carrier, route.ClosedLaneFrame{Kind: 4, Lane: lane, Body: body})
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
func (session *closedForwardingSession) writeChildFrame(frame route.ClosedLaneFrame, deadline time.Time, reverse *closedForwardingQueue) (bool, error) {
	if reverse.peerClosed() {
		return false, nil
	}
	if session == nil {
		return false, errors.New("closed forwarding Carrier session is unavailable")
	}
	session.writer.Lock()
	defer session.writer.Unlock()
	session.mu.Lock()
	closed, terminal := session.closed, reverse.peerClosed()
	session.mu.Unlock()
	if terminal {
		return false, nil
	}
	if closed {
		return false, errors.New("closed forwarding Carrier session is unavailable")
	}
	if deadline.IsZero() {
		return false, errors.New("closed forwarding write deadline is unavailable")
	}
	if err := closedForwardingWriteDeadline(session.carrier, deadline); err != nil {
		return false, err
	}
	err := route.WriteClosedLaneFrame(session.carrier, frame)
	if err != nil {
		_ = session.carrier.Close()
	}
	return err == nil, err
}

func (session *closedForwardingSession) retire(lane uint32) {
	if session == nil {
		return
	}
	session.mu.Lock()
	channel := session.children[lane]
	delete(session.children, lane)
	delete(session.queues, lane)
	delete(session.retirements, lane)
	if !session.closed {
		session.retired[lane] = struct{}{}
	}
	if channel != nil {
		channel.close()
	}
	session.mu.Unlock()
}

func (session *closedForwardingSession) copyReverse() {
	for {
		frame, err := route.ReadClosedLaneFrame(session.carrier)
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
func (session *closedForwardingSession) deliverReverse(frame route.ClosedLaneFrame) bool {
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

func (session *closedForwardingSession) fail() {
	if session == nil {
		return
	}
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return
	}
	session.closed = true
	children := session.children
	session.children = make(map[uint32]*closedForwardingQueue)
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

func (server *closedForwardingServer) closedForwardingOuterHello(snapshot dutyFacts, open route.ClosedOpen) (route.ClosedHello, error) {
	if server.config.CurrentClosedProfile == nil {
		return route.ClosedHello{}, errors.New("closed forwarding profile is unavailable")
	}
	profile, available := server.config.CurrentClosedProfile()
	if !available || !closedRouteProfileMatchesSnapshot(profile, snapshot, server.clock()) {
		return route.ClosedHello{}, errors.New("closed forwarding profile is unavailable")
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil || nonce == [32]byte{} {
		return route.ClosedHello{}, errors.New("draw closed forwarding channel nonce")
	}
	return route.ClosedHello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest, ProfileDigest: profile.Digest,
		RecipientNodeID: open.NextNodeID, RecipientDutyGeneration: open.NextDutyGeneration, Purpose: route.ClosedPurposeForwarding, ChannelNonce: nonce,
		Deadline: profile.NotAfter}, nil
}

// Every maintained selected Carrier supports independent write deadlines.
// Refuse an incompatible transport instead of resetting the shared reader's
// lifetime whenever one child writes.
func closedForwardingWriteDeadline(carrier route.Carrier, deadline time.Time) error {
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
