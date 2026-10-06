package join

import (
	"bytes"
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// Invalid public preparation must not touch an unowned physical connection.
func TestJoinPreparationRefusesUnreservedCapacityBeforeIO(t *testing.T) {
	if capacity, err := ReserveCapacity(nil); err == nil || capacity != nil {
		t.Fatal("absent principal budget acquired capacity")
	}
	capacity := &Capacity{}
	capacity.Release()
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	owner := NewPairing(nil)
	err := owner.Serve(t.Context(), local, ardp.Hello{Purpose: ardp.PurposeDataJoin}, 32<<20, nil, capacity)
	if err == nil || len(owner.entries) != 0 {
		t.Fatal("unreserved capacity entered pairing", err)
	}
	// Refusal leaves the caller's actual pipe open and usable in both directions.
	done := make(chan error, 1)
	go func() { _, err := local.Write([]byte{7}); done <- err }()
	var b [1]byte
	if _, err := io.ReadFull(remote, b[:]); err != nil || b[0] != 7 {
		t.Fatal("preparation refusal touched the original connection", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// This suite owns Pairing matching, setup bounds, the paired RESULT barrier
// and retirement of both original physical sides. The shared fixture exercises
// the post-admission pairing and physical lifetime.
// It supplies no successful Network, Admission, Hosting or Service authority.
type joinBoundsPeer struct {
	conn     net.Conn
	done     chan error
	capacity *Capacity
	finished bool
	result   error
}

type joinBoundsFixture struct {
	ctx    context.Context
	cancel context.CancelFunc
	pairs  Pairing
	queues *framing.Budget
	peers  []*joinBoundsPeer
}

func newJoinBoundsFixture() *joinBoundsFixture {
	ctx, cancel := context.WithCancel(context.Background())
	return &joinBoundsFixture{ctx: ctx, cancel: cancel, queues: framing.NewBudget(64 << 20)}
}

func (f *joinBoundsFixture) start(t *testing.T, request ardp.JoinRequest, wrap func(net.Conn) net.Conn) *joinBoundsPeer {
	return f.startWithProfile(t, request, wrap, [32]byte{3})
}

func (f *joinBoundsFixture) startWithProfile(t *testing.T, request ardp.JoinRequest, wrap func(net.Conn) net.Conn, profile [32]byte) *joinBoundsPeer {
	return f.startWithBounds(t, request, wrap, profile, 32<<20)
}

func (f *joinBoundsFixture) startWithBounds(t *testing.T, request ardp.JoinRequest, wrap func(net.Conn) net.Conn, profile [32]byte, limit uint64) *joinBoundsPeer {
	t.Helper()
	local, remote := net.Pipe()
	capacity, err := ReserveCapacity(f.queues)
	if err != nil {
		_ = local.Close()
		_ = remote.Close()
		t.Fatal(err)
	}
	peer := &joinBoundsPeer{conn: remote, done: make(chan error, 1), capacity: capacity}
	f.peers = append(f.peers, peer)
	if wrap != nil {
		local = wrap(local)
	}
	hello := ardp.Hello{ProfileDigest: profile, Purpose: ardp.PurposeDataJoin, Deadline: time.Now().Add(time.Minute).UTC().Truncate(time.Second)}
	go func() { peer.done <- f.pairs.Serve(f.ctx, local, hello, limit, nil, capacity) }()
	body, err := ardp.EncodeJoinRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	return peer
}

func (p *joinBoundsPeer) wait() error {
	if !p.finished {
		p.result = <-p.done
		p.finished = true
	}
	return p.result
}

func (f *joinBoundsFixture) close(t *testing.T) {
	t.Helper()
	f.cancel()
	for _, peer := range f.peers {
		_ = peer.conn.Close()
	}
	for _, peer := range f.peers {
		_ = peer.wait()
		peer.capacity.Release()
	}
	assertRetainedBudget(t, f.queues, 64<<20, 0, 0)
}

func joinBoundsRequest(side, nonce uint8) ardp.JoinRequest {
	return ardp.JoinRequest{Nonce: [32]byte{nonce}, Secret: [32]byte{4}, Context: [32]byte{5}, Side: side, Deadline: time.Now().Add(10 * time.Second).UTC().Truncate(time.Second)}
}

func readJoinBoundsResult(t *testing.T, peer *joinBoundsPeer, nonce uint8) {
	t.Helper()
	frame, err := ardp.ReadFrame(peer.conn)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Kind != ardp.KindResult || frame.Lane != 1 {
		t.Fatal("paired side did not receive its local RESULT")
	}
	if status, err := ardp.DecodeJoinResult(frame.Body, [32]byte{nonce}); err != nil || status != 0 {
		t.Fatal("original side or accepted result changed", err)
	}
}

func TestJoinPairRefusedArrivalPreservesWaitingSide(t *testing.T) {
	for _, refusal := range []string{"duplicate-side", "different-context", "different-profile"} {
		t.Run(refusal, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fixture := newJoinBoundsFixture()
				defer fixture.close(t)
				first := fixture.start(t, joinBoundsRequest(1, 1), nil)
				synctest.Wait()
				request := joinBoundsRequest(1, 2)
				if refusal == "different-context" {
					request.Side, request.Context = 2, [32]byte{9}
				}
				profile := [32]byte{3}
				if refusal == "different-profile" {
					request.Side, profile = 2, [32]byte{9}
				}
				refused := fixture.startWithProfile(t, request, nil, profile)
				frame, err := ardp.ReadFrame(refused.conn)
				if err != nil || frame.Kind != ardp.KindResult || frame.Lane != 1 || len(frame.Body) != 16384 {
					t.Fatal("canonical conflicting JOIN did not receive fixed local refusal RESULT", err)
				}
				if status, err := ardp.DecodeJoinResult(frame.Body, [32]byte{2}); err != nil || status == 0 {
					t.Fatal("conflicting JOIN refusal lost its own nonce or status", err)
				}
				if err := refused.wait(); err == nil {
					t.Fatal("conflicting arrival joined the retained side")
				}
				synctest.Wait()
				select {
				case err := <-first.done:
					first.finished, first.result = true, err
					t.Fatal("refused arrival retired the waiting side", err)
				default:
				}
				second := fixture.start(t, joinBoundsRequest(2, 3), nil)
				readJoinBoundsResult(t, first, 1)
				readJoinBoundsResult(t, second, 3)
				written := make(chan error, 1)
				go func() {
					written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("original side ciphertext")})
				}()
				frame, err = ardp.ReadFrame(second.conn)
				if err != nil || frame.Kind != ardp.KindBytes || frame.Lane != 1 || string(frame.Body) != "original side ciphertext" {
					t.Fatal("original side was replaced or lost its data path", err)
				}
				if err := <-written; err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

func TestJoinPairThirdArrivalCannotReplaceEitherMatchedSide(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		first := fixture.start(t, joinBoundsRequest(1, 1), nil)
		second := fixture.start(t, joinBoundsRequest(2, 2), nil)
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		synctest.Wait()
		fixture.pairs.mu.Lock()
		original := fixture.pairs.entries[[32]byte{4}]
		sides := original.sides
		fixture.pairs.mu.Unlock()
		third := fixture.start(t, joinBoundsRequest(1, 3), nil)
		frame, err := ardp.ReadFrame(third.conn)
		if err != nil || frame.Kind != ardp.KindResult || frame.Lane != 1 {
			t.Fatal("third arrival lost its own refusal", err)
		}
		if status, err := ardp.DecodeJoinResult(frame.Body, [32]byte{3}); err != nil || status == 0 || third.wait() == nil {
			t.Fatal("third arrival acquired the matched pair", err)
		}
		fixture.pairs.mu.Lock()
		retained := fixture.pairs.entries[[32]byte{4}]
		unchanged := retained == original && original.sides == sides && !original.sealed
		fixture.pairs.mu.Unlock()
		if !unchanged {
			t.Fatal("third arrival replaced or retired an original matched side")
		}
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("retained pair")})
		}()
		frame, err = ardp.ReadFrame(second.conn)
		if err != nil || frame.Kind != ardp.KindBytes || string(frame.Body) != "retained pair" {
			t.Fatal("original pair stopped carrying its own bytes", frame, err)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
	})
}

func TestJoinPairUnmatchedSetupExpiresAtOriginalBound(t *testing.T) {
	for _, limit := range []struct {
		name    string
		request time.Duration
		wait    time.Duration
	}{
		{name: "request-deadline", request: 3 * time.Second, wait: 3 * time.Second},
		{name: "reservation-ceiling", request: 30 * time.Second, wait: 10 * time.Second},
	} {
		t.Run(limit.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fixture := newJoinBoundsFixture()
				defer fixture.close(t)
				request := joinBoundsRequest(1, 1)
				request.Deadline = time.Now().Add(limit.request).UTC().Truncate(time.Second)
				peer := fixture.start(t, request, nil)
				time.Sleep(limit.wait - time.Nanosecond)
				synctest.Wait()
				select {
				case err := <-peer.done:
					peer.finished, peer.result = true, err
					t.Fatal("waiting side ended before its original setup bound", err)
				default:
				}
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				select {
				case err := <-peer.done:
					peer.finished, peer.result = true, err
					if !errors.Is(err, context.DeadlineExceeded) {
						t.Fatal("unmatched side lost its setup expiry", err)
					}
				default:
					t.Fatal("unmatched side remained live past its original setup bound")
				}
				if len(fixture.pairs.entries) != 0 {
					t.Fatal("expired unmatched side retained a pairing entry")
				}
			})
		})
	}
}

func TestJoinPairMatchedDataOutlivesSetupWithinOriginalHello(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		first := fixture.start(t, joinBoundsRequest(1, 1), nil)
		second := fixture.start(t, joinBoundsRequest(2, 2), nil)
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		time.Sleep(11 * time.Second)
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("after setup expiry")})
		}()
		frame, err := ardp.ReadFrame(second.conn)
		if err != nil || frame.Kind != ardp.KindBytes || frame.Lane != 1 || string(frame.Body) != "after setup expiry" {
			t.Fatal("matched data incorrectly retained the ten-second setup deadline", err)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		time.Sleep(49 * time.Second)
		synctest.Wait()
		for _, peer := range []*joinBoundsPeer{first, second} {
			select {
			case err := <-peer.done:
				peer.finished, peer.result = true, err
				if err == nil {
					t.Fatal("original HELLO expiry became a successful terminal outcome")
				}
			default:
				t.Fatal("paired data outlived its original HELLO deadline")
			}
		}
		if len(fixture.pairs.entries) != 0 {
			t.Fatal("original HELLO expiry retained the matched pair")
		}
	})
}

// The first BYTES write puts an actual header prefix onto net.Pipe. Its
// remainder blocks in the real connection until CLOSE interrupts the writer.
type joinBoundsWriteObservation struct {
	net.Conn
	partial      chan struct{}
	cut          bool
	writes       atomic.Int32
	failed       atomic.Bool
	afterFailure atomic.Int32
}

func (c *joinBoundsWriteObservation) Write(p []byte) (int, error) {
	c.writes.Add(1)
	if c.failed.Load() {
		c.afterFailure.Add(1)
	}
	if c.partial != nil && !c.cut && len(p) >= ardp.HeaderSize && p[6] == ardp.KindBytes {
		c.cut = true
		n, err := c.Conn.Write(p[:8])
		if n > 0 {
			close(c.partial)
		}
		if err != nil {
			c.failed.Store(true)
		}
		return n, err
	}
	n, err := c.Conn.Write(p)
	if err != nil {
		c.failed.Store(true)
	}
	return n, err
}

func TestJoinPairCloseInterruptsStartedPartialWriterWithoutFurtherWrite(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		var firstWire, secondWire *joinBoundsWriteObservation
		first := fixture.start(t, joinBoundsRequest(1, 1), func(conn net.Conn) net.Conn {
			firstWire = &joinBoundsWriteObservation{Conn: conn, partial: make(chan struct{})}
			return firstWire
		})
		second := fixture.start(t, joinBoundsRequest(2, 2), func(conn net.Conn) net.Conn {
			secondWire = &joinBoundsWriteObservation{Conn: conn}
			return secondWire
		})
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(second.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("partially relayed ciphertext")})
		}()
		var prefix [8]byte
		if _, err := io.ReadFull(first.conn, prefix[:]); err != nil {
			t.Fatal("BYTES prefix did not enter the physical connection", err)
		}
		if prefix[6] != ardp.KindBytes {
			t.Fatal("partial writer did not start a BYTES frame")
		}
		<-firstWire.partial
		synctest.Wait()
		if firstWire.failed.Load() {
			t.Fatal("partial writer failed before the CLOSE interruption")
		}
		beforeSecond := secondWire.writes.Load()
		closedAt := time.Now()
		if err := ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatal(err)
		}
		for _, peer := range []*joinBoundsPeer{first, second} {
			if err := peer.wait(); !containsPhysicalWrite(err) {
				t.Fatal("joined result lost the interrupted physical write", err)
			}
		}
		if !time.Now().Equal(closedAt) {
			t.Fatal("CLOSE waited for a later deadline to interrupt the partial writer")
		}
		if err := <-written; err != nil {
			t.Fatal("sending side never supplied the complete input frame", err)
		}
		if !firstWire.failed.Load() || firstWire.afterFailure.Load() != 0 {
			t.Fatal("failed partial frame admitted a later physical Write")
		}
		if secondWire.writes.Load() != beforeSecond {
			t.Fatal("pair emitted terminal output after the opposite framing failure")
		}
	})
}

type joinBoundsDeadlineObservation struct {
	net.Conn
	interrupted chan struct{}
	once        sync.Once
}

func (c *joinBoundsDeadlineObservation) SetDeadline(deadline time.Time) error {
	err := c.Conn.SetDeadline(deadline)
	if err == nil && !deadline.IsZero() && !time.Now().Before(deadline) {
		c.once.Do(func() { close(c.interrupted) })
	}
	return err
}

type joinBoundsCloseGate struct {
	net.Conn
	peerInterrupted <-chan struct{}
	entered         chan bool
	release, rescue <-chan struct{}
}

func (c *joinBoundsCloseGate) Close() error {
	interrupted := false
	select {
	case <-c.peerInterrupted:
		interrupted = true
	default:
	}
	c.entered <- interrupted
	select {
	case <-c.peerInterrupted:
	case <-c.rescue:
	}
	<-c.release
	return c.Conn.Close()
}

func TestJoinPairInterruptsBothSidesBeforeJoiningEitherClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		secondInterrupted := make(chan struct{})
		firstCloseEntered := make(chan bool, 1)
		release, rescue := make(chan struct{}), make(chan struct{})
		unblock := sync.OnceFunc(func() { close(rescue); close(release) })
		defer func() { unblock(); fixture.close(t) }()
		first := fixture.start(t, joinBoundsRequest(1, 1), func(conn net.Conn) net.Conn {
			return &joinBoundsCloseGate{Conn: conn, peerInterrupted: secondInterrupted, entered: firstCloseEntered, release: release, rescue: rescue}
		})
		second := fixture.start(t, joinBoundsRequest(2, 2), func(conn net.Conn) net.Conn {
			return &joinBoundsDeadlineObservation{Conn: conn, interrupted: secondInterrupted}
		})
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		fixture.cancel()
		if interrupted := <-firstCloseEntered; !interrupted {
			t.Error("first physical Close started before the opposite deadline interruption")
		}
		for _, peer := range []*joinBoundsPeer{first, second} {
			select {
			case err := <-peer.done:
				peer.finished, peer.result = true, err
				t.Error("pair handler returned while the first physical Close was still blocked", err)
			default:
			}
		}
		unblock()
		for _, peer := range []*joinBoundsPeer{first, second} {
			if err := peer.wait(); !errors.Is(err, context.Canceled) {
				t.Fatal("joined cancellation lost its original cause", err)
			}
		}
		if len(fixture.pairs.entries) != 0 {
			t.Fatal("physically joined cancellation retained the pair")
		}
	})
}

// These are mechanical post-admission controls. Genuine Network, token spend
// and Hosting acceptance are exercised by the separate command integration.
func TestJoinPairWaitsForBothActualResultsBeforeReadingData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		queues := framing.NewBudget(64 << 20)
		pairs := &Pairing{}
		var peers [2]net.Conn
		var observed [2]*joinReadObservation
		var capacities [2]*Capacity
		results := make(chan error, 2)
		for i := range 2 {
			local, remote := net.Pipe()
			peers[i] = remote
			defer remote.Close()
			observed[i] = &joinReadObservation{Conn: local}
			var err error
			capacities[i], err = ReserveCapacity(queues)
			if err != nil {
				t.Fatal(err)
			}
			h := ardp.Hello{ProfileDigest: [32]byte{3}, Purpose: ardp.PurposeDataJoin, Deadline: time.Now().Add(time.Minute).UTC().Truncate(time.Second)}
			r := ardp.JoinRequest{Nonce: [32]byte{byte(i + 1)}, Secret: [32]byte{4}, Context: [32]byte{5}, Side: uint8(i + 1), Deadline: time.Now().Add(10 * time.Second).UTC().Truncate(time.Second)}
			body, err := ardp.EncodeJoinRequest(r)
			if err != nil {
				t.Fatal(err)
			}
			go func() { results <- pairs.Serve(ctx, observed[i], h, 32<<20, nil, capacities[i]) }()
			if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: body}); err != nil {
				t.Fatal(err)
			}
		}
		first, err := ardp.ReadFrame(peers[0])
		if err != nil {
			t.Fatal(err)
		}
		if first.Kind != ardp.KindResult || first.Lane != 1 {
			t.Fatal("missing local JOIN result")
		}
		if status, err := ardp.DecodeJoinResult(first.Body, [32]byte{1}); err != nil || status != 0 {
			t.Fatal("first local nonce or result differs", err)
		}
		synctest.Wait()
		before := observed[0].reads.Load()
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(peers[0], ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("opaque")})
		}()
		synctest.Wait()
		if observed[0].reads.Load() != before {
			t.Fatal("first side read data before second actual RESULT write")
		}
		select {
		case err := <-written:
			t.Fatal("data entered before pair barrier", err)
		default:
		}
		second, err := ardp.ReadFrame(peers[1])
		if err != nil {
			t.Fatal(err)
		}
		if status, err := ardp.DecodeJoinResult(second.Body, [32]byte{2}); err != nil || status != 0 {
			t.Fatal("second local nonce or result differs", err)
		}
		data, err := ardp.ReadFrame(peers[1])
		if err != nil || data.Kind != ardp.KindBytes || data.Lane != 1 || string(data.Body) != "opaque" {
			t.Fatal("paired opaque data not forwarded", err)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		cancel()
		for range 2 {
			if err := <-results; err == nil || err == io.EOF {
				t.Fatal("cancellation became successful completion", err)
			}
		}
		for _, capacity := range capacities {
			capacity.Release()
		}
		assertRetainedBudget(t, queues, 64<<20, 0, 0)
	})
}

// The second read has already entered physical I/O, but its result returns only
// when retirement interrupts it. This models late input completion without
// claiming any authority, Admission or genuine Carrier evidence.
type joinLateInput struct {
	net.Conn
	input    *bytes.Reader
	late     chan struct{}
	started  chan struct{}
	waitRead <-chan struct{}
	readOnce sync.Once
	once     sync.Once
	writes   int
}

func (c *joinLateInput) Read(p []byte) (int, error) {
	if c.waitRead != nil {
		<-c.waitRead
	}
	if c.started != nil {
		c.readOnce.Do(func() { close(c.started) })
	}
	if c.late != nil {
		<-c.late
	}
	return c.input.Read(p)
}
func (c *joinLateInput) Write(p []byte) (int, error) { c.writes++; return len(p), nil }
func (c *joinLateInput) SetDeadline(time.Time) error {
	if c.late != nil {
		c.once.Do(func() { close(c.late) })
	}
	return nil
}
func (c *joinLateInput) SetWriteDeadline(time.Time) error { return nil }
func (c *joinLateInput) Close() error                     { return nil }

func TestJoinCleanTerminalCannotEraseLateOppositeRefusal(t *testing.T) {
	for _, late := range []ardp.Frame{
		{Kind: ardp.KindClose, Lane: 1, Body: []byte{1}},
		{Kind: ardp.KindAdmit, Body: make([]byte, 355)},
	} {
		synctest.Test(t, func(t *testing.T) {
			owner := &Pairing{}
			p := &joinPair{owner: owner, stopped: make(chan struct{})}
			started := make(chan struct{})
			for i, frame := range []ardp.Frame{{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}, late} {
				raw, err := ardp.EncodeFrame(frame)
				if err != nil {
					t.Fatal(err)
				}
				c := &joinLateInput{input: bytes.NewReader(raw)}
				if i == 1 {
					c.late = make(chan struct{})
					c.started = started
				} else {
					// The opposite physical read, rather than goroutine launch,
					// must precede the first terminal in this late-result control.
					c.waitRead = started
				}
				p.sides[i] = &joinSide{ctx: context.Background(), conn: c, hello: ardp.Hello{Deadline: time.Now().Add(time.Minute)}, pair: p, limit: 1 << 20, credit: framing.Window}
			}
			p.relay(p.sides)
			if p.err == nil {
				t.Fatal("clean terminal erased the opposite late refusal")
			}
		})
	}
}

type joinCancelAtAttach struct {
	context.Context
	cancel context.CancelFunc
	calls  atomic.Int32
}

func (c *joinCancelAtAttach) Err() error {
	count := c.calls.Add(1)
	err := c.Context.Err()
	if count == 2 {
		// Capture a still-live final observation, then cancel before the
		// subsequent locked attachment. Context cancellation is real.
		c.cancel()
	}
	return err
}

func TestJoinCanceledAttachmentCannotLeaveUnownedPair(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		base, cancel := context.WithCancel(context.Background())
		defer cancel()
		owner := &Pairing{}
		ctx := &joinCancelAtAttach{Context: base, cancel: cancel}
		local, remote := net.Pipe()
		defer remote.Close()
		capacity, err := ReserveCapacity(framing.NewBudget(1 << 20))
		if err != nil {
			t.Fatal(err)
		}
		defer capacity.Release()
		end := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
		raw, err := ardp.EncodeJoinRequest(ardp.JoinRequest{Nonce: [32]byte{1}, Secret: [32]byte{2}, Context: [32]byte{3}, Side: 1, Deadline: end})
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			done <- owner.Serve(ctx, local, ardp.Hello{Purpose: ardp.PurposeDataJoin, Deadline: end}, 1<<20, nil, capacity)
		}()
		if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: raw}); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatal("missing caller refusal", err)
		}
		if len(owner.entries) != 0 {
			t.Fatal("canceled attachment left an unowned pair without timer or join")
		}
	})
}

type joinReadObservation struct {
	net.Conn
	reads atomic.Int32
}

func (c *joinReadObservation) Read(p []byte) (int, error) {
	c.reads.Add(1)
	return c.Conn.Read(p)
}

// Probe both finite resources through the same claims real borrowers use.
// Filling the expected remainder detects retained memory in either direction;
// zero-memory claims independently detect an early or leaked child return.
func assertRetainedBudget(t *testing.T, budget *framing.Budget, maximum, used uint64, children int) {
	t.Helper()
	remainder, err := budget.HoldChild(maximum - used)
	if err != nil {
		t.Fatal("expected remaining principal memory unavailable", err)
	}
	defer remainder()
	if extra, err := budget.HoldChild(1); err == nil {
		extra()
		t.Fatal("principal memory returned before its owner joined")
	}
	var claims []func()
	defer func() {
		for _, release := range claims {
			release()
		}
	}()
	for range 1023 - children {
		release, err := budget.HoldChild(0)
		if err != nil {
			t.Fatal("expected remaining child capacity unavailable", err)
		}
		claims = append(claims, release)
	}
	if extra, err := budget.HoldChild(0); err == nil {
		extra()
		t.Fatal("child returned before its owner joined")
	}
}

func containsPhysicalWrite(err error) bool {
	stage := framing.TerminalFailureStage(err)
	if stage == "physical-write" || stage == "peer-retired-write" {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if containsPhysicalWrite(child) {
				return true
			}
		}
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return containsPhysicalWrite(wrapped.Unwrap())
	}
	return false
}
