package channel

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Hold the original physical deadline call before Write begins, then seal the
// exact payload lane. This mechanism grants no TLS or domain authority. A late
// deadline return must not start output or retire a healthy sibling's parent.
func TestPayloadSealDuringDeadlineRefusesBeforePhysicalWrite(t *testing.T) {
	physical := &heldPhysicalDeadline{lifecycleConn: newLifecycleConn(false), armed: true,
		entered: make(chan struct{}), resume: make(chan struct{})}
	owner := Prepare(t.Context(), physical, time.Now().Add(5*time.Second), 1<<20, nil, false, NewBudget(4<<20), Handlers{})
	lane := lifecycleLane(t, owner, 1)
	written := make(chan error, 1)
	var once sync.Once
	resume := func() { once.Do(func() { close(physical.resume) }) }
	joined := false
	t.Cleanup(func() {
		resume()
		if !joined {
			<-written
		}
		_ = owner.Close()
	})
	go func() { _, err := lane.Write([]byte("retired payload")); written <- err }()
	select {
	case <-physical.entered:
	case <-time.After(time.Second):
		t.Fatal("original deadline call did not start")
	}
	lane.Seal()
	resume()
	err := <-written
	joined = true
	if !errors.Is(err, net.ErrClosed) {
		t.Fatal("sealed payload did not retain its local refusal", err)
	}
	select {
	case <-physical.writes:
		t.Fatal("sealed payload entered physical output after deadline observation")
	default:
	}
	if !owner.Live() {
		t.Fatal("unstarted payload retired its healthy parent")
	}
	sibling := lifecycleLane(t, owner, 3)
	if n, err := sibling.Write([]byte("live sibling")); n != len("live sibling") || err != nil {
		t.Fatal("original parent cannot serve its live sibling", n, err)
	}
	if err := lane.Close(); err != nil {
		t.Fatal("original rejected lane did not join", err)
	}
	if cleanup, complete := lane.CleanupResult(); !complete || cleanup != nil {
		t.Fatal("unstarted payload invented a physical cleanup failure", cleanup, complete)
	}
}

// The actual OPEN allocator retains its local ID even when canceled before
// Write. Cleanup must not emit CLOSE for an OPEN that the peer never received.
func TestOPENCanceledDuringDeadlineEmitsNeitherOPENNorCLOSE(t *testing.T) {
	physical := &heldPhysicalDeadline{lifecycleConn: newLifecycleConn(false), armed: true,
		entered: make(chan struct{}), resume: make(chan struct{})}
	end := time.Now().Add(5 * time.Second).UTC().Truncate(time.Second)
	owner := Prepare(t.Context(), physical, end, 1<<20, nil, false, NewBudget(4<<20), Handlers{})
	caller, cancel := context.WithCancel(t.Context())
	opened := make(chan error, 1)
	var once sync.Once
	resume := func() { once.Do(func() { close(physical.resume) }) }
	joined := false
	t.Cleanup(func() {
		cancel()
		resume()
		if !joined {
			<-opened
		}
		_ = owner.Close()
	})
	body := ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: end}, false)
	go func() {
		lane, err := owner.Open(caller, caller, body)
		if lane != nil {
			err = errors.Join(err, errors.New("canceled OPEN returned a lane"), lane.Close())
			lane.Finish()
		}
		opened <- err
	}()
	select {
	case <-physical.entered:
	case <-time.After(time.Second):
		t.Fatal("OPEN did not reach its original deadline effect")
	}
	owner.mu.Lock()
	lane := owner.active
	owner.mu.Unlock()
	if lane == nil {
		t.Fatal("OPEN did not retain its original lane")
	}
	cancel()
	waitFramingState(t, lane, func() bool { return lane.localClosed && !lane.cleanupEnd.IsZero() })
	resume()
	err := <-opened
	joined = true
	if !errors.Is(err, context.Canceled) {
		t.Fatal("original OPEN caller failure lost", err)
	}
	select {
	case raw := <-physical.writes:
		t.Fatalf("canceled unstarted OPEN emitted a frame: %x", raw)
	default:
	}
	owner.mu.Lock()
	retained := owner.next == 3 && owner.live == 0 && len(owner.lanes) == 0 && !lane.openEmitted
	owner.mu.Unlock()
	if !retained || !owner.Live() {
		t.Fatal("unstarted OPEN reused its ID, retained its child or retired its parent")
	}
}
