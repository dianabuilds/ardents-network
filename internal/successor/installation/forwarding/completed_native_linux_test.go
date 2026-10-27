//go:build linux && network_signing_native

package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"testing"
)

func nativeCompletedBinding(t *testing.T) string {
	t.Helper()
	path := os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING")
	if os.Geteuid() != 0 || !validPath(path) {
		t.Fatal("invalid environment: genuine completed private initial stand required")
	}
	return path
}

func nativeCompleted(t *testing.T, ctx context.Context) Completed {
	t.Helper()
	h, err := ReadCompleted(ctx, nativeCompletedBinding(t))
	if err != nil {
		t.Fatal("independent genuine completed initial observation", err)
	}
	return h
}

func probeCompletedLock(t *testing.T, path string, available bool) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	closeErr := file.Close()
	if closeErr != nil || available && err != nil || !available && !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("original completed journal lease availability differs", available, err, closeErr)
	}
}

func TestForwardingCompletedNativeOriginalLeaseAndNoAdoption(t *testing.T) {
	h := nativeCompleted(t, t.Context())
	defer func() { _ = h.Close() }()
	b := h.original.binding
	journal := b.Request.InstallationRoot + ".forwarding-initial-journal"
	probeCompletedLock(t, journal, false)
	other, err := ReadCompleted(t.Context(), nativeCompletedBinding(t))
	if err == nil || other.original != nil {
		t.Fatal("second completed observer adopted the original lease", err)
	}
	if err := h.Check(); err != nil {
		t.Fatal(err)
	}
	copy := h
	if err := h.Close(); err != nil || copy.Close() != nil {
		t.Fatal("original completed close", err)
	}
	probeCompletedLock(t, journal, true)
	if err := copy.Check(); err == nil {
		t.Fatal("closed copied native observation revived")
	}
	if err := copy.Close(); err != nil {
		t.Fatal("late Check rewrote clean Close", err)
	}
	// Even canonical complete bytes cannot replace an absent pending publication.
	pending := b.Request.InstallationRoot + ".forwarding-initial-pending"
	file, err := os.OpenFile(pending, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("ardents-forwarding-initial-pending-v1\nblocked\n"); err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err = errors.Join(err, file.Sync(), file.Close()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		now, err := os.Lstat(pending)
		if err != nil || !os.SameFile(info, now) {
			t.Error("original causal pending changed; do not remove foreign state", err)
			return
		}
		if err := os.Remove(pending); err != nil {
			t.Error(err)
		}
	}()
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)
	count := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	before := count()
	for range 16 {
		failed, err := ReadCompleted(t.Context(), nativeCompletedBinding(t))
		if !errors.Is(err, ErrRepairRequired) || failed.original != nil {
			t.Fatal("complete transcript admitted before pending publication", err)
		}
	}
	if after := count(); after != before {
		t.Fatal("failed completed openings retained descriptors without GC", before, after)
	}
}

func TestForwardingCompletedNativeReadCloseAndLatePhysicalFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	h := nativeCompleted(t, ctx)
	s := h.original.files
	program := s.files[filepath.Join(s.sealed, programName)].file
	unit := s.files["/etc/systemd/system/ardents-forwarding.service"].file
	readReached, readRelease := make(chan struct{}), make(chan struct{})
	closeReached, closeRelease := make(chan struct{}), make(chan struct{})
	denied := make(chan struct{})
	var readOnce, closeOnce, denyOnce sync.Once
	var readReleaseOnce, closeReleaseOnce sync.Once
	releaseRead := func() { readReleaseOnce.Do(func() { close(readRelease) }) }
	releaseClose := func() { closeReleaseOnce.Do(func() { close(closeRelease) }) }
	defer func() { releaseRead(); releaseClose(); cancel(); _ = h.Close() }()
	s.observe = func(stage string, file *os.File) {
		if stage == "read" && file == program {
			readOnce.Do(func() { close(readReached); <-readRelease })
		}
		if stage == "denied" {
			denyOnce.Do(func() { close(denied) })
		}
		if stage == "before-close" && file == unit {
			closeOnce.Do(func() {
				close(closeReached)
				<-closeRelease
				if err := file.Close(); err != nil {
					t.Error("original physical descriptor failure control", err)
				}
			})
		}
	}
	checked, closed := make(chan error, 1), make(chan error, 1)
	go func() { checked <- h.Check() }()
	<-readReached
	cancel()
	copy := h
	go func() { closed <- copy.Close() }()
	<-denied
	if err := h.Check(); err == nil {
		t.Fatal("Close did not deny before joining actual read")
	}
	probeCompletedLock(t, s.journal.path, false)
	select {
	case err := <-closed:
		t.Fatal("Close returned before actual read joined", err)
	default:
	}
	releaseRead()
	if err := <-checked; !errors.Is(err, context.Canceled) {
		t.Fatal("original canceled read cause lost", err)
	}
	<-closeReached
	probeCompletedLock(t, s.journal.path, false)
	select {
	case err := <-closed:
		t.Fatal("Close released lease before held physical Close", err)
	default:
	}
	releaseClose()
	err := <-closed
	if !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) || h.Close() != err {
		t.Fatal("copied completed owner lost cancellation or actual close failure", err)
	}
	probeCompletedLock(t, s.journal.path, true)
}

func TestForwardingCompletedNativeLostOriginalBytesDoNotRevive(t *testing.T) {
	h := nativeCompleted(t, t.Context())
	defer func() { _ = h.Close() }()
	path := h.original.files.journal.path
	file, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	var first [1]byte
	if _, err := file.ReadAt(first[:], 0); err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{'['}, 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	failed := h.Check()
	_, restoreErr := file.WriteAt(first[:], 0)
	restoreErr = errors.Join(restoreErr, file.Sync(), file.Close())
	if restoreErr != nil {
		t.Fatal("restore only causal test bytes on original inode", restoreErr)
	}
	if failed == nil || h.Check() != failed {
		t.Fatal("restoring bytes revived failed original observation", failed)
	}
	closed := h.Close()
	if !errors.Is(closed, failed) || h.Close() != closed {
		t.Fatal("completed Close lost the original failure or retained result", closed)
	}
	probeCompletedLock(t, path, true)
}

func TestForwardingCompletedNativeRejectsEquivalentNewNativeResources(t *testing.T) {
	h := nativeCompleted(t, t.Context())
	b := h.original.binding
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	// Retain original resources on every failed test cleanup. A test temporary
	// directory with automatic recursive cleanup must never own these backups.
	backups := filepath.Join(filepath.Dir(b.Request.InstallationRoot), "completed-negative-backup")
	if err := os.Mkdir(backups, 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{b.Request.StateRoot, b.History, filepath.Join(b.Request.CredentialDirectory, "private-key.pem")} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			original, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			native := original.Sys().(*syscall.Stat_t)
			var body []byte
			if !original.IsDir() {
				body, err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				defer clear(body)
			}
			backup := filepath.Join(backups, filepath.Base(path))
			if _, err := os.Lstat(backup); !os.IsNotExist(err) {
				t.Fatal("preserve unknown previous backup", err)
			}
			if err := os.Rename(path, backup); err != nil {
				t.Fatal(err)
			}
			var substitute os.FileInfo
			defer func() {
				if substitute != nil {
					named, err := os.Lstat(path)
					if err != nil || !os.SameFile(substitute, named) {
						t.Error("causal replacement changed; retain original backup", err)
						return
					}
					// Remove only this exact empty directory or finite test file.
					if err := os.Remove(path); err != nil {
						t.Error("retain original backup after failed replacement removal", err)
						return
					}
				} else if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Error("unknown partial replacement; retain original backup", err)
					return
				}
				if err := os.Rename(backup, path); err != nil {
					t.Error("original native resource remains retained in backup", err)
				}
			}()
			if original.IsDir() {
				err = os.Mkdir(path, original.Mode().Perm())
			} else {
				err = os.WriteFile(path, body, original.Mode().Perm())
			}
			if err != nil {
				t.Fatal(err)
			}
			substitute, err = os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chown(path, int(native.Uid), int(native.Gid)); err != nil {
				t.Fatal(err)
			}
			if os.SameFile(original, substitute) {
				t.Fatal("negative control did not replace native resource")
			}
			failed, err := ReadCompleted(t.Context(), nativeCompletedBinding(t))
			if !errors.Is(err, ErrBinding) || failed.original != nil {
				_ = failed.Close()
				t.Fatal("equivalent contents/access adopted as original resource", err)
			}
			probeCompletedLock(t, b.Request.InstallationRoot+".forwarding-initial-journal", true)
		})
	}
	reopened := nativeCompleted(t, t.Context())
	if err := reopened.Close(); err != nil {
		t.Fatal("causal controls did not retain genuine completed installation", err)
	}
}
