package join

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

// Actual framing provides socket metadata/deadline mechanisms. This incomplete
// acquisition only proves refusal; it supplies no accepting role or JOIN ACK.
func TestJoinedCanceledOriginalCannotReadWriteOrCloseWrite(t *testing.T) {
	caller, cancel := context.WithCancel(t.Context())
	a, claim := lifetimeAcquisition(t, caller)
	local, remote := net.Pipe()
	end := time.Now().Add(time.Minute)
	session, lane, err := newJoinedSession(t.Context(), local, end, 1<<20, nil, framing.NewBudget(4<<20))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	defer remote.Close()
	j := &Joined{acquisition: a, lane: lane}
	if j.LocalAddr() == nil || j.RemoteAddr() == nil {
		t.Fatal("physical socket metadata unavailable")
	}
	for _, set := range []func(time.Time) error{j.SetDeadline, j.SetReadDeadline, j.SetWriteDeadline} {
		if err := set(end.Add(time.Minute)); err != nil {
			t.Fatal("clamped physical deadline refused", err)
		}
		if err := set(end.Add(-time.Second)); err != nil {
			t.Fatal("bounded physical deadline refused", err)
		}
	}
	if err := j.SetDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if n, err := lane.Read(make([]byte, 1)); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("public deadline did not interrupt physical read", n, err)
	}
	if n, err := lane.Write([]byte{1}); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("public deadline did not deny physical output", n, err)
	}
	cancel()
	if n, err := j.Read(make([]byte, 1)); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled original read reached the lane", n, err)
	}
	if n, err := j.Write([]byte{1}); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled original write reached the lane", n, err)
	}
	if err := j.CloseWrite(); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled original emitted stream close", err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil || claim.returns.Load() != 1 {
		t.Fatal("original acquisition return was lost or repeated", err)
	}
}

func TestJoinedPhysicalCloseWaitsForInnerPeerBeforeLowerRetirement(t *testing.T) {
	for _, terminal := range []string{"accepted", "raw-EOF", "cleanup-timeout", "original-deadline"} {
		t.Run(terminal, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				started := time.Now()
				end := started.Add(time.Minute)
				if terminal == "original-deadline" {
					end = started.Add(250 * time.Millisecond)
				}
				local, remote := net.Pipe()
				// Actual framing lanes carry the nested dedicated session's bytes. No fake
				// successful net.Conn writes or authority callbacks replace physical I/O.
				budget := framing.NewBudget(4 << 20)
				lower, parent, err := newJoinedSession(context.Background(), local, end, 4<<20, nil, budget)
				if err != nil {
					t.Fatal(err)
				}
				defer lower.Close()
				opposite, peer, err := newJoinedSession(context.Background(), remote, end, 4<<20, nil, framing.NewBudget(4<<20))
				if err != nil {
					t.Fatal(err)
				}
				defer opposite.Close()
				inner, child, err := newJoinedSession(context.Background(), parent, end, 1<<20, nil, framing.NewBudget(4<<20))
				if err != nil {
					t.Fatal(err)
				}
				defer inner.Close()
				var returns atomic.Int32
				joined := &Joined{session: inner, lane: child, parent: &joinedParentRetirement{lane: parent, returned: &returns}, acquisition: &JoinAcquisition{ctx: context.Background()}, retiring: make(chan struct{}), watcherDone: make(chan struct{})}
				go joined.watch(context.Background())
				completed := make(chan error, 1)
				go func() { completed <- joined.closePhysical() }()
				frame, err := ardp.ReadFrame(peer)
				if err != nil || frame.Kind != ardp.KindClose || frame.Lane != 1 || frame.Body[0] != 0 {
					t.Fatalf("nested local terminal differs: %+v / %v", frame, err)
				}
				synctest.Wait()
				assertRetainedBudget(t, budget, 4<<20, 0, 1)
				if returns.Load() != 0 {
					t.Fatal("lower parent or acquisition returned before peer inner terminal")
				}
				select {
				case err := <-completed:
					t.Fatalf("physical close returned before peer inner terminal: %v", err)
				default:
				}
				switch terminal {
				case "accepted":
					if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
						t.Fatal(err)
					}
				case "raw-EOF":
					if err := peer.CloseWrite(); err != nil {
						t.Fatal(err)
					}
				}
				result := <-completed
				switch terminal {
				case "accepted":
					if result != nil {
						t.Fatal("authentic inner terminal did not permit clean retirement", result)
					}
				case "raw-EOF":
					if !errors.Is(result, io.ErrUnexpectedEOF) {
						t.Fatal("raw EOF erased missing inner terminal", result)
					}
				default:
					if !errors.Is(result, os.ErrDeadlineExceeded) && !errors.Is(result, context.DeadlineExceeded) {
						t.Fatal("missing peer terminal timeout became success", result)
					}
					expected := time.Second
					if terminal == "original-deadline" {
						expected = 250 * time.Millisecond
					}
					if elapsed := time.Since(started); elapsed != expected {
						t.Fatalf("inner terminal wait changed original bound: %s want %s", elapsed, expected)
					}
				}
				assertRetainedBudget(t, budget, 4<<20, 0, 0)
				if _, err := parent.Write([]byte{1}); err == nil {
					t.Fatal("retired parent accepted new output")
				}
				if returns.Load() != 1 {
					t.Fatal("joined cleanup did not return parent/acquisition exactly once")
				}
				if repeated := joined.closePhysical(); repeated != result || returns.Load() != 1 {
					t.Fatal("repeated cleanup changed terminal or returned twice")
				}
			})
		})
	}
}

// This observes the consumer's ordering over a real lower framing lane. It
// supplies no opening, authority or ACK; Prefix tests own its control claim.
type joinedParentRetirement struct {
	lane     *framing.Lane
	returned *atomic.Int32
}

func (p *joinedParentRetirement) CloseParent() error {
	err := p.lane.Close()
	p.lane.Finish()
	p.returned.Add(1)
	return err
}
