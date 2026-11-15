//go:build !windows

package durable

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetainedLeaseRefusesSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "foreign")
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, rootLockName)); err != nil {
		t.Fatal(err)
	}
	if lease, err := acquireRetainedRootLease(root); err == nil {
		_ = lease.release()
		t.Fatal("foreign lease accepted")
	}
}

func TestRetainedLeaseRefusesHardLink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "foreign")
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, filepath.Join(root, rootLockName)); err != nil {
		t.Fatal(err)
	}
	if lease, err := acquireRetainedRootLease(root); err == nil {
		_ = lease.release()
		t.Fatal("shared lease inode accepted")
	}
}
