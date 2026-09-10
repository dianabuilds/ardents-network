package route

import (
	"errors"
	"io"
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
	lease, err := pool.Acquire(key, func() error { return nil }, open)
	if err != nil || opens != 1 {
		t.Fatalf("first acquisition = %v / opens %d", err, opens)
	}
	if err := lease.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	lease, err = pool.Acquire(key, func() error { return nil }, open)
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
	lease, err = pool.Acquire(key, func() error { return nil }, func() (Carrier, error) { return unused, nil })
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
	if _, err := pool.Acquire(closedCarrierPoolKey(1), func() error { return errors.New("state changed") }, func() (Carrier, error) {
		called = true
		return &closedPoolCarrier{}, nil
	}); err == nil || called {
		t.Fatal("opened a Carrier after State validation failed")
	}
	for index := 1; index <= closedCarrierPoolMaximum; index++ {
		key := closedCarrierPoolKey(byte(index))
		lease, err := pool.Acquire(key, func() error { return nil }, func() (Carrier, error) { return &closedPoolCarrier{}, nil })
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
	if _, err := pool.Acquire(over, func() error { return nil }, func() (Carrier, error) { return &closedPoolCarrier{}, nil }); err == nil {
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
	lease, err := pool.Acquire(key, func() error { return nil }, func() (Carrier, error) { return carrier, nil })
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
	old, err := pool.Acquire(key, func() error { return nil }, func() (Carrier, error) { return first, nil })
	if err != nil {
		t.Fatal(err)
	}
	changed := key
	changed.PeerKey[0]++
	second := &closedPoolCarrier{}
	current, err := pool.Acquire(changed, func() error { return nil }, func() (Carrier, error) { return second, nil })
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
	if _, err := pool.Acquire(changed, func() error { return nil }, func() (Carrier, error) { called = true; return &closedPoolCarrier{}, nil }); err == nil || called {
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
	old, err := pool.Acquire(key, func() error { return nil }, func() (Carrier, error) { return oldCarrier, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Release(); err != nil {
		t.Fatal(err)
	}
	replacementCarrier := &closedPoolCarrier{}
	replacement, err := pool.Acquire(key, func() error { return nil }, func() (Carrier, error) { return replacementCarrier, nil })
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
	shared, err := pool.Acquire(key, func() error { return nil }, func() (Carrier, error) { t.Fatal("redialed shared incarnation"); return nil, nil })
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
