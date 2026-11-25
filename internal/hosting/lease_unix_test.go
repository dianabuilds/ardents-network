//go:build linux || darwin

package hosting

import (
	"context"
	"testing"
	"time"
)

// The exclusive root lease is a Unix-family flock fence. Each acquire opens the
// lock file anew, so a second acquire is a distinct open file description and
// cannot take the exclusive lock while the first holds it; releasing the first
// makes the fence available again. This exercises the same guarantee on Linux
// and macOS (BSD flock per-open-file-description semantics) on the native
// temporary filesystem.
func TestHostingLeaseIsExclusiveAcrossOpens(t *testing.T) {
	rootPath, _, _ := hostingFixture(t)
	root, err := openHostingRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx := t.Context()
	first, err := acquireHostingLease(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if second, err := acquireHostingLease(short, root); err == nil {
		_ = second.close()
		t.Fatal("contended lease acquired while the exclusive lock was held")
	}
	if err := first.close(); err != nil {
		t.Fatal(err)
	}
	third, err := acquireHostingLease(ctx, root)
	if err != nil {
		t.Fatalf("lease unavailable after release: %v", err)
	}
	if err := third.close(); err != nil {
		t.Fatal(err)
	}
}
