package join

import (
	"context"
	"encoding/binary"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"io"
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

// Hold a real incoming control at either completed-header or completed-body
// observation. Sealing cannot turn joined CREDIT into payload or grant credit.
type joinRelayInputGate struct {
	net.Conn
	reads, hold      int
	observed, resume chan struct{}
}

func (c *joinRelayInputGate) Read(body []byte) (int, error) {
	n, err := c.Conn.Read(body)
	c.reads++
	if c.reads == c.hold && n != 0 {
		close(c.observed)
		<-c.resume
	}
	return n, err
}

func TestJoinRelaySealJoinsStartedCreditWithoutGrantOrOutput(t *testing.T) {
	for _, phase := range []struct {
		name string
		read int
	}{{"header", 1}, {"body", 2}} {
		for _, name := range []string{"valid", "zero", "overflow", "payload", "currentness-loss", "cancellation"} {
			t.Run(phase.name+"/"+name, func(t *testing.T) {
				local, remote := net.Pipe()
				defer local.Close()
				defer remote.Close()
				gate := &joinRelayInputGate{Conn: local, hold: phase.read, observed: make(chan struct{}), resume: make(chan struct{})}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				pair := &joinPair{owner: &Pairing{}}
				end := time.Now().Add(time.Minute)
				side := &joinSide{ctx: ctx, conn: gate, pair: pair, hello: ardp.Hello{Deadline: end}, limit: 65536, credit: framing.Window}
				peer := &joinSide{ctx: t.Context(), pair: pair, hello: ardp.Hello{Deadline: end}, limit: 65536, credit: framing.Window - 1}
				lost := errors.New("original incoming CREDIT observation lost")
				side.check = func() error {
					pair.owner.mu.Lock()
					sealed := pair.sealed
					pair.owner.mu.Unlock()
					if sealed && name == "currentness-loss" {
						return lost
					}
					return nil
				}
				frame := ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: []byte{0, 0, 0, 1}}
				if name == "overflow" {
					frame.Body[3] = 2
				}
				if name == "payload" {
					frame.Kind = ardp.KindBytes
				}
				raw, err := ardp.EncodeFrame(frame)
				if err != nil {
					t.Fatal(err)
				}
				if name == "zero" {
					raw[len(raw)-1] = 0
				} // deliberately invalid incoming bytes
				result := make(chan joinPumpResult, 1)
				go func() { result <- side.pump(peer) }()
				written := make(chan error, 1)
				go func() { _, err := remote.Write(raw); written <- err }()
				<-gate.observed
				pair.owner.mu.Lock()
				pair.sealed = true
				pair.owner.mu.Unlock()
				if name == "cancellation" {
					cancel()
				}
				close(gate.resume)
				got := <-result
				if name == "valid" {
					if got.err != nil || got.terminal || got.write {
						t.Errorf("joined started CREDIT failed retirement: %+v", got)
					}
				} else if got.err == nil {
					t.Error("invalid or revoked input became clean retirement", got)
				}
				if name == "currentness-loss" && !errors.Is(got.err, lost) {
					t.Error("original observation failure erased", got.err)
				}
				if name == "cancellation" && !errors.Is(got.err, context.Canceled) {
					t.Error("original cancellation erased", got.err)
				}
				if peer.credit != framing.Window-1 || peer.used != 0 || peer.physicalErr != nil {
					t.Error("sealed input granted credit or started output")
				}
				local.Close()
				<-written
			})
		}
	}
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

// Pause the opposite pump's actual pre-header currentness observation. A peer
// CLOSE can seal the pair before that observation returns, so no new input may
// start. These real pipes isolate relay ordering, not successful authority or
// Admission. A separate currentness failure must still prevent clean output.
func TestJoinRelaySealBeforeOppositeHeaderStillEmitsTerminals(t *testing.T) {
	for _, name := range []string{"clean", "refused", "opposite-currentness-loss"} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				first, firstPeer := net.Pipe()
				second, secondPeer := net.Pipe()
				defer firstPeer.Close()
				defer secondPeer.Close()
				defer first.Close()
				defer second.Close()
				pair := &joinPair{owner: &Pairing{}, stopped: make(chan struct{})}
				end := time.Now().Add(time.Minute)
				observing, resume := make(chan struct{}), make(chan struct{})
				var once sync.Once
				lost := errors.New("original opposite currentness lost")
				sides := [2]*joinSide{
					{ctx: t.Context(), conn: first, pair: pair, hello: ardp.Hello{Purpose: ardp.PurposeDataJoin, Deadline: end}, limit: 1 << 20, credit: framing.Window},
					{ctx: t.Context(), conn: second, pair: pair, hello: ardp.Hello{Purpose: ardp.PurposeDataJoin, Deadline: end}, limit: 1 << 20, credit: framing.Window},
				}
				sides[1].check = func() error {
					once.Do(func() { close(observing); <-resume })
					if name == "opposite-currentness-loss" {
						return lost
					}
					return nil
				}
				joined := make(chan struct{})
				go func() {
					pair.relay(sides)
					_ = first.Close()
					_ = second.Close()
					close(joined)
				}()
				<-observing
				status := byte(0)
				if name == "refused" {
					status = 1
				}
				if err := ardp.WriteFrame(firstPeer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{status}}); err != nil {
					t.Fatal(err)
				}
				<-pair.stopped
				synctest.Wait()
				type terminal struct {
					frame ardp.Frame
					err   error
				}
				terminals := make(chan terminal, 2)
				for _, peer := range []net.Conn{firstPeer, secondPeer} {
					go func() { frame, err := ardp.ReadFrame(peer); terminals <- terminal{frame, err} }()
				}
				close(resume)
				for range 2 {
					got := <-terminals
					if name == "opposite-currentness-loss" {
						if got.err == nil {
							t.Error("currentness loss produced terminal output", got.frame)
						}
					} else if got.err != nil || got.frame.Kind != ardp.KindClose || got.frame.Lane != 1 || len(got.frame.Body) != 1 || got.frame.Body[0] != status {
						t.Errorf("sealed pre-header pump prevented matching terminal: %+v / %v", got.frame, got.err)
					}
				}
				<-joined
				switch name {
				case "clean":
					if pair.err != nil {
						t.Fatal("ordinary seal became a failed pair", pair.err)
					}
				case "refused":
					if pair.err == nil {
						t.Fatal("peer refusal was erased")
					}
				case "opposite-currentness-loss":
					if !errors.Is(pair.err, lost) {
						t.Fatal("original currentness failure was erased", pair.err)
					}
				}
			})
		})
	}
}

// A genuine input CREDIT is fully read and validated while live. Pause only
// its recipient's pre-output observation, then consume the opposite CLOSE.
// The pair must join this unemitted control and still send both terminals.
// Pipe framing supplies no successful Network, Admission or Service authority.
func TestJoinRelaySealAfterCreditValidationJoinsUnstartedOutput(t *testing.T) {
	for _, name := range []string{"clean", "late-check-failure", "original-cancellation"} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				first, firstPeer := net.Pipe()
				second, secondPeer := net.Pipe()
				defer first.Close()
				defer second.Close()
				defer firstPeer.Close()
				defer secondPeer.Close()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				pair := &joinPair{owner: &Pairing{}, stopped: make(chan struct{})}
				end := time.Now().Add(time.Minute)
				sides := [2]*joinSide{
					{ctx: ctx, conn: first, pair: pair, hello: ardp.Hello{Purpose: ardp.PurposeDataJoin, Deadline: end}, limit: 1 << 20, credit: framing.Window - 1},
					{ctx: context.Background(), conn: second, pair: pair, hello: ardp.Hello{Purpose: ardp.PurposeDataJoin, Deadline: end}, limit: 1 << 20, credit: framing.Window},
				}
				initial, observing, resume := make(chan struct{}), make(chan struct{}), make(chan struct{})
				lost := errors.New("original pre-output observation failed")
				var checkMu sync.Mutex
				checks := 0
				sides[0].check = func() error {
					checkMu.Lock()
					checks++
					n := checks
					checkMu.Unlock()
					if n == 1 {
						close(initial) // original input header; no frame sent yet
					}
					if n == 2 {
						close(observing) // already validated CREDIT, before output
						<-resume
						if name == "late-check-failure" {
							return lost
						}
					}
					return nil
				}
				joined := make(chan struct{})
				go func() {
					pair.relay(sides)
					_ = first.Close()
					_ = second.Close()
					close(joined)
				}()
				<-initial
				if err := ardp.WriteFrame(secondPeer, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: []byte{0, 0, 0, 1}}); err != nil {
					t.Fatal(err)
				}
				<-observing
				if err := ardp.WriteFrame(firstPeer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
					t.Fatal(err)
				}
				<-pair.stopped
				if name == "original-cancellation" {
					cancel()
				}
				type terminal struct {
					frame ardp.Frame
					err   error
				}
				terminals := make(chan terminal, 2)
				for _, peer := range []net.Conn{firstPeer, secondPeer} {
					go func() { f, err := ardp.ReadFrame(peer); terminals <- terminal{f, err} }()
				}
				close(resume)
				for range 2 {
					got := <-terminals
					if name == "clean" {
						if got.err != nil || got.frame.Kind != ardp.KindClose || got.frame.Lane != 1 || len(got.frame.Body) != 1 || got.frame.Body[0] != 0 {
							t.Errorf("unstarted CREDIT prevented clean terminal: %+v / %v", got.frame, got.err)
						}
					} else if got.err != io.EOF {
						t.Errorf("failed original emitted terminal or partial output: %+v / %v", got.frame, got.err)
					}
				}
				<-joined
				switch name {
				case "clean":
					if pair.err != nil || sides[0].physicalErr != nil || sides[1].physicalErr != nil {
						t.Fatal("unemitted control became failed physical retirement", pair.err)
					}
				case "late-check-failure":
					if !errors.Is(pair.err, lost) {
						t.Fatal("late original failure was erased", pair.err)
					}
				case "original-cancellation":
					if !errors.Is(pair.err, context.Canceled) {
						t.Fatal("original cancellation was erased", pair.err)
					}
				}
			})
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
