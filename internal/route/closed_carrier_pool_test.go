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
		lease, err := pool.Acquire(closedCarrierPoolKey(byte(index)), func() error { return nil }, func() (Carrier, error) { return &closedPoolCarrier{}, nil })
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
	if _, err := pool.Acquire(closedCarrierPoolKey(33), func() error { return nil }, func() (Carrier, error) { return &closedPoolCarrier{}, nil }); err == nil {
		t.Fatal("opened a Carrier beyond the Node pool limit")
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedCarrierPoolInvalidatesActiveStateKey(t *testing.T) {
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
	if err := pool.Invalidate(key); err != nil || carrier.closes != 1 {
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
