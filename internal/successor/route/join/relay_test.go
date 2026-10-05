package join

import (
	"encoding/binary"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"net"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// Pause an actual pipe read after the original pump has consumed its header,
// before it can account or interpret the body. No authority is supplied here.
type joinRelayHeaderGate struct {
	net.Conn
	header, resume chan struct{}
	once           sync.Once
}

func (c *joinRelayHeaderGate) Read(body []byte) (int, error) {
	n, err := c.Conn.Read(body)
	if n != 0 {
		c.once.Do(func() {
			close(c.header)
			<-c.resume
		})
	}
	return n, err
}

func TestJoinRelaySealKeepsAlreadyStartedOppositeTerminal(t *testing.T) {
	for _, test := range []struct {
		name     string
		frame    ardp.Frame
		terminal bool
	}{
		{"clean close", ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}, true},
		{"refused close", ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{1}}, true},
		{"payload", ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{9}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			local, remote := net.Pipe()
			defer local.Close()
			defer remote.Close()
			gate := &joinRelayHeaderGate{Conn: local, header: make(chan struct{}), resume: make(chan struct{})}
			pair := &joinPair{owner: &Pairing{}}
			side := &joinSide{ctx: t.Context(), conn: gate, pair: pair, hello: ardp.Hello{Deadline: time.Now().Add(time.Minute)}, limit: 65536}
			result := make(chan joinPumpResult, 1)
			go func() { result <- side.pump(nil) }()
			written := make(chan error, 1)
			go func() { written <- ardp.WriteFrame(remote, test.frame) }()
			<-gate.header
			pair.owner.mu.Lock()
			pair.sealed = true
			pair.owner.mu.Unlock()
			close(gate.resume)
			got := <-result
			if test.terminal {
				if !got.terminal || got.err != nil || got.status != test.frame.Body[0] {
					t.Errorf("original terminal became a framing failure after opposite seal: %+v", got)
				}
			} else if got.terminal || got.err == nil {
				t.Error("sealed pair admitted new payload", got)
			}
			local.Close()
			<-written
		})
	}
}

// These controls exercise actual post-admission frame I/O. They grant no
// Network, Admission, Hosting or Service authority; genuine composition is
// separately covered by the both-Carrier command scenarios.
func TestJoinRelayRefusesActivatedControlWithoutForwarding(t *testing.T) {
	request, err := ardp.EncodeJoinRequest(joinBoundsRequest(1, 9))
	if err != nil {
		t.Fatal(err)
	}
	for name, frame := range map[string]ardp.Frame{
		"lane-zero-refill": {Kind: ardp.KindAdmit, Body: make([]byte, 355)},
		"lane-one-refill":  {Kind: ardp.KindAdmit, Lane: 1, Body: make([]byte, 355)},
		"second-join":      {Kind: ardp.KindOperation, Lane: 1, Body: request},
		"unknown-lane":     {Kind: ardp.KindBytes, Lane: 3, Body: []byte{1}},
		"unearned-credit":  {Kind: ardp.KindCredit, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, 1)},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fixture := newJoinBoundsFixture()
				defer fixture.close(t)
				first := fixture.start(t, joinBoundsRequest(1, 1), nil)
				second := fixture.start(t, joinBoundsRequest(2, 2), nil)
				readJoinBoundsResult(t, first, 1)
				readJoinBoundsResult(t, second, 2)
				written := make(chan error, 1)
				go func() { written <- ardp.WriteFrame(first.conn, frame) }()
				if forwarded, err := ardp.ReadFrame(second.conn); err == nil {
					t.Fatal("forbidden frame reached opposite side", forwarded)
				}
				<-written // an early header refusal can interrupt the body write
				if first.wait() == nil || second.wait() == nil {
					t.Fatal("forbidden frame became successful joined completion")
				}
				assertRetainedBudget(t, fixture.queues, 64<<20, 2*joinFrameMemory, 2)
			})
		})
	}
}

func TestJoinRelayRefusesBytesBeyondOriginalReceiveCredit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		first := fixture.start(t, joinBoundsRequest(1, 1), nil)
		second := fixture.start(t, joinBoundsRequest(2, 2), nil)
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		for range 4 {
			written := make(chan error, 1)
			go func() {
				written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: make([]byte, 16384)})
			}()
			frame, err := ardp.ReadFrame(second.conn)
			if err != nil || frame.Kind != ardp.KindBytes || len(frame.Body) != 16384 {
				t.Fatal("original 64 KiB credit unavailable", err)
			}
			if err := <-written; err != nil {
				t.Fatal(err)
			}
		}
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1}})
		}()
		if frame, err := ardp.ReadFrame(second.conn); err == nil {
			t.Fatal("uncredited fifth frame reached opposite side", frame)
		}
		<-written
		if first.wait() == nil || second.wait() == nil {
			t.Fatal("receive-credit exhaustion became successful completion")
		}
	})
}

func TestJoinCapacityAggregateRefusalRetainsOriginalBorrowers(t *testing.T) {
	queues := framing.NewBudget(2 * joinFrameMemory)
	first, err := ReserveCapacity(queues)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReserveCapacity(queues)
	if err != nil {
		t.Fatal(err)
	}
	if third, err := ReserveCapacity(queues); err == nil || third != nil {
		t.Fatal("aggregate exhaustion acquired another side")
	}
	assertRetainedBudget(t, queues, 2*joinFrameMemory, 2*joinFrameMemory, 2)
	first.Release()
	first.Release()
	assertRetainedBudget(t, queues, 2*joinFrameMemory, joinFrameMemory, 1)
	second.Release()
	assertRetainedBudget(t, queues, 2*joinFrameMemory, 0, 0)
}

func TestJoinRelayOriginalByteAllowanceCountsSetupAndHeaders(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		// Independent wire accounting: HELLO/ADMIT/ACCEPT, JOIN and RESULT,
		// one single-byte data frame, then only the next header can fit.
		const limit = (3*16 + 209 + 355 + 5) + (16 + 4096) + (16 + 16384) + (16 + 1) + 16
		first := fixture.startWithBounds(t, joinBoundsRequest(1, 1), nil, [32]byte{3}, limit)
		second := fixture.start(t, joinBoundsRequest(2, 2), nil)
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{7}})
		}()
		frame, err := ardp.ReadFrame(second.conn)
		if err != nil || frame.Kind != ardp.KindBytes || len(frame.Body) != 1 || frame.Body[0] != 7 {
			t.Fatal("original remaining allowance was unavailable", frame, err)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		go func() {
			written <- ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{8}})
		}()
		if frame, err := ardp.ReadFrame(second.conn); err == nil {
			t.Fatal("bytes beyond original allowance were forwarded", frame)
		}
		<-written
		if first.wait() == nil || second.wait() == nil {
			t.Fatal("byte exhaustion became successful completion")
		}
		assertRetainedBudget(t, fixture.queues, 64<<20, 2*joinFrameMemory, 2)
	})
}

// These real pipe controls isolate post-admission relay termination. They do
// not supply Network, token, Hosting or Service authority.
type joinRelayCreditConn struct {
	net.Conn
	credit, interrupted       chan struct{}
	creditOnce, interruptOnce sync.Once
}

func TestJoinRelayPeerCloseCannotWaitBeyondCreditCleanupBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		first := fixture.start(t, joinBoundsRequest(1, 1), nil)
		var observed *joinRelayCreditConn
		second := fixture.start(t, joinBoundsRequest(2, 2), func(conn net.Conn) net.Conn {
			observed = &joinRelayCreditConn{Conn: conn, credit: make(chan struct{}), interrupted: make(chan struct{})}
			return observed
		})
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(second.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{92}})
		}()
		if frame, err := ardp.ReadFrame(first.conn); err != nil || frame.Kind != ardp.KindBytes {
			t.Fatal("consumed data absent", err)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		if err := ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: []byte{0, 0, 0, 1}}); err != nil {
			t.Fatal(err)
		}
		<-observed.credit
		start := time.Now()
		if err := ardp.WriteFrame(second.conn, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatal(err)
		}
		<-observed.interrupted
		// No peer consumes the selected CREDIT. Its actual physical timeout
		// must fail the pair and return, rather than grant unbounded cleanup.
		for _, peer := range []*joinBoundsPeer{first, second} {
			if err := peer.wait(); !errors.Is(err, os.ErrDeadlineExceeded) {
				t.Fatal("failed CREDIT was erased", err)
			}
		}
		if elapsed := time.Since(start); elapsed != time.Second {
			t.Fatal("CREDIT cleanup did not retain its one-second bound", elapsed)
		}
	})
}

func (c *joinRelayCreditConn) Write(body []byte) (int, error) {
	if len(body) >= ardp.HeaderSize && body[6] == ardp.KindCredit {
		c.creditOnce.Do(func() { close(c.credit) })
	}
	return c.Conn.Write(body)
}

func (c *joinRelayCreditConn) SetDeadline(end time.Time) error {
	err := c.Conn.SetDeadline(end)
	if !end.After(time.Now()) {
		c.interruptOnce.Do(func() { close(c.interrupted) })
	}
	return err
}

func (c *joinRelayCreditConn) SetReadDeadline(end time.Time) error {
	err := c.Conn.SetReadDeadline(end)
	if !end.After(time.Now()) {
		c.interruptOnce.Do(func() { close(c.interrupted) })
	}
	return err
}

func TestJoinRelayPeerCloseAllowsSelectedCreditToJoin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		first := fixture.start(t, joinBoundsRequest(1, 1), nil)
		var observed *joinRelayCreditConn
		second := fixture.start(t, joinBoundsRequest(2, 2), func(conn net.Conn) net.Conn {
			observed = &joinRelayCreditConn{Conn: conn, credit: make(chan struct{}), interrupted: make(chan struct{})}
			return observed
		})
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(second.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{91}})
		}()
		data, err := ardp.ReadFrame(first.conn)
		if err != nil || data.Kind != ardp.KindBytes || len(data.Body) != 1 || data.Body[0] != 91 {
			t.Fatal("actual consumed data absent", err)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		var credit [4]byte
		binary.BigEndian.PutUint32(credit[:], 1)
		if err := ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: credit[:]}); err != nil {
			t.Fatal(err)
		}
		<-observed.credit
		if err := ardp.WriteFrame(second.conn, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatal(err)
		}
		<-observed.interrupted
		firstTerminal := make(chan error, 1)
		go func() {
			frame, err := ardp.ReadFrame(first.conn)
			if err == nil && (frame.Kind != ardp.KindClose || len(frame.Body) != 1 || frame.Body[0] != 0) {
				err = net.ErrClosed
			}
			firstTerminal <- err
		}()
		frame, err := ardp.ReadFrame(second.conn)
		if err != nil || frame.Kind != ardp.KindCredit || len(frame.Body) != 4 || binary.BigEndian.Uint32(frame.Body) != 1 {
			t.Fatal("selected CREDIT was interrupted instead of joined", err)
		}
		frame, err = ardp.ReadFrame(second.conn)
		if err != nil || frame.Kind != ardp.KindClose || len(frame.Body) != 1 || frame.Body[0] != 0 {
			t.Fatal("opposite terminal absent", err)
		}
		if err := <-firstTerminal; err != nil {
			t.Fatal("first terminal absent", err)
		}
		if err := first.wait(); err != nil {
			t.Fatal("first side failed retirement", err)
		}
		if err := second.wait(); err != nil {
			t.Fatal("second side failed retirement", err)
		}
	})
}
