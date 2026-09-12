package duty

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOperationLeaseCancellationPreservesExclusiveOwner(t *testing.T) {
	root := localRoleFixtureRoot(t)
	config := Config{Root: root, Clock: time.Now, Create: true}
	owner, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()
	if got, err := OpenOperation(ctx, config); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		if got != nil {
			_ = got.Close()
		}
		t.Fatalf("cancelled waiter: %v", err)
	}
	if duplicate, err := Open(config); err == nil {
		_ = duplicate.Close()
		t.Fatal("waiter cancellation released another owner's lease")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOperationLeaseHasItsOwnFiniteWait(t *testing.T) {
	root := localRoleFixtureRoot(t)
	config := Config{Root: root, Clock: time.Now, Create: true}
	owner, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	started := time.Now()
	got, err := OpenOperation(context.Background(), config)
	elapsed := time.Since(started)
	if got != nil {
		_ = got.Close()
		t.Fatal("duplicate owner acquired")
	}
	if !errors.Is(err, context.DeadlineExceeded) || elapsed < operationLeaseWait || elapsed > 3*time.Second {
		t.Fatalf("unbounded or premature lease wait: %v after %s", err, elapsed)
	}
}

func TestCancelledOperationDoesNotCreateRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := OpenOperation(ctx, Config{Root: root, Clock: time.Now, Create: true}); got != nil || !errors.Is(err, context.Canceled) {
		if got != nil {
			_ = got.Close()
		}
		t.Fatalf("cancelled creation: %v", err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("cancelled creation touched root: %v", err)
	}
}

func TestOperationDoesNotWaitForUnclaimedRoot(t *testing.T) {
	root := localRoleFixtureRoot(t)
	lease, err := acquireRootLease(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.release()
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()
	got, err := OpenOperation(ctx, Config{Root: root, Clock: time.Now, Create: true})
	if got != nil {
		_ = got.Close()
		t.Fatal("unclaimed root was adopted")
	}
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("incomplete claim became a busy-root wait: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != rootLockName {
		t.Fatalf("unclaimed root changed: %v, %v", entries, err)
	}
}
