//go:build !windows

package issuer

import (
	"bytes"
	"errors"
	"os"
	"sync"
	"testing"
)

func TestClosedTokenIssuerCloseRetainsLeaseReleaseFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireIssuerRootLease(root)
	if err != nil {
		t.Fatal(err)
	}
	private := []byte{1, 2, 3}
	issuer := &ClosedTokenIssuer{lease: lease, material: closedIssuerMaterial{keys: []closedIssuerPrivateKey{{der: private}}}}
	if err := lease.file.Close(); err != nil {
		t.Fatal(err)
	}
	first := issuer.Close()
	if first == nil {
		t.Fatalf("Close = %v; want file-close failure", first)
	}
	if issuer.material.keys != nil || !bytes.Equal(private, make([]byte, len(private))) {
		t.Fatal("Close retained private material")
	}
	for range 3 {
		if next := issuer.Close(); next != first {
			t.Fatalf("repeated Close = %v; want retained %v", next, first)
		}
	}
	if !errors.Is(first, os.ErrClosed) {
		t.Fatalf("Close lost file-close cause: %v", first)
	}
	start := make(chan struct{})
	var callers sync.WaitGroup
	for range 32 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			<-start
			if result := issuer.Close(); result != first {
				t.Errorf("concurrent Close = %v; want retained %v", result, first)
			}
		}()
	}
	close(start)
	callers.Wait()

}
