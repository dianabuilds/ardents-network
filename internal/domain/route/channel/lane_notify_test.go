package channel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
)

// The reusable notification (audit RD-4) must wake every registered waiter,
// coalesce signals while an owner rechecks state, and stop notifying a forgotten
// waiter without retaining it. A signal with no waiters is a no-op.
func TestLaneNotificationBroadcastCoalesceAndForget(t *testing.T) {
	l := &Lane{}
	l.signalLocked()
	first := l.notifyLocked()
	second := l.notifyLocked()
	l.signalLocked()
	for _, w := range []*laneWaiter{first, second} {
		select {
		case <-w.ch:
		default:
			t.Fatal("registered waiter was not notified")
		}
	}
	// Several signals while the owner is not selecting coalesce to one token.
	l.signalLocked()
	l.signalLocked()
	for _, w := range []*laneWaiter{first, second} {
		select {
		case <-w.ch:
		default:
			t.Fatal("coalesced notification absent")
		}
		select {
		case <-w.ch:
			t.Fatal("coalescing retained more than one pending token")
		default:
		}
	}
	l.forgetLocked(first)
	l.signalLocked()
	select {
	case <-first.ch:
		t.Fatal("forgotten waiter still notified")
	default:
	}
	select {
	case <-second.ch:
	default:
		t.Fatal("remaining waiter was not notified")
	}
	l.forgetLocked(second)
	if len(l.waiters) != 0 {
		t.Fatalf("forgotten waiters retained: %d", len(l.waiters))
	}
}

// A signal that fires after registration but before the owner selects must be
// retained by the buffered token, so the wait cannot lose the wakeup.
func TestLaneNotificationRetainsSignalBeforeSelect(t *testing.T) {
	l := &Lane{}
	w := l.notifyLocked()
	l.signalLocked()
	select {
	case <-w.ch:
	case <-time.After(time.Second):
		t.Fatal("signal between registration and select was lost")
	}
	l.forgetLocked(w)
}

// One timer reused across waits (audit RD-4) must honor each successive deadline
// rather than deliver a stale fire from a prior iteration's selected wake.
func TestWaitLaneReusedTimerHonorsSuccessiveDeadlines(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var timer laneTimer
		defer timer.stop()
		changed := make(chan struct{}, 1)
		// A wake well before a long deadline leaves that timer running.
		changed <- struct{}{}
		if err := waitLane(changed, time.Now().Add(time.Hour), &timer); err != nil {
			t.Fatal(err)
		}
		// The same timer reused for a short deadline must wait for the new
		// deadline, not return early from the prior long deadline.
		start := time.Now()
		if err := waitLane(changed, time.Now().Add(20*time.Millisecond), &timer); !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("reused timer did not honor the new deadline: %v", err)
		}
		if time.Since(start) < 20*time.Millisecond {
			t.Fatal("reused timer delivered a stale early fire")
		}
		// An already-expired deadline refuses immediately without waiting.
		if err := waitLane(changed, time.Now().Add(-time.Second), &timer); !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("expired deadline not refused: %v", err)
		}
		// A wake on the reused timer still returns nil before the deadline.
		changed <- struct{}{}
		if err := waitLane(changed, time.Now().Add(time.Hour), &timer); err != nil {
			t.Fatal(err)
		}
	})
}

// A credit-starved Write that wakes once per granted byte reuses the same waiter
// and timer across many wait iterations (audit RD-4) and still delivers every
// byte intact through the genuine peer CREDIT decode path.
func TestLaneWriteReusesWaiterAcrossManyCreditWaits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, peer := net.Pipe()
		s := New(t.Context(), local, time.Now().Add(time.Minute), 1<<20, nil, false, NewBudget(4<<20), nil)
		defer func() { _ = peer.Close(); _ = s.Close() }()
		lane := lifecycleLane(t, s, 1)
		s.mu.Lock()
		lane.credit = 0
		s.mu.Unlock()
		const total = 64
		body := bytes.Repeat([]byte{7}, total)
		written := make(chan error, 1)
		go func() { _, err := lane.Write(body); written <- err }()
		peerDone := make(chan struct{})
		go func() {
			defer close(peerDone)
			grant := func() bool {
				return ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, 1)}) == nil
			}
			if !grant() {
				return
			}
			for received := 0; received < total; {
				f, err := ardp.ReadFrame(peer)
				if err != nil {
					return
				}
				if f.Kind == ardp.KindBytes {
					received += len(f.Body)
				}
				if received < total && !grant() {
					return
				}
			}
		}()
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		<-peerDone
	})
}
