//go:build linux

package forwarding

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// queuedWriteCarrier pins the scheduling boundary after one real physical byte.
// The selected TCP/TLS or QUIC transport still owns framing and deadlines. This
// avoids relying on platform socket/stream buffer sizes to create backpressure.
type queuedWriteCarrier struct {
	carrier.Carrier
	mu           sync.Mutex
	hold         bool
	entered      chan struct{}
	release      chan struct{}
	closed       chan struct{}
	once         sync.Once
	deadlines    []time.Time
	closeFailure error
	closeCalls   int
}

func (c *queuedWriteCarrier) Write(p []byte) (int, error) {
	c.mu.Lock()
	hold := c.hold
	c.hold = false
	c.mu.Unlock()
	if !hold || len(p) == 0 {
		return c.Carrier.Write(p)
	}
	n, err := c.Carrier.Write(p[:1])
	if err != nil {
		return n, err
	}
	close(c.entered)
	select {
	case <-c.release:
	case <-c.closed:
		return n, net.ErrClosed
	}
	rest, err := c.Carrier.Write(p[n:])
	return n + rest, err
}

func (c *queuedWriteCarrier) SetWriteDeadline(end time.Time) error {
	c.mu.Lock()
	c.deadlines = append(c.deadlines, end)
	c.mu.Unlock()
	return c.Carrier.(interface{ SetWriteDeadline(time.Time) error }).SetWriteDeadline(end)
}

func (c *queuedWriteCarrier) Close() error {
	c.once.Do(func() { close(c.closed) })
	c.mu.Lock()
	c.closeCalls++
	first, failure := c.closeCalls == 1, c.closeFailure
	c.mu.Unlock()
	err := c.Carrier.Close()
	if first {
		return errors.Join(err, failure)
	}
	return err
}

func TestClosedForwardingQueuedDeadlineActualCaller(t *testing.T) {
	for _, profile := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		for _, outcome := range []string{"expiry", "peer-close", "parent-cancellation", "open-expiry", "open-cancellation", "partial-failure"} {
			t.Run(string(profile)+"/"+outcome, func(t *testing.T) { testQueuedDeadlineActualCaller(t, profile, outcome) })
		}
	}
}

func testQueuedDeadlineActualCaller(t *testing.T, profile carrier.CarrierProfile, outcome string) {
	clientCertificate, clientKey := nodeCertificate(t, 480, "queued-client")
	peerCertificate, peerKey := nodeCertificate(t, 481, "queued-peer")
	endpoint := closedForwardingActualCarrierEndpoint(t, profile)
	listener, err := carrier.ListenClosedSharedCarrier(profile, endpoint, peerCertificate, func(key [32]byte) bool { return key == clientKey }, 1)
	if err != nil {
		t.Fatal(err)
	}

	end := time.Now().UTC().Truncate(time.Second).Add(8 * time.Second)
	frames := make(chan ardp.Frame, 16)
	peerResult := make(chan error, 1)
	peerStop := make(chan struct{})
	var peer net.Conn
	var closePeer func() error
	var peerMu sync.Mutex
	peerCloseResult := make(chan error, 1)
	acceptCtx, cancelAccept := context.WithCancel(t.Context())
	defer func() {
		cancelAccept()
		close(peerStop)
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("listener cleanup: %v", err)
		}
		peerMu.Lock()
		closeConnection := closePeer
		peerMu.Unlock()
		if closeConnection != nil {
			_ = closeConnection() // The worker retains this same close result below.
		}
		select {
		case <-peerResult:
			if err := <-peerCloseResult; err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("peer cleanup: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("peer cleanup did not join")
		}
	}()
	go func() {
		accepted, err := listener.Accept(acceptCtx, 10*time.Second)
		if err != nil {
			peerCloseResult <- nil
			peerResult <- err
			return
		}
		connection := accepted.Connection
		closeConnection := sync.OnceValue(connection.Close)
		defer func() { peerCloseResult <- closeConnection() }()
		peerMu.Lock()
		peer = connection
		closePeer = closeConnection
		peerMu.Unlock()
		if accepted.Kind != carrier.ClosedSharedNode || accepted.NodeKey != clientKey {
			peerResult <- errors.New("Node authentication lost")
			return
		}
		if err := connection.SetDeadline(end); err != nil {
			peerResult <- err
			return
		}
		frame, err := ardp.ReadFrame(connection)
		if err != nil || frame.Kind != ardp.KindHello {
			peerResult <- fmt.Errorf("HELLO: %+v %v", frame, err)
			return
		}
		if err := ardp.WriteFrame(connection, mustClosedForwardAccept(t)); err != nil {
			peerResult <- err
			return
		}
		for {
			frame, err = ardp.ReadFrame(connection)
			if err != nil {
				peerResult <- err
				return
			}
			select {
			case frames <- frame:
			case <-peerStop:
				peerResult <- nil
				return
			}
		}
	}()
	physical, err := carrier.OpenClosedNodeCarrier(t.Context(), carrier.ClosedNodeCarrierRequest{CarrierProfile: profile, Endpoint: endpoint, Certificate: clientCertificate, ExpectedPeerKey: peerKey, Deadline: end})
	if err != nil {
		t.Fatal(err)
	}
	physicalTransferred := false
	defer func() {
		if !physicalTransferred {
			if err := physical.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("untransferred physical Carrier cleanup: %v", err)
			}
		}
	}()
	observed := &queuedWriteCarrier{Carrier: physical, entered: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
	var closeFailure error
	if outcome == "partial-failure" {
		closeFailure = errors.New("first physical close failure")
		observed.closeFailure = closeFailure
	}
	pool, err := carrier.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	sessions := newSessionSet()
	var links []*forwardLink
	var channels []*route.ClosedForwardingChannel
	var release sync.Once
	defer func() {
		release.Do(func() { close(observed.release) })
		poolErr := pool.Close()
		if closeFailure != nil && !errors.Is(poolErr, closeFailure) {
			t.Errorf("pool lost original close failure: %v", poolErr)
		}
		if closeFailure == nil && poolErr != nil {
			t.Errorf("pool cleanup: %v", poolErr)
		}
		for _, link := range links {
			if err := link.close(); err != nil {
				t.Errorf("link cleanup: %v", err)
			}
		}
		for _, channel := range channels {
			if err := channel.Cancel(); err != nil {
				t.Errorf("prefix cleanup: %v", err)
			}
		}
		if err := sessions.joinedResult(); (closeFailure == nil && err != nil) || (closeFailure != nil && !errors.Is(err, closeFailure)) {
			t.Errorf("session cleanup: %v", err)
		}
	}()
	key := carrier.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3}, PeerNodeID: [32]byte{4}, PeerKey: peerKey, CarrierProfile: profile}
	hello := func() (ardp.Hello, error) {
		return ardp.Hello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{2}, RecipientNodeID: [32]byte{4}, RecipientDutyGeneration: 1, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{5}, Deadline: end}, nil
	}
	for index := range 3 {
		lease, err := pool.AcquireContext(t.Context(), key, func() error { return nil }, func() (carrier.Carrier, error) { return observed, nil })
		if err != nil {
			t.Fatal(err)
		}
		physicalTransferred = true
		session, err := sessions.acquire(t.Context(), key, lease, end, hello)
		if err != nil {
			_ = lease.Release()
			t.Fatal(err)
		}
		open := route.ClosedOpen{NextNodeID: [32]byte{4}, NextDutyGeneration: 1, Purpose: ardp.PurposeIssuer, Deadline: end}
		if index == 1 && outcome == "expiry" {
			open.Deadline = time.Now().UTC().Truncate(time.Second).Add(2 * time.Second)
		}
		channel := closedForwardingActualChannel(t, end, open)
		channels = append(channels, channel)
		lane, reverse, err := session.attach(t.Context(), open, route.ClosedChildOrdinary, func(frame ardp.Frame) error { frame.Lane = 1; return channel.QueueReverse(frame) }, func() bool { return channel.ReverseRetired(1) })
		if err != nil {
			_ = lease.Release()
			t.Fatal(err)
		}
		link := &forwardLink{session: session, remoteLane: lane, localLane: 1, reverse: reverse, lease: lease, deadline: open.Deadline, channel: channel, done: make(chan struct{}), stopped: make(chan struct{}), write: func(frame ardp.Frame) error { return channel.AccountOutput(frame) }, abort: func() {}}
		links = append(links, link)
		go link.copyReverse()
		select {
		case frame := <-frames:
			if frame.Kind != ardp.KindOpen || frame.Lane != lane {
				t.Fatalf("OPEN: %+v", frame)
			}
		case <-time.After(time.Second):
			t.Fatal("OPEN missing")
		}
	}
	a, b, sibling := links[0], links[1], links[2]
	if a.session != b.session || !a.lease.SameCarrier(b.lease) {
		t.Fatal("prefixes did not share exact Carrier")
	}
	observed.mu.Lock()
	observed.hold = true
	before := len(observed.deadlines)
	observed.mu.Unlock()
	event := func(link *forwardLink, payload string) route.ClosedForwardingEvent {
		if _, err := link.channel.Accept(ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte(payload)}); err != nil {
			t.Fatal(err)
		}
		selected, ok := nextAvailableForwarding(link.channel, map[uint32]*forwardLink{1: link}, nil)
		if !ok || selected.Kind != ardp.KindBytes {
			t.Fatalf("actual admitted event: %+v %t", selected, ok)
		}
		return selected
	}
	aDone := make(chan struct{}, 1)
	if !a.startForwarding(event(a, "A"), func() { aDone <- struct{}{} }, func() {
		if outcome != "partial-failure" {
			t.Error("A prefix aborted")
		}
	}) {
		t.Fatal("A unavailable")
	}
	select {
	case <-observed.entered:
	case <-time.After(time.Second):
		t.Fatal("A never entered physical output")
	}
	if outcome == "partial-failure" {
		// A has physically emitted a byte; a received CLOSE or transport retirement
		// cannot turn this error into success or reuse the partial framing boundary.
		_ = observed.Carrier.Close()
		release.Do(func() { close(observed.release) })
		select {
		case <-aDone:
		case <-time.After(time.Second):
			t.Fatal("partial physical writer did not join")
		}
		if err := a.forwardingError(); err == nil {
			t.Fatal("started partial frame became success")
		}
		if err := sessions.joinedResult(); !errors.Is(err, closeFailure) {
			t.Fatalf("joined physical cleanup lost first failure: %v", err)
		}
		if _, err := sibling.lease.Carrier(); err == nil {
			t.Fatal("partial frame left healthy shared lease")
		}
		return
	}
	bDone := make(chan struct{}, 1)
	bAborted := make(chan struct{}, 1)
	parent, application := net.Pipe()
	defer parent.Close()
	defer application.Close()
	abortParent := func() { _ = parent.Close(); bAborted <- struct{}{} }
	if outcome == "open-expiry" || outcome == "open-cancellation" {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		open := route.ClosedOpen{NextNodeID: [32]byte{4}, NextDutyGeneration: 1, Purpose: ardp.PurposeIssuer, Deadline: time.Now().UTC().Truncate(time.Second).Add(2 * time.Second)}
		result := make(chan error, 1)
		b.session.mu.Lock()
		previousID := b.session.lastOdd
		b.session.mu.Unlock()
		go func() { _, _, err := b.session.attach(ctx, open, route.ClosedChildOrdinary, nil, nil); result <- err }()
		expected := error(context.DeadlineExceeded)
		if outcome == "open-cancellation" {
			cancel()
			expected = context.Canceled
		}
		select {
		case err := <-result:
			if !errors.Is(err, expected) {
				t.Fatalf("queued OPEN: %v", err)
			}
		case <-time.After(time.Until(open.Deadline) + 300*time.Millisecond):
			t.Fatal("queued OPEN survived its authority")
		}
		b.session.mu.Lock()
		unchanged := b.session.lastOdd == previousID
		b.session.mu.Unlock()
		if !unchanged {
			t.Fatal("unemitted OPEN allocated an unknown lane")
		}
	} else {
		if !b.startForwarding(event(b, "B"), func() { bDone <- struct{}{} }, abortParent) {
			t.Fatal("B unavailable")
		}
		var joined <-chan struct{}
		expected := error(context.DeadlineExceeded)
		if outcome == "peer-close" {
			peerMu.Lock()
			connection := peer
			peerMu.Unlock()
			if err := ardp.WriteFrame(connection, ardp.Frame{Kind: ardp.KindClose, Lane: b.remoteLane, Body: []byte{0}}); err != nil {
				t.Fatal(err)
			}
			expected = nil
		} else if outcome == "parent-cancellation" {
			stopped := make(chan struct{})
			go func() { b.stop(); close(stopped) }()
			joined = stopped
			expected = context.Canceled
		}
		bound := time.Second
		if outcome == "expiry" {
			bound = time.Until(b.deadline) + 300*time.Millisecond
		}
		select {
		case <-bDone:
		case <-time.After(bound):
			t.Fatal("queued B survived deadline/retirement while A still held writer")
		}
		if err := b.forwardingError(); (expected == nil && err != nil) || (expected != nil && !errors.Is(err, expected)) {
			t.Fatalf("queued outcome: %v, want %v", err, expected)
		}
		if joined != nil {
			select {
			case <-joined:
			case <-time.After(time.Second):
				t.Fatal("parent teardown did not join queued writer")
			}
		}
		if expected != nil {
			select {
			case <-bAborted:
			case <-time.After(time.Second):
				t.Fatal("failed prefix did not abort")
			}
			if _, err := application.Read(make([]byte, 1)); err == nil {
				t.Fatal("incoming failed prefix remained usable")
			}
		} else {
			if !b.reverse.peerClosed() {
				t.Fatal("authenticated CLOSE witness lost")
			}
			select {
			case <-bAborted:
				t.Fatal("peer CLOSE aborted parent")
			default:
			}
		}
	}
	observed.mu.Lock()
	count := len(observed.deadlines)
	active := observed.deadlines[count-1]
	observed.mu.Unlock()
	if count != before+1 || active != a.deadline {
		t.Fatal("unemitted B changed active A deadline")
	}
	if err := b.close(); err != nil {
		t.Fatal(err)
	}
	release.Do(func() { close(observed.release) })
	select {
	case <-aDone:
	case <-time.After(time.Second):
		t.Fatal("A did not finish")
	}
	if err := a.forwardingError(); err != nil {
		t.Fatalf("A failed: %v", err)
	}
	select {
	case frame := <-frames:
		if frame.Lane != a.remoteLane || string(frame.Body) != "A" {
			t.Fatalf("A frame: %+v", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("A frame missing")
	}
	siblingDone := make(chan struct{}, 1)
	if !sibling.startForwarding(event(sibling, "sibling"), func() { siblingDone <- struct{}{} }, func() { t.Error("healthy prefix aborted") }) {
		t.Fatal("sibling unavailable")
	}
	select {
	case <-siblingDone:
	case <-time.After(time.Second):
		t.Fatal("healthy prefix stalled")
	}
	if err := sibling.forwardingError(); err != nil {
		t.Fatalf("healthy prefix poisoned: %v", err)
	}
	select {
	case frame := <-frames:
		if frame.Lane != sibling.remoteLane || string(frame.Body) != "sibling" {
			t.Fatalf("late B or wrong sibling frame: %+v", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("sibling frame missing")
	}
	select {
	case frame := <-frames:
		t.Fatalf("unexpected late output: %+v", frame)
	case <-time.After(30 * time.Millisecond):
	}
}
