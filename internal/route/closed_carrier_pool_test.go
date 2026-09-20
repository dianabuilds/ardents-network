package route

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClosedCarrierPoolRetainsOnlyActualWorkForFixedWindow(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	pool, err := NewClosedCarrierPool(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	key := closedCarrierPoolKey(1)
	carrier := &closedPoolCarrier{}
	opens := 0
	open := func() (Carrier, error) { opens++; return carrier, nil }
	lease, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, open)
	if err != nil || opens != 1 {
		t.Fatalf("first acquisition = %v / opens %d", err, opens)
	}
	if err := lease.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	lease, err = pool.AcquireContext(context.Background(), key, func() error { return nil }, open)
	if err != nil || opens != 1 {
		t.Fatalf("retained acquisition = %v / opens %d", err, opens)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(closedCarrierRetention - time.Second)
	if err := pool.Reap(); err != nil || carrier.closes != 0 {
		t.Fatalf("early reap = %v / closes %d", err, carrier.closes)
	}
	now = now.Add(time.Second)
	if err := pool.Reap(); err != nil || carrier.closes != 1 {
		t.Fatalf("expiry reap = %v / closes %d", err, carrier.closes)
	}
	unused := &closedPoolCarrier{}
	lease, err = pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { return unused, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil || unused.closes != 1 {
		t.Fatalf("unused release = %v / closes %d", err, unused.closes)
	}
}

func TestClosedCarrierPoolRefusesInvalidStateAndBoundsEntries(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	pool, err := NewClosedCarrierPool(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	called := false
	if _, err := pool.AcquireContext(context.Background(), closedCarrierPoolKey(1), func() error { return errors.New("state changed") }, func() (Carrier, error) {
		called = true
		return &closedPoolCarrier{}, nil
	}); err == nil || called {
		t.Fatal("opened a Carrier after State validation failed")
	}
	for index := 1; index <= closedCarrierPoolMaximum; index++ {
		key := closedCarrierPoolKey(byte(index))
		lease, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { return &closedPoolCarrier{}, nil })
		if err != nil {
			t.Fatalf("acquire %d: %v", index, err)
		}
		if err := lease.MarkUsed(); err != nil {
			t.Fatal(err)
		}
		if err := lease.Release(); err != nil {
			t.Fatal(err)
		}
	}
	over := closedCarrierPoolKey(closedCarrierPoolMaximum + 1)
	if _, err := pool.AcquireContext(context.Background(), over, func() error { return nil }, func() (Carrier, error) { return &closedPoolCarrier{}, nil }); err == nil {
		t.Fatal("opened a Carrier beyond the Node pool limit")
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedCarrierPoolInvalidatesActiveLease(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	pool, err := NewClosedCarrierPool(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	key, carrier := closedCarrierPoolKey(1), &closedPoolCarrier{}
	lease, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { return carrier, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Invalidate(); err != nil || carrier.closes != 1 {
		t.Fatalf("invalidate = %v / closes %d", err, carrier.closes)
	}
	if err := lease.MarkUsed(); err == nil {
		t.Fatal("used an invalidated Carrier lease")
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
}

func closedCarrierPoolKey(peer byte) ClosedCarrierKey {
	return ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3}, PeerNodeID: [32]byte{peer + 10}, PeerKey: [32]byte{peer + 20}, CarrierProfile: ClosedCarrierTCP}
}

type closedPoolCarrier struct{ closes uint32 }

func (*closedPoolCarrier) Read([]byte) (int, error)        { return 0, io.EOF }
func (*closedPoolCarrier) Write(value []byte) (int, error) { return len(value), nil }
func (*closedPoolCarrier) SetDeadline(time.Time) error     { return nil }
func (carrier *closedPoolCarrier) Close() error            { carrier.closes++; return nil }

func TestClosedCarrierPoolRetiresChangedDirectedPairAndCannotReopenAfterClose(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	first := &closedPoolCarrier{}
	key := closedCarrierPoolKey(1)
	old, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { return first, nil })
	if err != nil {
		t.Fatal(err)
	}
	changed := key
	changed.PeerKey[0]++
	second := &closedPoolCarrier{}
	current, err := pool.AcquireContext(context.Background(), changed, func() error { return nil }, func() (Carrier, error) { return second, nil })
	if err != nil {
		t.Fatal(err)
	}
	if first.closes != 1 || len(pool.entries) != 1 {
		t.Fatal("changed pair retained two Carriers")
	}
	if _, err := old.Carrier(); err == nil {
		t.Fatal("old active lease retained changed State authority")
	}
	if err := old.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := current.Carrier(); err != nil {
		t.Fatal(err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	called := false
	if _, err := pool.AcquireContext(context.Background(), changed, func() error { return nil }, func() (Carrier, error) { called = true; return &closedPoolCarrier{}, nil }); err == nil || called {
		t.Fatal("withdrawn pool reopened a Carrier")
	}
}

func TestClosedCarrierLeaseLateInvalidationCannotCloseReplacement(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	key := closedCarrierPoolKey(1)
	oldCarrier := &closedPoolCarrier{}
	old, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { return oldCarrier, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Release(); err != nil {
		t.Fatal(err)
	}
	replacementCarrier := &closedPoolCarrier{}
	replacement, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { return replacementCarrier, nil })
	if err != nil {
		t.Fatal(err)
	}
	if old.SameCarrier(replacement) {
		t.Fatal("replacement reused retired incarnation")
	}
	if err := old.Invalidate(); err != nil {
		t.Fatal(err)
	}
	if replacementCarrier.closes != 0 {
		t.Fatal("late reader invalidated replacement")
	}
	if _, err := replacement.Carrier(); err != nil {
		t.Fatal(err)
	}
	shared, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { t.Fatal("redialed shared incarnation"); return nil, nil })
	if err != nil || !shared.SameCarrier(replacement) {
		t.Fatal("live borrowers disagree about incarnation")
	}
	if err := shared.Release(); err != nil {
		t.Fatal(err)
	}
	if err := replacement.Invalidate(); err != nil {
		t.Fatal(err)
	}
	if replacementCarrier.closes != 1 {
		t.Fatal("current incarnation was not invalidated")
	}
}

func TestClosedCarrierPoolBlockedDialDoesNotBlockOtherKeyOrCancelledWaiter(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	blocked, opened := make(chan struct{}), make(chan struct{})
	firstDone := make(chan error, 1)
	workerDone := make(chan struct{})
	var unblockOnce sync.Once
	unblock := func() { unblockOnce.Do(func() { close(blocked) }) }
	defer pool.Close()
	defer func() { <-workerDone }()
	defer unblock()
	key := closedCarrierPoolKey(1)
	go func() {
		defer close(workerDone)
		_, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) { close(opened); <-blocked; return &closedPoolCarrier{}, nil })
		firstDone <- err
	}()
	<-opened
	ready, err := pool.AcquireContext(context.Background(), closedCarrierPoolKey(2), func() error { return nil }, func() (Carrier, error) { return &closedPoolCarrier{}, nil })
	if err != nil {
		t.Fatalf("ready peer blocked: %v", err)
	}
	if err := ready.Release(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pool.AcquireContext(ctx, key, func() error { return nil }, func() (Carrier, error) { t.Fatal("same key redialed"); return nil, nil }); err == nil {
		t.Fatal("cancelled same-key waiter acquired")
	}
	unblock()
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

func TestClosedCarrierPoolSharesOneExactKeyOpening(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	opening, releaseDial, waiterValidated := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var validations atomic.Int32
	validate := func() error {
		if validations.Add(1) == 2 {
			close(waiterValidated)
		}
		return nil
	}
	var opens atomic.Int32
	open := func() (Carrier, error) {
		opens.Add(1)
		close(opening)
		<-releaseDial
		return &closedPoolCarrier{}, nil
	}
	key := closedCarrierPoolKey(1)
	firstDone, secondDone := make(chan *ClosedCarrierLease, 1), make(chan *ClosedCarrierLease, 1)
	go func() { lease, _ := pool.AcquireContext(context.Background(), key, validate, open); firstDone <- lease }()
	<-opening
	go func() {
		lease, _ := pool.AcquireContext(context.Background(), key, validate, open)
		secondDone <- lease
	}()
	<-waiterValidated
	close(releaseDial)
	first, second := <-firstDone, <-secondDone
	if first == nil || second == nil || !first.SameCarrier(second) || opens.Load() != 1 {
		t.Fatalf("exact-key opening leases = %v / %v, opens %d", first, second, opens.Load())
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}

type closedCarrierPoolWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (ctx *closedCarrierPoolWaitContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.waiting) })
	return ctx.Context.Done()
}

func TestClosedCarrierPoolRevalidatesAfterExactKeyWait(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	key := closedCarrierPoolKey(1)
	opened, releaseDial := make(chan struct{}), make(chan struct{})
	ownerDone := make(chan *ClosedCarrierLease, 1)
	ownerWorkerDone := make(chan struct{})
	var releaseDialOnce sync.Once
	unblockOwner := func() { releaseDialOnce.Do(func() { close(releaseDial) }) }
	defer pool.Close()
	defer func() { <-ownerWorkerDone }()
	defer unblockOwner()
	go func() {
		defer close(ownerWorkerDone)
		lease, _ := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (Carrier, error) {
			close(opened)
			<-releaseDial
			return &closedPoolCarrier{}, nil
		})
		ownerDone <- lease
	}()
	<-opened
	waiterWaiting := make(chan struct{})
	waiterContext := &closedCarrierPoolWaitContext{Context: context.Background(), waiting: waiterWaiting}
	var stateChanged atomic.Bool
	var validations atomic.Int32
	waiterDone := make(chan error, 1)
	waiterWorkerDone := make(chan struct{})
	defer func() { <-waiterWorkerDone }()
	defer unblockOwner()
	go func() {
		defer close(waiterWorkerDone)
		_, err := pool.AcquireContext(waiterContext, key, func() error {
			if validations.Add(1) == 1 {
				return nil
			}
			if !stateChanged.Load() {
				return errors.New("State revalidated before the test changed it")
			}
			return errors.New("State changed while waiting")
		}, func() (Carrier, error) { return nil, errors.New("waiter redialed") })
		waiterDone <- err
	}()
	<-waiterWaiting
	stateChanged.Store(true)
	unblockOwner()
	owner := <-ownerDone
	if owner == nil {
		t.Fatal("opening owner did not acquire")
	}
	defer owner.Release()
	if err := <-waiterDone; err == nil {
		t.Fatal("waiter acquired after State changed")
	}
	if got := validations.Load(); got != 2 {
		t.Fatalf("waiter validations = %d, want 2", got)
	}
	if err := owner.Release(); err != nil {
		t.Fatal(err)
	}
}

type closedCarrierPoolLateCloseCarrier struct {
	closeEntered chan struct{}
	allowClose   chan struct{}
}

func (*closedCarrierPoolLateCloseCarrier) Read([]byte) (int, error)        { return 0, io.EOF }
func (*closedCarrierPoolLateCloseCarrier) Write(value []byte) (int, error) { return len(value), nil }
func (*closedCarrierPoolLateCloseCarrier) SetDeadline(time.Time) error     { return nil }
func (carrier *closedCarrierPoolLateCloseCarrier) Close() error {
	close(carrier.closeEntered)
	<-carrier.allowClose
	return nil
}

func TestClosedCarrierPoolCloseWaitsForLateDialCarrierDisposal(t *testing.T) {
	pool, err := NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	opening, releaseDial := make(chan struct{}), make(chan struct{})
	carrier := &closedCarrierPoolLateCloseCarrier{closeEntered: make(chan struct{}), allowClose: make(chan struct{})}
	acquireDone := make(chan error, 1)
	acquireWorkerDone := make(chan struct{})
	var releaseDialOnce, allowCloseOnce sync.Once
	unblockDial := func() { releaseDialOnce.Do(func() { close(releaseDial) }) }
	finishCarrierClose := func() { allowCloseOnce.Do(func() { close(carrier.allowClose) }) }
	defer pool.Close()
	defer finishCarrierClose()
	defer func() { <-acquireWorkerDone }()
	defer unblockDial()
	go func() {
		defer close(acquireWorkerDone)
		_, err := pool.AcquireContext(context.Background(), closedCarrierPoolKey(1), func() error { return nil }, func() (Carrier, error) {
			close(opening)
			<-releaseDial
			return carrier, nil
		})
		acquireDone <- err
	}()
	<-opening
	closeDone := make(chan error, 1)
	closeWorkerDone := make(chan struct{})
	defer func() {
		unblockDial()
		finishCarrierClose()
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
	unblockDial()
	<-carrier.closeEntered
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before late Carrier disposal completed: %v", err)
	case <-time.After(250 * time.Millisecond):
	}
	finishCarrierClose()
	if err := <-acquireDone; err == nil {
		t.Fatal("late dial was accepted after pool withdrawal")
	}
	if err := <-closeDone; err != nil {
		t.Fatal(err)
	}
}
