package channel

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// These are physical framing oracles, not successful authority or admission
// fixtures. Concurrent forwarding callers share this same session operation.
func TestParentJoinsAlreadyStartedLaneReservationReturn(t *testing.T) {
	physical := newLifecycleConn(false)
	s := New(t.Context(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, NewBudget(64<<20), nil)
	defer s.Close()
	returning := make(chan struct{})
	finishReturn := make(chan struct{})
	var once sync.Once
	completeReturn := func() { once.Do(func() { close(finishReturn) }) }
	defer completeReturn()
	l, err := s.Open(t.Context(), t.Context(), ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}, true))
	if err != nil {
		t.Fatal(err)
	}
	if err := l.RetainUntilFinish(func() { close(returning); <-finishReturn }); err != nil {
		t.Fatal(err)
	}
	finished := make(chan struct{})
	go func() { l.Finish(); close(finished) }()
	<-returning
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	<-s.Done() // Parent retirement has actually joined its physical reader.
	select {
	case err := <-closed:
		t.Fatal("parent published completion before physical return joined", err)
	case <-time.After(25 * time.Millisecond):
	}
	completeReturn()
	<-finished
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
}

func TestJoinedParentReturnsUnfinishedLaneReservationOutsideLock(t *testing.T) {
	physical := newLifecycleConn(false)
	s := New(t.Context(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, NewBudget(64<<20), nil)
	defer s.Close()
	l, err := s.Open(t.Context(), t.Context(), ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}, true))
	if err != nil {
		t.Fatal(err)
	}
	returned := make(chan error, 2)
	if err := l.RetainUntilFinish(func() { returned <- l.SetReadDeadline(time.Now()) }); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("joined parent lost lane reservation return")
	}
	l.Finish()
	_ = s.Close()
	select {
	case <-returned:
		t.Fatal("parent and lane returned the same reservation twice")
	default:
	}
}

func TestOPENRejectsUnknownNodeRestrictionBeforeAllocationOrOutput(t *testing.T) {
	physical := newLifecycleConn(false)
	s := New(t.Context(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, NewBudget(64<<20), nil)
	defer s.Close()
	recipient := ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}, false)
	for _, restriction := range []byte{2, 255} {
		body := append(append([]byte(nil), recipient...), restriction)
		lane, err := s.Open(t.Context(), t.Context(), body)
		if lane != nil {
			_ = lane.Close()
			lane.Finish()
		}
		if err == nil || lane != nil {
			t.Fatal("unknown Node restriction reached lane allocation", restriction)
		}
	}
	select {
	case raw := <-physical.writes:
		t.Fatalf("invalid OPEN reached physical output: %x", raw)
	default:
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.next != 1 || s.live != 0 || s.used != 0 || len(s.lanes) != 0 {
		t.Fatal("invalid OPEN consumed lane identity or allowance")
	}
}

func TestLanePhysicalReturnWaitsForFinishAndRunsOutsideLock(t *testing.T) {
	physical := newLifecycleConn(false)
	s := New(t.Context(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, NewBudget(64<<20), nil)
	defer s.Close()
	l, err := s.Open(t.Context(), t.Context(), ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}, true))
	if err != nil {
		t.Fatal(err)
	}
	returned := make(chan error, 2)
	if err := l.RetainUntilFinish(func() { returned <- l.SetReadDeadline(time.Now()) }); err != nil {
		t.Fatal(err)
	}
	if err := l.RetainUntilFinish(func() { t.Error("second reservation taken") }); err == nil {
		t.Fatal("second physical return accepted")
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-returned:
		t.Fatal("reservation returned before owner finish")
	default:
	}
	l.Finish()
	if err := <-returned; err != nil {
		t.Fatal(err)
	}
	l.Finish()
	select {
	case <-returned:
		t.Fatal("reservation returned twice")
	default:
	}
	if err := l.RetainUntilFinish(func() { t.Error("finished lane took reservation") }); err == nil {
		t.Fatal("finished lane accepted physical return")
	}
}

func TestOPENRetainsOriginalCallerDeadlineAfterDerivedCancellation(t *testing.T) {
	physical := newLifecycleConn(false)
	s := New(context.Background(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &Budget{maximum: 64 << 20}, nil)
	defer s.Close()
	caller, cancelCaller := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancelCaller()
	child, cancelChild := context.WithCancel(t.Context())
	defer cancelChild()
	l, err := s.Open(child, caller, ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: time.Now().Add(40 * time.Second).UTC().Truncate(time.Second)}, true))
	if err != nil {
		t.Fatal(err)
	}
	<-physical.writes // The original OPEN is permitted before expiry.
	<-caller.Done()
	cancelChild()
	if child.Err() != context.Canceled || caller.Err() != context.DeadlineExceeded {
		t.Fatal("fixture did not preserve distinct cancellation reasons")
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-physical.writes:
		t.Fatal("derived cancellation lost original expiry and emitted terminal traffic", frame)
	default:
	}
}

func TestConcurrentOPENEmitsMonotonicLaneIDs(t *testing.T) {
	for range 16 {
		physical := newLifecycleConn(false)
		physical.writes = make(chan []byte, 128)
		s := New(context.Background(), physical, time.Now().Add(30*time.Second), 32<<20, nil, false, &Budget{maximum: 64 << 20}, nil)
		body := ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}, true)
		start := make(chan struct{})
		results := make(chan error, 64)
		var callers sync.WaitGroup
		for range 64 {
			callers.Add(1)
			go func() {
				defer callers.Done()
				<-start
				_, err := s.Open(t.Context(), t.Context(), body)
				results <- err
			}()
		}
		close(start)
		callers.Wait()
		close(results)
		for err := range results {
			if err != nil {
				_ = s.Close()
				t.Fatal(err)
			}
		}
		physical.mu.Lock()
		wire := bytes.Clone(physical.output)
		physical.mu.Unlock()
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		reader := bytes.NewReader(wire)
		var last uint32
		for range 64 {
			frame, err := ardp.ReadFrame(reader)
			if err != nil || frame.Kind != ardp.KindOpen || frame.Lane <= last {
				t.Fatalf("OPEN after lane %d: %+v %v", last, frame, err)
			}
			last = frame.Lane
		}
		if reader.Len() != 0 {
			t.Fatal("unexpected physical output")
		}
	}
}

func TestOPENSetupDeadlineDoesNotUseWholeChildLease(t *testing.T) {
	physical := newLifecycleConn(false)
	s := New(context.Background(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &Budget{maximum: 64 << 20}, nil)
	defer s.Close()
	childEnd := time.Now().Add(40 * time.Second).UTC().Truncate(time.Second)
	before := time.Now()
	l, err := s.Open(t.Context(), t.Context(), ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: childEnd}, true))
	if err != nil {
		t.Fatal(err)
	}
	physical.mu.Lock()
	deadline := physical.writeEnd
	physical.mu.Unlock()
	if deadline.After(before.Add(10*time.Second + 50*time.Millisecond)) {
		t.Fatal("OPEN borrowed whole child lifetime", deadline)
	}
	if l.hardEnd != childEnd || l.end != childEnd {
		t.Fatal("setup bound changed original child authority", l.hardEnd, l.end)
	}
	shorter := before.Add(5 * time.Second)
	if err := l.Bound(shorter); err != nil {
		t.Fatal(err)
	}
	if l.Deadline() != shorter {
		t.Fatal("physical deadline accessor lost shortened bound", l.Deadline())
	}
	if err := l.Bound(childEnd); err == nil || l.Deadline() != shorter {
		t.Fatal("physical deadline was renewed")
	}
}

func TestOPENWaiterCancellationPreservesActiveWriter(t *testing.T) {
	physical := newLifecycleConn(true)
	s := New(context.Background(), physical, time.Now().Add(30*time.Second), 32<<20, nil, false, &Budget{maximum: 64 << 20}, nil)
	defer s.Close()
	body := ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}, true)
	first := make(chan error, 1)
	go func() { _, err := s.Open(t.Context(), t.Context(), body); first <- err }()
	select {
	case <-physical.writes:
	case <-time.After(time.Second):
		t.Fatal("first OPEN did not start physical output")
	}
	physical.mu.Lock()
	before := physical.writeEnd
	physical.mu.Unlock()
	ctx, cancel := context.WithCancel(t.Context())
	second := make(chan error, 1)
	go func() { _, err := s.Open(ctx, ctx, body); second <- err }()
	cancel()
	if err := lifecycleResult(t, second); !errors.Is(err, context.Canceled) {
		t.Fatal("OPEN waiter cancellation lost", err)
	}
	physical.mu.Lock()
	after := physical.writeEnd
	physical.mu.Unlock()
	if after != before {
		t.Fatal("OPEN waiter changed sibling physical deadline", before, after)
	}
	close(physical.writeGate)
	if err := lifecycleResult(t, first); err != nil {
		t.Fatal("active OPEN failed after waiter cancellation", err)
	}
	select {
	case <-physical.writes:
		t.Fatal("canceled OPEN reached physical output")
	default:
	}
}
