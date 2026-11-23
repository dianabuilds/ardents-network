//go:build !windows

package durablefile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func leaseFlags() int { return os.O_RDWR | os.O_CREATE | syscall.O_NOFOLLOW }

func TestOpenLeaseRetainsRegularSingleLinkFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	file, err := OpenLease(path, leaseFlags(), "", "busy")
	if err != nil {
		t.Fatal(err)
	}
	if err := ReleaseLease(file); err != nil {
		t.Fatalf("release = %v", err)
	}
	if err := ReleaseLease(nil); err != nil {
		t.Fatalf("nil release = %v", err)
	}
}

// TestOpenLeaseRefusesHardLink is the causal G1 proof: a multiply-linked lease
// inode is refused before it can become the exclusive fence.
func TestOpenLeaseRefusesHardLink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "foreign")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, filepath.Join(root, "lock")); err != nil {
		t.Fatal(err)
	}
	file, err := OpenLease(filepath.Join(root, "lock"), leaseFlags(), "", "busy")
	if err == nil {
		_ = ReleaseLease(file)
		t.Fatal("shared lease inode accepted")
	}
	if !strings.Contains(err.Error(), "invalid history lease identity") {
		t.Fatalf("hardlink refusal = %v; want identity refusal", err)
	}
}

func TestOpenLeaseRefusesNonRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := OpenLease(path, os.O_RDWR|syscall.O_NOFOLLOW, "", "busy")
	if err == nil {
		_ = ReleaseLease(file)
		t.Fatal("non-regular lease accepted")
	}
	if !strings.Contains(err.Error(), "invalid history lease identity") {
		t.Fatalf("FIFO refusal = %v; want identity refusal", err)
	}
}

func TestOpenLeaseRefusesSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "foreign")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "lock")); err != nil {
		t.Fatal(err)
	}
	if file, err := OpenLease(filepath.Join(root, "lock"), leaseFlags(), "", "busy"); err == nil {
		_ = ReleaseLease(file)
		t.Fatal("symlink lease accepted")
	}
}

func TestOpenLeaseRefusesContendedFlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	held, err := OpenLease(path, leaseFlags(), "", "acquire exclusive test lease")
	if err != nil {
		t.Fatal(err)
	}
	defer ReleaseLease(held)
	file, err := OpenLease(path, leaseFlags(), "", "acquire exclusive test lease")
	if err == nil {
		_ = ReleaseLease(file)
		t.Fatal("contended lease acquired")
	}
	if !strings.Contains(err.Error(), "acquire exclusive test lease") || !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("busy refusal = %v; want wrapped EWOULDBLOCK", err)
	}
}

func TestOpenLeaseWrapsOpenFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent-dir", "lock")
	_, err := OpenLease(path, leaseFlags(), "open test lease", "busy")
	if err == nil || !strings.Contains(err.Error(), "open test lease") {
		t.Fatalf("open failure = %v; want caller open wrap", err)
	}
}
