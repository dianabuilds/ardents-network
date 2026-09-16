package route

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestClosedCarrierPoolSerializesChangedDirectedPairPublication(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	firstKey := closedCarrierPoolKey(1)
	secondKey := firstKey
	secondKey.PeerKey[0]++
	firstOpened, releaseFirst := make(chan struct{}), make(chan struct{})
	firstDone := make(chan *ClosedCarrierLease, 1)
	firstWorkerDone, secondWorkerDone := make(chan struct{}), make(chan struct{})
	var releaseFirstOnce sync.Once
	unblockFirst := func() { releaseFirstOnce.Do(func() { close(releaseFirst) }) }
	defer pool.Close()
	defer func() { <-firstWorkerDone; <-secondWorkerDone }()
	defer unblockFirst()
	go func() {
		defer close(firstWorkerDone)
		lease, _ := pool.AcquireContext(context.Background(), firstKey, func() error { return nil }, func() (Carrier, error) {
			close(firstOpened)
			<-releaseFirst
			return &closedPoolCarrier{}, nil
		})
		firstDone <- lease
	}()
	<-firstOpened
	secondOpened := make(chan struct{}, 1)
	secondDone := make(chan *ClosedCarrierLease, 1)
	go func() {
		defer close(secondWorkerDone)
		lease, _ := pool.AcquireContext(context.Background(), secondKey, func() error { return nil }, func() (Carrier, error) {
			secondOpened <- struct{}{}
			return &closedPoolCarrier{}, nil
		})
		secondDone <- lease
	}()
	select {
	case <-secondOpened:
		t.Fatal("changed directed pair opened while the original pair was still opening")
	default:
	}
	unblockFirst()
	first, second := <-firstDone, <-secondDone
	if first == nil || second == nil {
		t.Fatal("changed directed pair did not complete both acquisitions")
	}
	if _, err := first.Carrier(); err == nil {
		t.Fatal("previous directed-pair incarnation remained live after replacement")
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}

type closedCarrierPoolBlockedRetirement struct {
	closeEntered chan struct{}
	allowClose   chan struct{}
}

func (*closedCarrierPoolBlockedRetirement) Read([]byte) (int, error)        { return 0, io.EOF }
func (*closedCarrierPoolBlockedRetirement) Write(value []byte) (int, error) { return len(value), nil }
func (*closedCarrierPoolBlockedRetirement) SetDeadline(time.Time) error     { return nil }
func (carrier *closedCarrierPoolBlockedRetirement) Close() error {
	close(carrier.closeEntered)
	<-carrier.allowClose
	return nil
}

func TestClosedCarrierPoolCloseJoinsChangedPairRetirement(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	old := &closedCarrierPoolBlockedRetirement{closeEntered: make(chan struct{}), allowClose: make(chan struct{})}
	var unblockOldOnce sync.Once
	unblockOld := func() { unblockOldOnce.Do(func() { close(old.allowClose) }) }
	defer func() {
		unblockOld()
		_ = pool.Close()
	}()
	firstKey := closedCarrierPoolKey(1)
	first, err := pool.AcquireContext(context.Background(), firstKey, func() error { return nil }, func() (Carrier, error) { return old, nil })
	if err != nil {
		t.Fatal(err)
	}
	secondKey := firstKey
	secondKey.PeerKey[0]++
	secondDone := make(chan error, 1)
	secondWorkerDone := make(chan struct{})
	defer func() {
		unblockOld()
		<-secondWorkerDone
	}()
	go func() {
		defer close(secondWorkerDone)
		_, err := pool.AcquireContext(context.Background(), secondKey, func() error { return nil }, func() (Carrier, error) { return &closedPoolCarrier{}, nil })
		secondDone <- err
	}()
	<-old.closeEntered
	closeDone := make(chan error, 1)
	closeWorkerDone := make(chan struct{})
	defer func() {
		unblockOld()
		<-closeWorkerDone
	}()
	go func() {
		defer close(closeWorkerDone)
		closeDone <- pool.Close()
	}()
	for {
		pool.mu.Lock()
		closed := pool.closed
		pool.mu.Unlock()
		if closed {
			break
		}
		runtime.Gosched()
	}
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before changed-pair retirement completed: %v", err)
	case <-time.After(250 * time.Millisecond):
	}
	unblockOld()
	if err := <-secondDone; err == nil {
		t.Fatal("replacement acquired after pool withdrawal")
	}
	if err := <-closeDone; err != nil {
		t.Fatal(err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedCarrierPoolExpiresIdleEntryDuringAcquire(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	pool, err := NewClosedCarrierPool(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	key := closedCarrierPoolKey(1)
	firstCarrier, secondCarrier := &closedPoolCarrier{}, &closedPoolCarrier{}
	first, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { return firstCarrier, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := first.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(closedCarrierRetention)
	second, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { return secondCarrier, nil })
	if err != nil {
		t.Fatal(err)
	}
	if firstCarrier.closes != 1 || first.SameCarrier(second) {
		t.Fatalf("expired entry reused: closes %d, same %t", firstCarrier.closes, first.SameCarrier(second))
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedCarrierPoolRefusesCancelledReadyBorrow(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	key := closedCarrierPoolKey(1)
	carrier := &closedPoolCarrier{}
	lease, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { return carrier, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opened := false
	if _, err := pool.AcquireContext(ctx, key, func() error { return nil }, func() (Carrier, error) {
		opened = true
		return &closedPoolCarrier{}, nil
	}); err == nil || opened {
		t.Fatal("cancelled caller borrowed a ready Carrier")
	}
}

func TestClosedCarrierPoolDisposesLateCancelledDial(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx, cancel := context.WithCancel(context.Background())
	carrier := &closedPoolCarrier{}
	if _, err := pool.AcquireContext(ctx, closedCarrierPoolKey(1), func() error { return nil }, func() (Carrier, error) {
		cancel()
		return carrier, nil
	}); err == nil {
		t.Fatal("cancelled dial published a Carrier")
	}
	if carrier.closes != 1 {
		t.Fatalf("late cancelled Carrier closes = %d, want 1", carrier.closes)
	}
}

func TestClosedCarrierPoolReapAfterCloseDoesNotStartRetirement(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	pool, err := NewClosedCarrierPool(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	key := closedCarrierPoolKey(1)
	carrier := &closedPoolCarrier{}
	lease, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { return carrier, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	if carrier.closes != 1 {
		t.Fatalf("Close closes = %d, want 1", carrier.closes)
	}
	now = now.Add(closedCarrierRetention)
	if err := pool.Reap(); err != nil {
		t.Fatal(err)
	}
	if carrier.closes != 1 {
		t.Fatalf("Reap after Close started a second retirement: closes %d", carrier.closes)
	}
}

func TestClosedCarrierPoolCloseRejectsReadyLeaseWhileJoiningPendingDial(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := pool.AcquireContext(context.Background(), closedCarrierPoolKey(1), func() error { return nil }, func() (Carrier, error) { return &closedPoolCarrier{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := ready.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	opened, release := make(chan struct{}), make(chan struct{})
	pendingDone := make(chan error, 1)
	pendingWorkerDone := make(chan struct{})
	var releaseOnce sync.Once
	unblockPending := func() { releaseOnce.Do(func() { close(release) }) }
	go func() {
		defer close(pendingWorkerDone)
		_, acquireErr := pool.AcquireContext(context.Background(), closedCarrierPoolKey(2), func() error { return nil }, func() (Carrier, error) {
			close(opened)
			<-release
			return &closedPoolCarrier{}, nil
		})
		pendingDone <- acquireErr
	}()
	<-opened
	defer pool.Close()
	defer func() { <-pendingWorkerDone }()
	defer unblockPending()
	closeDone := make(chan error, 1)
	closeWorkerDone := make(chan struct{})
	defer func() {
		unblockPending()
		<-closeWorkerDone
	}()
	go func() {
		defer close(closeWorkerDone)
		closeDone <- pool.Close()
	}()
	for {
		pool.mu.Lock()
		closed := pool.closed
		pool.mu.Unlock()
		if closed {
			break
		}
		runtime.Gosched()
	}
	if _, err := ready.Carrier(); err == nil {
		t.Fatal("ready lease remained usable after pool withdrawal")
	}
	if err := ready.MarkUsed(); err == nil {
		t.Fatal("ready lease recorded work after pool withdrawal")
	}
	unblockPending()
	if err := <-pendingDone; err == nil {
		t.Fatal("pending dial published after pool withdrawal")
	}
	if err := <-closeDone; err != nil {
		t.Fatal(err)
	}
	if err := ready.Release(); err != nil {
		t.Fatal(err)
	}
}

type closedCarrierPoolDisposalError struct{ err error }

func (*closedCarrierPoolDisposalError) Read([]byte) (int, error)        { return 0, io.EOF }
func (*closedCarrierPoolDisposalError) Write(value []byte) (int, error) { return len(value), nil }
func (*closedCarrierPoolDisposalError) SetDeadline(time.Time) error     { return nil }
func (carrier *closedCarrierPoolDisposalError) Close() error            { return carrier.err }

func TestClosedCarrierPoolCloseRetainsReadyDisposalError(t *testing.T) {
	for _, test := range []struct {
		name    string
		dispose func(*ClosedCarrierLease) error
	}{
		{"release", func(lease *ClosedCarrierLease) error { return lease.Release() }},
		{"invalidate", func(lease *ClosedCarrierLease) error { return lease.Invalidate() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			pool, err := NewClosedCarrierPool(time.Now)
			if err != nil {
				t.Fatal(err)
			}
			want := errors.New("ready Carrier disposal failed")
			ready, err := pool.AcquireContext(context.Background(), closedCarrierPoolKey(1), func() error { return nil }, func() (Carrier, error) { return &closedCarrierPoolDisposalError{err: want}, nil })
			if err != nil {
				t.Fatal(err)
			}
			opened, release := make(chan struct{}), make(chan struct{})
			pending := make(chan error, 1)
			go func() {
				_, err := pool.AcquireContext(context.Background(), closedCarrierPoolKey(2), func() error { return nil }, func() (Carrier, error) { close(opened); <-release; return &closedPoolCarrier{}, nil })
				pending <- err
			}()
			<-opened
			closed := make(chan error, 1)
			go func() { closed <- pool.Close() }()
			for {
				pool.mu.Lock()
				withdrawing := pool.closed
				pool.mu.Unlock()
				if withdrawing {
					break
				}
				runtime.Gosched()
			}
			if err := test.dispose(ready); !errors.Is(err, want) {
				t.Fatalf("dispose = %v, want %v", err, want)
			}
			close(release)
			if err := <-pending; err == nil {
				t.Fatal("pending dial published after withdrawal")
			}
			if err := <-closed; !errors.Is(err, want) {
				t.Fatalf("Close = %v, want retained %v", err, want)
			}
		})
	}
}
