package route

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type poolCloseFixture struct {
	closedPoolCarrier
	calls   atomic.Uint32
	failure error
}

func (carrier *poolCloseFixture) Close() error {
	if carrier.calls.Add(1) > 1 {
		return net.ErrClosed
	}
	return carrier.failure
}

func TestClosedCarrierPoolSharesOneRetainedCloseResultWithBorrower(t *testing.T) {
	for _, failure := range []error{nil, errors.New("physical close failed")} {
		pool, err := NewClosedCarrierPool(time.Now)
		if err != nil {
			t.Fatal(err)
		}
		physical := &poolCloseFixture{failure: failure}
		lease, err := pool.Acquire(closedCarrierPoolKey(1), func() error { return nil }, func() (Carrier, error) { return physical, nil })
		if err != nil {
			t.Fatal(err)
		}
		borrowed, err := lease.Carrier()
		if err != nil {
			t.Fatal(err)
		}
		var workers sync.WaitGroup
		results := make(chan error, 9)
		for range 8 {
			workers.Go(func() { results <- borrowed.Close() })
		}
		workers.Go(func() { results <- pool.Close() })
		workers.Wait()
		close(results)
		for result := range results {
			if !errors.Is(result, failure) {
				t.Errorf("close outcome changed: got %v want %v", result, failure)
			}
		}
		if calls := physical.calls.Load(); calls != 1 {
			t.Errorf("physical carrier closed %d times", calls)
		}
	}
}
