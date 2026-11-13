//go:build network_signing_native

package join

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"testing"
)

// The canonical Control/Node producer must have completed this exact private
// stand first. No fixture row or parser result supplies native birth provenance.
func TestJoinCompletedNativeOriginalLeaseCopiesAndDescriptors(t *testing.T) {
	path := os.Getenv("ARDENTS_JOIN_INITIAL_BINDING")
	if !validPath(path) {
		t.Fatal("invalid environment: genuine completed initial JOIN required")
	}
	open := func() {
		t.Helper()
		h, err := ReadCompleted(t.Context(), path)
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		c := h.original
		probe, err := os.OpenFile(c.journal.path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer probe.Close()
		if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err == nil {
			t.Fatal("independent writer exclusion absent")
		}
		other, err := ReadCompleted(t.Context(), path)
		if err == nil || other.original != nil {
			_ = other.Close()
			t.Fatal("second original journal owner admitted")
		}
		if err := h.Check(); err != nil {
			t.Fatal("sibling refusal poisoned original", err)
		}
		secret := c.files[filepath.Join(c.binding.Request.CredentialDirectory, "private-key.pem")].body
		copy := h
		if err := copy.Close(); err != nil {
			t.Fatal(err)
		}
		if err := h.Check(); !errors.Is(err, errCompletedClosed) {
			t.Fatal("copy renewed closed original", err)
		}
		if !bytes.Equal(secret, make([]byte, len(secret))) {
			t.Fatal("private input survived joined Close")
		}
		if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
			t.Fatal("joined reader retained journal lease", err)
		}
		if err := h.Close(); err != nil {
			t.Fatal("post-close denial poisoned original result", err)
		}
	}
	open()
	old := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(old)
	before := nativeInputFDCount(t)
	for range 4 {
		open()
		if after := nativeInputFDCount(t); after != before {
			t.Fatal("completed reader leaked original descriptors without GC", before, after)
		}
	}
}

func TestJoinCompletedNativeCloseJoinsOriginalReadAndRetainsLateFailure(t *testing.T) {
	path := os.Getenv("ARDENTS_JOIN_INITIAL_BINDING")
	if !validPath(path) {
		t.Fatal("invalid environment: genuine completed initial JOIN required")
	}
	for _, kind := range []string{"clean", "late-close-failure"} {
		t.Run(kind, func(t *testing.T) {
			h, err := ReadCompleted(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			c := h.original
			entered, release := make(chan *os.File, 1), make(chan struct{})
			var once, safeRelease sync.Once
			unblock := func() { safeRelease.Do(func() { close(release) }) }
			defer unblock()
			c.observe = func(file *os.File) { once.Do(func() { entered <- file; <-release }) }
			observed, closed := make(chan error, 1), make(chan error, 1)
			go func() { observed <- h.Check() }()
			fd := <-entered
			private := c.files[filepath.Join(c.binding.Request.CredentialDirectory, "private-key.pem")].body
			copy := h
			go func() { closed <- copy.Close() }()
			// Wait for actual admission denial rather than assuming Close was scheduled.
			for {
				c.mu.Lock()
				denied := c.closed
				c.mu.Unlock()
				if denied {
					break
				}
				runtime.Gosched()
			}
			if err := h.Check(); !errors.Is(err, errCompletedClosed) {
				t.Fatal("late observation queued behind held original", err)
			}
			select {
			case err := <-closed:
				t.Fatal("Close returned before original observer joined", err)
			default:
			}
			if bytes.Equal(private, make([]byte, len(private))) {
				t.Fatal("secret erased while original reader retained custody")
			}
			if kind == "late-close-failure" {
				if err := fd.Close(); err != nil {
					t.Fatal("causal original descriptor close", err)
				}
			}
			unblock()
			if err := <-observed; err == nil {
				t.Fatal("closed original handed off successful observation")
			}
			result := <-closed
			if kind == "clean" && result != nil {
				t.Fatal("own closed-admission denial poisoned clean completion", result)
			}
			if kind == "late-close-failure" && !errors.Is(result, os.ErrClosed) {
				t.Fatal("actual late native failure lost", result)
			}
			if h.Close() != result {
				t.Fatal("copies lost retained joined result")
			}
			if !bytes.Equal(private, make([]byte, len(private))) {
				t.Fatal("joined original retained private bytes")
			}
		})
	}
}

func TestJoinCompletedNativeRefusesCopiedBirthAndUnknownPending(t *testing.T) {
	path := os.Getenv("ARDENTS_JOIN_INITIAL_BINDING")
	if !validPath(path) {
		t.Fatal("invalid environment: genuine completed initial JOIN required")
	}
	h, err := ReadCompleted(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	b := h.original.binding
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	program := filepath.Join(sealed, programName)
	original := filepath.Join(filepath.Dir(b.Request.InstallationRoot), "retained-original-program")
	before, err := os.Lstat(program)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	// Both original and clone remain retained until all independent readers
	// close. Matching bytes and access cannot supply the recorded born inode.
	if err := os.Chmod(sealed, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(program, original); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(program, raw, 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(program, 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sealed, 0555); err != nil {
		t.Fatal(err)
	}
	rejected, err := ReadCompleted(t.Context(), path)
	if err == nil || rejected.original != nil {
		_ = rejected.Close()
		t.Fatal("copied native birth accepted")
	}
	if err := os.Chmod(sealed, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(program); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(original, program); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sealed, 0555); err != nil {
		t.Fatal(err)
	}
	after, err := os.Lstat(program)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("original native program not restored", err)
	}
	h, err = ReadCompleted(t.Context(), path)
	if err != nil {
		t.Fatal("restored actual birth refused", err)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	pending := b.Request.InstallationRoot + ".join-initial-pending"
	if err := os.WriteFile(pending, []byte("unknown retained completion\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rejected, err = ReadCompleted(t.Context(), path)
	if err == nil || rejected.original != nil {
		_ = rejected.Close()
		t.Fatal("unproven completion published")
	}
	if _, err := os.Lstat(pending); err != nil {
		t.Fatal("refusal removed original blocker", err)
	}
	if err := os.Remove(pending); err != nil {
		t.Fatal(err)
	}
}

func TestJoinCompletedNativeRefusesUnknownInitialInventory(t *testing.T) {
	path := os.Getenv("ARDENTS_JOIN_INITIAL_BINDING")
	if !validPath(path) {
		t.Fatal("invalid environment: genuine completed initial JOIN required")
	}
	h, err := ReadCompleted(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	b := h.original.binding
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{b.Request.InstallationRoot, filepath.Join(b.Request.InstallationRoot, "generations"), b.Request.CredentialDirectory} {
		unknown := filepath.Join(directory, "unknown-original")
		if err := os.WriteFile(unknown, []byte("retained unknown resource\n"), 0600); err != nil {
			t.Fatal(err)
		}
		refused, err := ReadCompleted(t.Context(), path)
		if err == nil || refused.original != nil {
			_ = refused.Close()
			t.Fatal("unknown initial resource admitted", directory)
		}
		if _, err := os.Lstat(unknown); err != nil {
			t.Fatal("reader removed unknown original data", err)
		}
		if err := os.Remove(unknown); err != nil {
			t.Fatal(err)
		}
		h, err := ReadCompleted(t.Context(), path)
		if err != nil {
			t.Fatal("restored exact inventory refused", err)
		}
		if err := h.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
