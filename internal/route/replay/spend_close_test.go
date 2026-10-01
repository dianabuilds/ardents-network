//go:build !windows

package replay

import (
	"errors"
	"os"
	"sync"
	"testing"
)

func TestCloseRetainsJoinedFailure(t *testing.T) {
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	ledger, err := Open(t.TempDir(), binding)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("ambiguous journal write")
	ledger.failure = failure
	if err := ledger.lease.file.Close(); err != nil {
		t.Fatal(err)
	}
	first := ledger.Close()
	if !errors.Is(first, failure) || !errors.Is(first, os.ErrClosed) {
		t.Fatalf("Close = %v; want journal and release causes", first)
	}
	for range 3 {
		if next := ledger.Close(); next != first {
			t.Fatalf("repeated Close = %v; want retained %v", next, first)
		}
	}
	start := make(chan struct{})
	var callers sync.WaitGroup
	for range 32 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			<-start
			if result := ledger.Close(); result != first {
				t.Errorf("concurrent Close = %v; want retained %v", result, first)
			}
		}()
	}
	close(start)
	callers.Wait()

}
