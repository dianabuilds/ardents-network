package channel

import (
	"context"
	"errors"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

func TestShortenedRoleBoundInterruptsAlreadyStartedCREDIT(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newCreditCloseFixture(t, 5*time.Second, nil)
		defer f.close()
		start := time.Now()
		if err := f.l.Bound(start.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		// Its real eight-byte prefix is already emitted; the peer withholds
		// consumption of the rest. Shortening the original lane horizon must
		// interrupt this original physical write, not wait its old five seconds.
		if err := <-f.consumed; !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatal("started CREDIT lost its physical deadline failure", err)
		}
		if time.Since(start) > time.Second {
			t.Fatal("started CREDIT outlived shortened role retirement bound", time.Since(start))
		}
		if err := f.s.Close(); !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatal("joined retirement erased physical CREDIT failure", err)
		}
	})
}

// The mechanical role direction has no TLS or authority claim. It reproduces
// CloseWrite's documented deadline change; actual TLS and both Carriers are
// exercised by the genuine command regression. Lower terminals here are real
// canonical frames over net.Pipe, not injected retirement flags.
type retirementRoleDirection struct {
	net.Conn
	finished atomic.Int32
}

func (c *retirementRoleDirection) NetConn() net.Conn { return c.Conn }
func (c *retirementRoleDirection) CloseWrite() error {
	c.finished.Add(1)
	return c.Conn.SetWriteDeadline(time.Now())
}

func TestRoleRetirementRequiresExactLowerTerminal(t *testing.T) {
	for _, terminal := range []string{"clean", "refusal", "raw-close", "cancel", "missing"} {
		t.Run(terminal, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				local, peer := net.Pipe()
				end := time.Now().Add(3 * time.Second)
				lower, lane, err := newJoinedSession(ctx, local, end, 1<<20, nil, NewBudget(4<<20))
				if err != nil {
					t.Fatal(err)
				}
				role := &retirementRoleDirection{Conn: lane}
				upper := New(ctx, role, end, 1<<20, nil, false, NewBudget(4<<20), nil)
				defer func() { _ = peer.Close(); _ = upper.Close(); _ = lower.Close() }()
				finished := make(chan error, 1)
				go func() { finished <- upper.FinishRole() }()
				frame, err := ardp.ReadFrame(peer)
				if err != nil || frame.Kind != ardp.KindEOF || frame.Lane != 1 || len(frame.Body) != 0 || role.finished.Load() != 1 {
					t.Fatal("inner completion did not precede canonical lower EOF", frame, err, role.finished.Load())
				}
				if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindEOF, Lane: 1}); err != nil {
					t.Fatal(err)
				}
				<-upper.Done()
				synctest.Wait()
				select {
				case err := <-finished:
					t.Fatal("reverse EOF alone completed role retirement", err)
				default:
				}
				if upper.Live() {
					t.Fatal("terminating role remained available for new work")
				}
				switch terminal {
				case "clean", "refusal":
					status := byte(0)
					if terminal == "refusal" {
						status = 1
					}
					if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{status}}); err != nil {
						t.Fatal(err)
					}
				case "raw-close":
					_ = peer.Close()
				case "cancel":
					cancel()
				}
				result := <-finished
				if (result == nil) != (terminal == "clean") {
					t.Fatal("role terminal classification", terminal, result)
				}
				if terminal == "cancel" && !errors.Is(result, context.Canceled) {
					t.Fatal("original cancellation lost", result)
				}
				if terminal == "missing" && time.Since(end.Add(-3*time.Second)) > time.Second {
					t.Fatal("cleanup exceeded its original one-second horizon")
				}
				joined := upper.Close()
				if (joined == nil) != (terminal == "clean") || upper.Close() != joined {
					t.Fatal("joined terminal was lost or replaced", terminal, result, joined)
				}
			})
		})
	}
}

func TestRolePeerRetirementJoinsStartedCREDITAndRetainsLateFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "clean", true: "late-failure"}[fail], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				late := errors.New("original CREDIT physical completion failed")
				gate := make(chan struct{})
				f := newCreditCloseFixture(t, 5*time.Second, func(c *creditCloseConn) {
					c.completion = gate
					if fail {
						c.writeFailure = late
					}
				})
				defer f.close()
				if err := ardp.WriteFrame(f.peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
					t.Fatal(err)
				}
				f.readCredit(t)
				joined := make(chan error, 1)
				go func() { joined <- f.l.waitPeerRetirement(t.Context()) }()
				synctest.Wait()
				select {
				case err := <-joined:
					t.Fatal("peer CLOSE completed before original CREDIT joined", err)
				default:
				}
				close(gate)
				result := <-joined
				if (result != nil) != fail {
					t.Fatal("late physical result incorrectly discharged", result)
				}
				if fail && !errors.Is(result, late) {
					t.Fatal("peer retirement lost original physical cause", result)
				}
				if err := f.s.Close(); fail && !errors.Is(err, late) {
					t.Fatal("physical failure vanished at joined Close", err)
				}
			})
		})
	}
}
