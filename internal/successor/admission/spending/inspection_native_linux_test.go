//go:build linux && network_signing_native

package spending

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"testing"
	"time"
)

// This root/ext4 component uses actual spend storage and native leases. It
// supplies no signed State, installed Node, original retirement or recovery.
func nativeSpendInspection(t *testing.T) (string, Binding, []byte, time.Time) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: root inspection driver required")
	}
	root := t.TempDir()
	var fs syscall.Statfs_t
	if err := syscall.Statfs(root, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: private native ext4 TMPDIR required", err)
	}
	for parent := filepath.Dir(root); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
			t.Fatal("invalid environment: root-controlled inspection ancestors required", parent, err)
		}
		if parent == "/" {
			break
		}
	}
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	ledger, err := Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour).Add(time.Minute)
	raw := bytes.Repeat([]byte{5}, 354)
	if err := ledger.Spend(raw, now.Truncate(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	return root, binding, raw, now
}

func TestSpendInspectionNativeOriginalHistoryAndLease(t *testing.T) {
	root, binding, token, now := nativeSpendInspection(t)
	path := filepath.Join(root, closedSpendLedgerName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	inspection, err := OpenInspection(ctx, root, binding, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer inspection.Close()
	copy := inspection
	for range 3 {
		if err := copy.Check(); err != nil {
			t.Fatal(err)
		}
	}
	foreign, err := OpenRetained(root, binding)
	if foreign != nil {
		_ = foreign.Close()
	}
	if err == nil {
		t.Fatal("inspection lent its original receiving lease")
	}
	if err := inspection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	if err := copy.Check(); err == nil {
		t.Fatal("closed copy revived")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("read-only inspection mutated history", err)
	}
	ledger, err := OpenRetained(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	if err := ledger.Spend(token, now.Truncate(time.Hour), now); err == nil {
		t.Fatal("inspection refunded original burn")
	}
}

func TestSpendInspectionNativeRefusesWithoutRepairOrCreation(t *testing.T) {
	for _, fault := range []string{"missing-lock", "missing-journal", "crash-tail", "foreign-binding", "wrong-principal", "extra-file", "journal-link"} {
		t.Run(fault, func(t *testing.T) {
			root, binding, _, _ := nativeSpendInspection(t)
			path := filepath.Join(root, closedSpendLedgerName)
			uid := uint32(0)
			switch fault {
			case "missing-lock":
				if err := os.Remove(filepath.Join(root, closedSpendLockName)); err != nil {
					t.Fatal(err)
				}
			case "missing-journal":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "crash-tail":
				file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, err = file.Write([]byte{1})
				if err := errors.Join(err, file.Close()); err != nil {
					t.Fatal(err)
				}
			case "foreign-binding":
				binding.ReceiverDutyGeneration++
			case "wrong-principal":
				uid = 123
			case "extra-file":
				if err := os.WriteFile(filepath.Join(root, "unknown"), []byte{1}, 0600); err != nil {
					t.Fatal(err)
				}
			case "journal-link":
				if err := os.Rename(path, path+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+"-original", path); err != nil {
					t.Fatal(err)
				}
			}
			before, beforeErr := os.ReadFile(path)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			for range 2 {
				opened, err := OpenInspection(ctx, root, binding, uid, 0)
				if opened.original != nil {
					_ = opened.Close()
				}
				if err == nil || opened.original != nil {
					t.Fatal("unavailable history inspected", fault)
				}
			}
			after, afterErr := os.ReadFile(path)
			if !bytes.Equal(before, after) || (beforeErr == nil) != (afterErr == nil) {
				t.Fatal("refusal repaired or created history")
			}
			if fault == "missing-lock" {
				if _, err := os.Lstat(filepath.Join(root, closedSpendLockName)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("inspection recreated lease", err)
				}
			}
		})
	}
}

func TestSpendInspectionNativeDenialRetainsOriginalUsersAndLateFailure(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	for _, boundary := range []string{"read", "before-close"} {
		t.Run(boundary, func(t *testing.T) {
			root, binding, _, _ := nativeSpendInspection(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			inspection, err := OpenInspection(ctx, root, binding, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			entered, resume := make(chan struct{}), make(chan struct{})
			var unhold sync.Once
			defer unhold.Do(func() { close(resume) })
			t.Cleanup(func() { _ = inspection.Close() })
			s := inspection.original
			var held sync.Once
			s.observe = func(phase string, file *os.File) {
				if phase == boundary && file == s.journal.file {
					held.Do(func() { close(entered); <-resume })
				}
			}
			checked, closed := make(chan error, 1), make(chan error, 1)
			if boundary == "read" {
				go func() { checked <- inspection.Check() }()
			} else {
				go func() { closed <- inspection.Close() }()
			}
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("native held boundary not reached", ctx.Err())
			}
			if boundary == "read" {
				go func() { closed <- inspection.Close() }()
			}
			limit := time.NewTimer(time.Second)
			tick := time.NewTicker(time.Millisecond)
			defer limit.Stop()
			defer tick.Stop()
			for {
				s.mu.Lock()
				denied := s.closed
				s.mu.Unlock()
				if denied {
					break
				}
				select {
				case <-tick.C:
				case <-limit.C:
					t.Fatal("Close denial blocked behind native user")
				}
			}
			if err := inspection.Check(); err == nil {
				t.Fatal("closed inspection admitted another check")
			}
			foreign, err := OpenRetained(root, binding)
			if foreign != nil {
				_ = foreign.Close()
			}
			if err == nil {
				t.Fatal("held native user returned lease")
			}
			select {
			case err := <-closed:
				t.Fatal("Close returned before physical join", err)
			default:
			}
			// Actual close failure after the held read/Close cannot be masked by
			// local denial, cancellation or a later copied Close.
			if err := s.journal.file.Close(); err != nil {
				t.Fatal(err)
			}
			unhold.Do(func() { close(resume) })
			if boundary == "read" {
				if err := <-checked; err == nil {
					t.Fatal("late read accepted")
				}
			}
			first := <-closed
			if !errors.Is(first, os.ErrClosed) {
				t.Fatal("late native Close failure lost", first)
			}
			if next := inspection.Close(); next != first {
				t.Fatal("copied Close changed retained result", next, first)
			}
			ledger, err := OpenRetained(root, binding)
			if err != nil {
				t.Fatal("original lease not returned after real joins", err)
			}
			if err := ledger.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSpendInspectionNativeChangedHistoryCannotRevive(t *testing.T) {
	for _, fault := range []string{"bytes", "inode", "inventory", "root-name"} {
		t.Run(fault, func(t *testing.T) {
			root, binding, _, _ := nativeSpendInspection(t)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			inspection, err := OpenInspection(ctx, root, binding, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer inspection.Close()
			path := filepath.Join(root, closedSpendLedgerName)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var restore func() error
			switch fault {
			case "bytes":
				changed := bytes.Clone(original)
				changed[len(changed)-1] ^= 1
				err = os.WriteFile(path, changed, 0600)
				restore = func() error { return os.WriteFile(path, original, 0600) }
			case "inode":
				err = os.Rename(path, path+"-original")
				if err == nil {
					err = os.WriteFile(path, original, 0600)
				}
				restore = func() error {
					if err := os.Remove(path); err != nil {
						return err
					}
					return os.Rename(path+"-original", path)
				}
			case "inventory":
				err = os.WriteFile(filepath.Join(root, "unexpected"), nil, 0600)
				restore = func() error { return os.Remove(filepath.Join(root, "unexpected")) }
			case "root-name":
				err = os.Rename(root, root+"-original")
				if err == nil {
					err = os.Mkdir(root, 0700)
				}
				restore = func() error {
					if err := os.Remove(root); err != nil {
						return err
					}
					return os.Rename(root+"-original", root)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			first := inspection.Check()
			if first == nil {
				t.Fatal("changed original history accepted")
			}
			if err := restore(); err != nil {
				t.Fatal(err)
			}
			if err := inspection.Check(); !errors.Is(err, first) {
				t.Fatal("restoration revived inspection or lost first failure", err)
			}
			if err := inspection.Close(); !errors.Is(err, first) {
				t.Fatal("Close lost original failure", err)
			}
			ledger, err := OpenRetained(root, binding)
			if err != nil {
				t.Fatal("joined inspection retained lease", err)
			}
			if err := ledger.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSpendInspectionNativeCancellationRetainsReader(t *testing.T) {
	root, binding, _, _ := nativeSpendInspection(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	inspection, err := OpenInspection(ctx, root, binding, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	entered, resume := make(chan struct{}), make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(resume) })
	t.Cleanup(func() { _ = inspection.Close() })
	inspection.original.observe = func(phase string, _ *os.File) {
		if phase == "read" {
			close(entered)
			<-resume
		}
	}
	checked, closed := make(chan error, 1), make(chan error, 1)
	go func() { checked <- inspection.Check() }()
	select {
	case <-entered:
	case err := <-checked:
		t.Fatal("reader returned before boundary", err)
	case <-ctx.Done():
		t.Fatal("reader failed to reach boundary", ctx.Err())
	}
	cancel()
	if err := inspection.Check(); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation did not deny fresh check", err)
	}
	go func() { closed <- inspection.Close() }()
	foreign, err := OpenRetained(root, binding)
	if foreign != nil {
		_ = foreign.Close()
	}
	if err == nil {
		t.Fatal("canceled reader returned original lease before join")
	}
	select {
	case err := <-closed:
		t.Fatal("Close returned while reader held", err)
	default:
	}
	release.Do(func() { close(resume) })
	if err := <-checked; !errors.Is(err, context.Canceled) {
		t.Fatal("late reader lost cancellation", err)
	}
	if err := <-closed; !errors.Is(err, context.Canceled) {
		t.Fatal("Close lost cancellation", err)
	}
	if next, err := OpenInspection(ctx, root, binding, 0, 0); !errors.Is(err, context.Canceled) || next.original != nil {
		t.Fatal("canceled caller reopened inspection", err)
	}
	ledger, err := OpenRetained(root, binding)
	if err != nil {
		t.Fatal("joined canceled inspection retained lease", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSpendInspectionNativeCloseJoinsCleanReader(t *testing.T) {
	root, binding, _, _ := nativeSpendInspection(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	inspection, err := OpenInspection(ctx, root, binding, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	entered, resume := make(chan struct{}), make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(resume) })
	t.Cleanup(func() { _ = inspection.Close() })
	inspection.original.observe = func(phase string, _ *os.File) {
		if phase == "read" {
			close(entered)
			<-resume
		}
	}
	checked, closed := make(chan error, 1), make(chan error, 1)
	go func() { checked <- inspection.Check() }()
	select {
	case <-entered:
	case err := <-checked:
		t.Fatal("reader returned before boundary", err)
	case <-ctx.Done():
		t.Fatal("reader failed to reach boundary", ctx.Err())
	}
	go func() { closed <- inspection.Close() }()
	denial := time.NewTimer(time.Second)
	defer denial.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		inspection.original.mu.Lock()
		denied := inspection.original.closed
		inspection.original.mu.Unlock()
		if denied {
			break
		}
		select {
		case <-tick.C:
		case <-denial.C:
			t.Fatal("Close denial waited for original reader")
		}
	}
	release.Do(func() { close(resume) })
	if err := <-checked; !errors.Is(err, errInspectionClosed) {
		t.Fatal("reader published success after local denial", err)
	}
	if err := <-closed; err != nil {
		t.Fatal("local denial became a false physical failure", err)
	}
	if err := inspection.Close(); err != nil {
		t.Fatal("copied Close lost clean terminal result", err)
	}
}
