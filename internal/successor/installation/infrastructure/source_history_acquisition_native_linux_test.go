//go:build linux && network_signing_native

package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// The outside controller supplies a fresh, genuinely installed and explicitly
// retired Source for each case. These readers neither create nor authorize an
// installation. A refused history is retained untouched after the test.
func TestSourceHistoryAcquisitionNative(t *testing.T) {
	binding := os.Getenv("ARDENTS_SOURCE_HISTORY_BINDING")
	history := os.Getenv("ARDENTS_SOURCE_HISTORY_RELEASE")
	fault := os.Getenv("ARDENTS_SOURCE_HISTORY_CASE")
	if os.Geteuid() != 0 || !sourcePath(binding) || !sourcePath(history) {
		t.Fatal("invalid environment: original private retired Source required")
	}
	v, err := release.OpenRetained(history)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	}()
	// Disable automatic GC for the whole oracle: process exit or finalizers
	// cannot make leaked original descriptors appear to have been released.
	previousGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previousGC)
	chain, err := openSourceHistoryChain(t.Context(), v, binding, history)
	if err != nil {
		t.Fatal("complete genuine start history", err)
	}
	defer chain.close()
	s := chain.initial.selection
	root := s.binding.Request.InstallationRoot
	journal := sourceAttemptNumber(0).journal(root)
	watched := map[string]bool{}
	for path := range s.files {
		watched[path] = true
	}
	for path := range s.dirs {
		watched[path] = true
	}
	for path := range chain.initial.history.dirs {
		watched[path] = true
	}
	probe, err := os.Open(journal)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	blocked := func() {
		t.Helper()
		if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatal("independent descriptor did not observe original live lease", err)
		}
	}
	released := func() {
		t.Helper()
		if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Error("rejected acquisition retained original first-journal lock before GC", err)
			return
		}
		if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_UN); err != nil {
			t.Fatal(err)
		}
	}
	blocked()
	if fault == "success" {
		before := sourceHistoryDescriptors(t, watched)
		for i := 0; i < 8; i++ {
			// This separate caller really reaches the native lock while the
			// original successful acquisition still owns it.
			denied := make(chan error, 1)
			go func() {
				other, err := openSourceHistoryChain(context.Background(), v, binding, history)
				if other != nil {
					err = errors.Join(err, other.close(), ErrBinding)
				}
				denied <- err
			}()
			if err := <-denied; !errors.Is(err, syscall.EWOULDBLOCK) {
				t.Fatal("foreign acquisition lost native contention refusal", err)
			}
			blocked()
			if got := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(got, before) {
				t.Errorf("foreign refusal leaked descriptors at call %d: before %v, after %v", i, before, got)
			}
		}
		if err := chain.close(); err != nil {
			t.Fatal(err)
		}
		released()
		return
	}
	if err := chain.close(); err != nil {
		t.Fatal(err)
	}
	released()
	switch fault {
	case "before-lock":
		err = os.Chmod(journal, 0644)
	case "malformed-attempt":
		err = os.Mkdir(filepath.Join(root, "source-invocation-foreign"), 0750)
	case "gapped-attempt":
		err = os.Mkdir(sourceAttemptNumber(chain.latest.number+2).directory(root), 0750)
	case "transcript":
		var f *os.File
		f, err = os.OpenFile(chain.latest.number.journal(root), os.O_WRONLY|os.O_APPEND, 0)
		if err == nil {
			_, err = f.WriteString("{}\n")
			err = errors.Join(err, f.Close())
		}
	case "retirement":
		err = os.WriteFile(filepath.Join(chain.latest.number.directory(root), "source-retirement-pending"), []byte("incomplete original retirement\n"), 0600)
	default:
		t.Fatal("invalid environment: selected history acquisition case required")
	}
	if err != nil {
		t.Fatal(err)
	}
	before := sourceHistoryDescriptors(t, watched)
	for i := 0; i < 8; i++ {
		if fault == "retirement" {
			retired, err := openSourceRetirement(t.Context(), v, binding, history)
			if retired != nil || !errors.Is(err, ErrBinding) {
				_ = retired.close()
				t.Error("original retirement refusal lost", err)
			}
		} else {
			other, err := openSourceHistoryChain(t.Context(), v, binding, history)
			if other != nil || !errors.Is(err, ErrBinding) {
				_ = other.close()
				t.Error("original history refusal lost", err)
			}
		}
		released()
		if got := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(got, before) {
			t.Errorf("rejected acquisition leaked descriptors at call %d: before %v, after %v", i, before, got)
		}
	}
}

// Count the actual original file/ancestor descriptors in procfs, including
// duplicate opens of one path. Unrelated Go runtime descriptors are excluded.
func sourceHistoryDescriptors(t *testing.T, watched map[string]bool) map[string]int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal("invalid environment: native descriptor inventory required", err)
	}
	counts := map[string]int{}
	for _, entry := range entries {
		path, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue // ReadDir's own, already closed descriptor.
		}
		if err != nil {
			t.Fatal(fmt.Errorf("native descriptor readback: %w", err))
		}
		if watched[path] {
			counts[path]++
		}
	}
	return counts
}
