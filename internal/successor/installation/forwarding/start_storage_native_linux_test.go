//go:build network_signing_native

package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func nativeStartStorage(t *testing.T, ctx context.Context) *startStorage {
	t.Helper()
	f := prepareInputFixture(t)
	root := filepath.Join(filepath.Dir(f.path), "installation")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := createStartStorage(ctx, root, 0, digest([]byte("independent physical fixture, no installed authority")))
	t.Cleanup(func() { _ = s.close() })
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestForwardingStartStorageNativeBirthsAndExclusiveHistory(t *testing.T) {
	s := nativeStartStorage(t, t.Context())
	if err := s.birthDirectory(988, s.check); err != nil {
		t.Fatal(err)
	}
	body := []byte("physical guard fixture only\n")
	file, err := s.write("start-guard.json", body, s.check)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(file.path)
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatal("original file bytes differ", err)
	}
	info, err := os.Lstat(s.path)
	if err != nil || !nodeAccess(info, 0750, 0, 988, true) {
		t.Fatal("completion directory access differs", err)
	}
	info, err = os.Lstat(file.path)
	if err != nil || !nodeAccess(info, 0600, 0, 0, false) {
		t.Fatal("private record access differs", err)
	}
	raw, err := os.ReadFile(s.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	want := []string{"first-start", "directory-intent", "directory-born", "directory-ready", "file-intent", "file-born", "file-ready"}
	if len(lines) != len(want) {
		t.Fatal("birth transcript length differs", len(lines))
	}
	for i, line := range lines {
		var entry startEntry
		if err := json.Unmarshal(line, &entry); err != nil || entry.Sequence != uint32(i) || entry.Phase != want[i] || entry.Schema != "ardents-forwarding-start-journal-v1" {
			t.Fatal("independent birth transcript differs", i, err)
		}
		if i == 0 && entry.SHA256 != digest([]byte("independent physical fixture, no installed authority")) {
			t.Fatal("binding commitment differs")
		}
	}
	probe, err := os.Open(s.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("original writer did not exclude independent reader", err)
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		t.Fatal("joined writer retained lease", err)
	}
	for range 3 {
		retry, err := createStartStorage(t.Context(), filepath.Dir(s.path), 0, digest(body))
		if err == nil || retry.journal != nil {
			t.Fatal("retained attempt was adopted or reopened")
		}
		if closeErr := retry.close(); !errors.Is(closeErr, os.ErrExist) {
			t.Fatal("constructor failure lost through Close", closeErr)
		}
	}
	after, err := os.ReadFile(s.journal.path)
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatal("fresh retry changed original history", err)
	}
}

func TestForwardingStartStorageNativeUncertainSyncCannotAppendOrRetry(t *testing.T) {
	s := nativeStartStorage(t, t.Context())
	late := errors.New("controlled failure after actual journal sync")
	s.observe = func(phase string, _ *os.File) error {
		if phase == "synced" {
			return late
		}
		return nil
	}
	if err := s.record("start-attempt", s.path, nil, ""); !errors.Is(err, late) {
		t.Fatal("uncertain sync failure lost", err)
	}
	before, err := os.ReadFile(s.journal.path)
	if err != nil || !bytes.Contains(before, []byte("start-attempt")) {
		t.Fatal("actual synced failure prefix disappeared", err)
	}
	s.observe = nil
	if err := s.record("ack-written", s.path, nil, ""); !errors.Is(err, late) {
		t.Fatal("late successful write erased uncertainty", err)
	}
	after, err := os.ReadFile(s.journal.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("denied append changed prefix", err)
	}
	if err := s.close(); !errors.Is(err, late) {
		t.Fatal("joined close erased original uncertainty", err)
	}
	if err := s.close(); !errors.Is(err, late) {
		t.Fatal("repeated close erased original uncertainty", err)
	}
}

func TestForwardingStartStorageNativeOriginalDirectoryLossLatches(t *testing.T) {
	s := nativeStartStorage(t, t.Context())
	if err := s.birthDirectory(988, s.check); err != nil {
		t.Fatal(err)
	}
	old := s.path + "-original"
	if err := os.Rename(s.path, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(s.path, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(s.path, 0, 988); err != nil {
		t.Fatal(err)
	}
	if err := s.check(); err == nil {
		t.Fatal("substitute directory admitted")
	}
	if err := os.Remove(s.path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(old, s.path); err != nil {
		t.Fatal(err)
	}
	if err := s.check(); err == nil {
		t.Fatal("restoration revived failed original owner")
	}
	if _, err := s.write("start-guard.json", []byte("late"), func() error { return nil }); err == nil {
		t.Fatal("failed original owner created a late record")
	}
	if _, err := os.Lstat(filepath.Join(s.path, "start-guard.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("denied write had an effect", err)
	}
}

func TestForwardingStartStorageNativeLeaseOutlivesPhysicalClose(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s := nativeStartStorage(t, ctx)
	if err := s.birthDirectory(988, s.check); err != nil {
		t.Fatal(err)
	}
	if _, err := s.write("start-guard.json", []byte("held original close\n"), s.check); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	late := errors.New("controlled original file Close failure")
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
		t.Fatal("journal lease returned before original physical Close joined", err)
	}
	close(release)
	if err := <-done; !errors.Is(err, late) || !errors.Is(err, context.Canceled) {
		t.Fatal("physical/caller result lost", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		t.Fatal("joined physical close did not return journal lease", err)
	}
	if err := s.close(); !errors.Is(err, late) || !errors.Is(err, context.Canceled) {
		t.Fatal("retained close changed", err)
	}
}
