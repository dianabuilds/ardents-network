//go:build !windows

package publication

import (
	"errors"
	"os"
	"sync"
	"testing"
)

func TestCloseRetainsLeaseReleaseFailure(t *testing.T) {
	fixture := newPublicationFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Publish(t.Context(), fixture.input(t, 1)); err != nil {
		t.Fatal(err)
	}
	generation := owner.root.current
	if err := owner.root.lease.file.Close(); err != nil {
		t.Fatal(err)
	}
	first := owner.Close()
	if !errors.Is(first, os.ErrClosed) {
		t.Fatalf("Close = %v; want file-close failure", first)
	}
	if generation.signer != nil || owner.root.retiring != nil {
		t.Fatal("Close retained private generation")
	}
	for range 3 {
		if next := owner.Close(); next != first {
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
			if result := owner.Close(); result != first {
				t.Errorf("concurrent Close = %v; want retained %v", result, first)
			}
		}()
	}
	close(start)
	callers.Wait()

}
