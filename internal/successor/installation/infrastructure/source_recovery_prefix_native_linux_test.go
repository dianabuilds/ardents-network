//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"testing"
	"time"
)

// These controls use actual ext4 files/flocks only. They construct no installed
// Source, manager, Release authorization, State or accepting recovery result.
func TestSourceRecoveryPrefixNativeExclusionAndRefusalCleanup(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	births, _ := nativeSourceBirths(t, t.Context())
	defer births.close()
	journal := births.journal.path
	probe, err := os.Open(journal)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	watched := map[string]bool{journal: true}
	for parent := filepath.Dir(journal); ; parent = filepath.Dir(parent) {
		watched[parent] = true
		if parent == "/" {
			break
		}
	}
	before := sourceHistoryDescriptors(t, watched)
	for i := 0; i < 8; i++ {
		other, err := openSourceRecoveryPrefix(t.Context(), journal, "initial")
		if other != nil || !errors.Is(err, syscall.EWOULDBLOCK) {
			_ = other.close()
			t.Fatal("live original writer did not exclude recovery-prefix acquisition", err)
		}
		if actual := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(actual, before) {
			t.Fatal("refused prefix acquisition retained native descriptors before GC", before, actual)
		}
	}
	if err := births.close(); err != nil {
		t.Fatal(err)
	}
	before = sourceHistoryDescriptors(t, watched)
	owner, err := openSourceRecoveryPrefix(t.Context(), journal, "initial")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	if len(owner.records) != 1 || owner.records[0].Phase != "initial" {
		t.Fatal("original canonical prefix was not independently read")
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("successful prefix owner did not retain its actual journal exclusion", err)
	}
	owned := sourceHistoryDescriptors(t, watched)
	for i := 0; i < 8; i++ {
		other, err := openSourceRecoveryPrefix(t.Context(), journal, "replacement")
		if other != nil || !errors.Is(err, syscall.EWOULDBLOCK) {
			_ = other.close()
			t.Fatal("another prefix owner bypassed the original exclusion", err)
		}
		if actual := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(actual, owned) {
			t.Fatal("foreign refused owner leaked descriptors", owned, actual)
		}
	}
	if err := owner.close(); err != nil {
		t.Fatal(err)
	}
	if actual := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(actual, before) {
		t.Fatal("successful owner Close leaked descriptors before GC", before, actual)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("original lease survived joined Close", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	// This refusal reaches parsing after taking the actual flock, unlike the
	// live-writer refusals above. Every attempt must release that acquisition.
	if err := os.WriteFile(journal, append(bytes.Clone(owner.journal.raw), '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		other, err := openSourceRecoveryPrefix(t.Context(), journal, "initial")
		if other != nil || !errors.Is(err, ErrBinding) {
			_ = other.close()
			t.Fatal("torn original prefix accepted", err)
		}
		if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Fatal("parse refusal retained journal exclusion", err)
		}
		if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_UN); err != nil {
			t.Fatal(err)
		}
		if actual := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(actual, before) {
			t.Fatal("parse refusal leaked native descriptors", before, actual)
		}
	}
}

func TestSourceRecoveryPrefixNativeMutationAndCopiedLineage(t *testing.T) {
	for _, fault := range []string{"same-inode", "copied-lineage", "ancestor-access"} {
		t.Run(fault, func(t *testing.T) {
			births, _ := nativeSourceBirths(t, t.Context())
			journal := births.journal.path
			if err := births.close(); err != nil {
				t.Fatal(err)
			}
			owner, err := openSourceRecoveryPrefix(t.Context(), journal, "initial")
			if err != nil {
				t.Fatal(err)
			}
			defer owner.close()
			switch fault {
			case "same-inode":
				file, err := os.OpenFile(journal, os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, written := file.WriteAt([]byte{'['}, 0)
				if err := errors.Join(written, file.Close()); err != nil {
					t.Fatal(err)
				}
			case "copied-lineage":
				if err := os.Rename(journal, journal+".retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(journal, owner.journal.raw, 0600); err != nil {
					t.Fatal(err)
				}
				other, err := openSourceRecoveryPrefix(t.Context(), journal, "initial")
				if other != nil || !errors.Is(err, ErrBinding) {
					_ = other.close()
					t.Fatal("copied bytes recreated original journal provenance", err)
				}
			case "ancestor-access":
				if err := os.Chmod(filepath.Dir(journal), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := owner.check(); !errors.Is(err, ErrBinding) {
				t.Fatal("changed original custody accepted", err)
			}
			if fault == "ancestor-access" {
				if err := os.Chmod(filepath.Dir(journal), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := owner.check(); !errors.Is(err, ErrBinding) {
				t.Fatal("restored appearance erased original denial", err)
			}
		})
	}
}

func TestSourceRecoveryPrefixNativeCanceledReadAndCloseJoin(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	births, _ := nativeSourceBirths(t, t.Context())
	journal := births.journal.path
	if err := births.close(); err != nil {
		t.Fatal(err)
	}
	probe, err := os.Open(journal)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	owner, err := openSourceRecoveryPrefix(ctx, journal, "initial")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	owner.observed = func(*os.File) { close(entered); <-release }
	checked := make(chan error, 1)
	go func() { checked <- owner.check() }()
	<-entered
	cancel()
	if err := owner.check(); !errors.Is(err, context.Canceled) {
		t.Fatal("caller cancellation did not deny a new observation", err)
	}
	// Force a real independent cleanup error without releasing the journal
	// lock. Close must join the still-held physical read before releasing it.
	if err := owner.parents[filepath.Dir(journal)].Close(); err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- owner.close() }()
	waitSourceRecoverySealed(t, owner)
	select {
	case err := <-closed:
		t.Fatal("Close returned while an original physical read was held", err)
	default:
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("Close released journal custody before read join", err)
	}
	unblock()
	if err := <-checked; !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) || !errors.Is(err, ErrBinding) {
		t.Fatal("original canceled read lost its physical failure", err)
	}
	if err := <-closed; !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) || !errors.Is(err, ErrBinding) {
		t.Fatal("joined Close lost cancellation/later observation/actual cleanup failure", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("joined Close retained original journal flock before GC", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if actual := sourceHistoryDescriptors(t, map[string]bool{journal: true, filepath.Dir(journal): true}); !reflect.DeepEqual(actual, map[string]int{journal: 1}) {
		t.Fatal("joined Close retained descriptors before GC", actual)
	}
}

func waitSourceRecoverySealed(t *testing.T, owner *sourceRecoveryPrefix) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		owner.mu.Lock()
		sealed := owner.closed
		owner.mu.Unlock()
		if sealed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("Close did not seal local admission")
		}
		runtime.Gosched()
	}
}
