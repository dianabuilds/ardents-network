//go:build !windows

package selection

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestSelectionRootLeaseRefusesHardLink is the causal G1 proof at the selection
// boundary: before the fix the Entry lease opened and flock'd a multiply-linked
// inode without an identity check, so a substituted lease file could become the
// exclusive fence. It now matches the duty and State durable strict identity.
func TestSelectionRootLeaseRefusesHardLink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "foreign")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, filepath.Join(root, rootLockName)); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireRootLease(root)
	if err == nil {
		_ = lease.release()
		t.Fatal("shared Entry lease inode accepted")
	}
	if !strings.Contains(err.Error(), "invalid history lease identity") {
		t.Fatalf("hardlink refusal = %v; want strict identity refusal", err)
	}
}

func TestSelectionRootLeaseRefusesNonRegularFile(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, rootLockName), 0o600); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireRootLease(root)
	if err == nil {
		_ = lease.release()
		t.Fatal("non-regular Entry lease accepted")
	}
	if !strings.Contains(err.Error(), "invalid history lease identity") {
		t.Fatalf("FIFO refusal = %v; want strict identity refusal", err)
	}
}

func TestSelectionRetainedRootLeaseRefusesHardLink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "foreign")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, filepath.Join(root, rootLockName)); err != nil {
		t.Fatal(err)
	}
	if lease, err := acquireRetainedRootLease(root); err == nil {
		_ = lease.release()
		t.Fatal("retained shared Entry lease inode accepted")
	}
}

func TestSelectionRootLeaseAcceptsRegularSingleLink(t *testing.T) {
	root := t.TempDir()
	lease, err := acquireRootLease(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.release(); err != nil {
		t.Fatalf("release = %v", err)
	}
}
