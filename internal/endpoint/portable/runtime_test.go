package portable

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenCreatesSeparatedRootsAndProvesAttachment(t *testing.T) {
	config := testConfig(t)
	runtime, err := Open(config)
	if err != nil {
		t.Fatalf("open portable runtime: %v", err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	for _, path := range []string{
		filepath.Join(config.ConfigHome, "grants"),
		filepath.Join(config.StateHome, "vault"),
		filepath.Join(config.StateHome, "floors"),
		filepath.Join(config.StateHome, "diagnostics"),
		filepath.Join(config.StateHome, "live"),
		config.CacheHome,
		config.RuntimeHome,
	} {
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("owned root %s is invalid: info=%v err=%v", path, info, statErr)
		}
	}
	if err := probeAttachment(runtime.Attachment()); err != nil {
		t.Fatalf("probe attachment: %v", err)
	}
	if _, err := os.Lstat(runtime.Attachment()); err != nil {
		t.Fatalf("attachment disappeared while runtime is live: %v", err)
	}
}

func TestOpenRefusesConcurrentOwnerWithoutTouchingLiveAttachment(t *testing.T) {
	config := testConfig(t)
	first, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })

	_, err = Open(config)
	var lifecycle *Error
	if !errors.As(err, &lifecycle) || lifecycle.Reason != ReasonOwnerBusy {
		t.Fatalf("concurrent Open error = %v, want owner-busy", err)
	}
	if err := probeAttachment(first.Attachment()); err != nil {
		t.Fatalf("live attachment was changed: %v", err)
	}
}

func TestOpenRecoversOnlyExpectedStaleSocket(t *testing.T) {
	config := testConfig(t)
	first, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	attachment := first.Attachment()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", attachment)
	if err != nil {
		t.Fatalf("create stale socket: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(config)
	if err != nil {
		t.Fatalf("recover expected stale socket: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if err := probeAttachment(second.Attachment()); err != nil {
		t.Fatalf("recovered attachment does not answer: %v", err)
	}
}

func TestOpenPreservesUnexpectedAttachmentEntry(t *testing.T) {
	config := testConfig(t)
	first, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	attachment := first.Attachment()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attachment, []byte("do not remove"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = Open(config)
	var lifecycle *Error
	if !errors.As(err, &lifecycle) || lifecycle.Reason != ReasonUnexpectedRuntimeEntry {
		t.Fatalf("unexpected attachment error = %v", err)
	}
	got, readErr := os.ReadFile(attachment)
	if readErr != nil || string(got) != "do not remove" {
		t.Fatalf("unexpected entry was changed: bytes=%q err=%v", got, readErr)
	}
}

func TestOpenRefusesAttachmentPathBeyondDeclaredBudgetBeforeBind(t *testing.T) {
	config := testConfig(t)
	config.RuntimeHome = filepath.Join(config.RuntimeHome, strings.Repeat("long-", 24))
	_, err := Open(config)
	var lifecycle *Error
	if !errors.As(err, &lifecycle) || lifecycle.Reason != ReasonLocalProfileInvalid {
		t.Fatalf("long attachment path error = %v", err)
	}
}

func TestRunReportsStartingReadyAndStopped(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	config := testConfig(t)
	states := make(chan Event, 3)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, config, func(event Event) { states <- event })
	}()

	first := <-states
	second := <-states
	if first.State != StateStarting || second.State != StateReady {
		t.Fatalf("startup events = %#v, %#v", first, second)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v after requested stop", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
	stopped := <-states
	if stopped.State != StateStopped {
		t.Fatalf("terminal event = %#v, want stopped", stopped)
	}
}

func testConfig(t *testing.T) Config {
	t.Helper()
	root, err := os.MkdirTemp(os.TempDir(), "an-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return Config{
		ConfigHome:  filepath.Join(root, "config"),
		StateHome:   filepath.Join(root, "state"),
		CacheHome:   filepath.Join(root, "cache"),
		RuntimeHome: filepath.Join(root, "runtime"),
	}
}
