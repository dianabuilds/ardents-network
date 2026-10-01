//go:build !windows

package reachability

import (
	"errors"
	"os"
	"sync"
	"testing"
)

func TestConcurrentCloseRetainsLeaseReleaseResult(t *testing.T) {
	for _, broken := range []bool{false, true} {
		name := "success"
		if broken {
			name = "release-failure"
		}
		t.Run(name, func(t *testing.T) {
			config := StoreConfig{Root: t.TempDir(), NetworkID: [32]byte{1}}
			store, err := OpenStore(config)
			if err != nil {
				t.Fatal(err)
			}
			if broken {
				if err := store.lease.file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			start := make(chan struct{})
			results := make(chan error, 32)
			var callers sync.WaitGroup
			for range 32 {
				callers.Add(1)
				go func() {
					defer callers.Done()
					<-start
					result := store.Close()
					store.mu.Lock()
					released := store.lease.file == nil
					store.mu.Unlock()
					if !released {
						t.Error("Close returned before physical lease release")
					}
					results <- result
				}()
			}
			close(start)
			callers.Wait()
			close(results)
			first := <-results
			if broken && !errors.Is(first, os.ErrClosed) {
				t.Fatalf("Close = %v; want file-close failure", first)
			}
			if !broken && first != nil {
				t.Fatal(first)
			}
			for result := range results {
				if result != first {
					t.Fatalf("concurrent Close = %v; want retained %v", result, first)
				}
			}
			if result := store.Close(); result != first {
				t.Fatalf("repeated Close = %v; want %v", result, first)
			}
			reopened, err := OpenStore(config)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
