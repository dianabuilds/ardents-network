//go:build network_signing_native

package forwarding

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Native physical custody only. No signal, domain report, native scope join or
// accepting retirement is supplied by these detached journal/file fixtures.
func nativeRetirementStorage(t *testing.T, ctx context.Context) *retirementStorage {
	t.Helper()
	_ = prepareInputFixture(t)
	parent, err := os.MkdirTemp(os.TempDir(), "frs-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	root := filepath.Join(parent, "installation")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := createRetirementStorage(ctx, root, 0, digest([]byte("physical archive fixture only")))
	t.Cleanup(func() { _ = s.close() })
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestForwardingRetirementStorageNativeOriginalBirthsAndNoFreshRetry(t *testing.T) {
	s := nativeRetirementStorage(t, t.Context())
	if err := s.birthDirectory(988, s.check); err != nil {
		t.Fatal(err)
	}
	body := []byte("physical pending fixture only\n")
	if _, err := s.write("retirement-pending", body, s.check); err != nil {
		t.Fatal(err)
	}
	if err := s.prepareSocket(s.check); err != nil {
		t.Fatal(err)
	}
	if s.publish() == nil {
		t.Fatal("unjoined physical writer published retirement")
	}
	if err := s.check(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(s.path)
	if err != nil || !nodeAccess(info, 0750, 0, 988, true) {
		t.Fatal("retirement directory access differs", err)
	}
	info, err = os.Lstat(filepath.Join(s.path, "stop-completion.socket"))
	if err != nil || !sameStartSocket(info, info, 0660, 988) {
		t.Fatal("retirement socket access differs", err)
	}
	before, err := os.ReadFile(s.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := os.Open(s.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("original retirement journal not held", err)
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		t.Fatal("joined writer retained its lease", err)
	}
	for range 3 {
		retry, err := createRetirementStorage(t.Context(), filepath.Dir(s.path), 0, digest(body))
		if !errors.Is(err, os.ErrExist) {
			_ = retry.close()
			t.Fatal("unknown stop prefix adopted", err)
		}
		if err := retry.close(); !errors.Is(err, os.ErrExist) {
			t.Fatal("original creation refusal lost", err)
		}
	}
	after, err := os.ReadFile(s.journal.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("retry rewrote retirement prefix", err)
	}
	if _, err := os.Lstat(filepath.Join(s.path, "retirement-pending")); err != nil {
		t.Fatal("ordinary close removed the blocker", err)
	}
}

func TestForwardingRetirementStorageNativeUncertainSyncPermanentlyDeniesAppend(t *testing.T) {
	s := nativeRetirementStorage(t, t.Context())
	late := errors.New("controlled failure after actual retirement journal sync")
	s.observe = func(phase string, _ *os.File) error {
		if phase == "synced" {
			return late
		}
		return nil
	}
	if err := s.record("interrupt-attempt", s.path, nil, digest([]byte("physical nonce fixture"))); !errors.Is(err, late) {
		t.Fatal("actual sync failure lost", err)
	}
	before, err := os.ReadFile(s.journal.path)
	if err != nil || !bytes.Contains(before, []byte("interrupt-attempt")) {
		t.Fatal("synced uncertain prefix missing", err)
	}
	s.observe = nil
	if err := s.record("native-closed", s.path, nil, digest([]byte("late fixture"))); !errors.Is(err, late) {
		t.Fatal("late completion erased uncertainty", err)
	}
	after, err := os.ReadFile(s.journal.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("denied append changed original prefix", err)
	}
	if err := s.close(); !errors.Is(err, late) {
		t.Fatal("close erased uncertainty", err)
	}
}

func TestForwardingRetirementStorageNativeCloseRetainsLeaseAndLateFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s := nativeRetirementStorage(t, ctx)
	if err := s.birthDirectory(988, s.check); err != nil {
		t.Fatal(err)
	}
	if _, err := s.write("retirement-pending", []byte("held physical pending\n"), s.check); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	late := errors.New("controlled late original retirement Close failure")
	s.observe = func(phase string, file *os.File) error {
		if phase == "before-close" && file != s.journal.file {
			close(entered)
			<-release
			return late
		}
		return nil
	}
	probe, err := os.Open(s.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	done := make(chan error, 1)
	go func() { done <- s.close() }()
	<-entered
	cancel()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		close(release)
		<-done
		t.Fatal("lease returned before original close joined", err)
	}
	close(release)
	if err := <-done; !errors.Is(err, late) || !errors.Is(err, context.Canceled) {
		t.Fatal("original physical/caller result lost", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		t.Fatal("actual joined close retained lease", err)
	}
	if err := s.close(); !errors.Is(err, late) || !errors.Is(err, context.Canceled) {
		t.Fatal("repeated close changed retained outcome", err)
	}
}
