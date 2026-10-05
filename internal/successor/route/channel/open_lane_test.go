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
