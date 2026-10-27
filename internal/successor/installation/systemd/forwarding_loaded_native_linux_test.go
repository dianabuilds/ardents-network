//go:build network_signing_native

package systemd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The installed-profile root driver retains this independent physical reference
// through Stop and its fresh native exit observation. It starts nothing and
// supplies no accepting fact, domain join, history lease or runtime authority.
func TestForwardingLoadedNativeUnitReference(t *testing.T) {
	marker, err := os.ReadFile("/etc/forwarding-initial-component-marker")
	if os.Getenv("ARDENTS_FORWARDING_UNIT_REFERENCE") != "original" || os.Geteuid() != 0 || err != nil || string(marker) != "ardents-forwarding-coherent-preparation-v1\n" {
		t.Fatal("invalid original private reference environment")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
	control, err := OpenForwardingControl(ctx)
	if control != nil {
		defer control.Close()
	}
	if err != nil {
		t.Fatal("original fixed-unit reference unavailable")
	}
	if _, err := control.Observe(); err != nil {
		t.Fatal("original fixed-unit observation unavailable")
	}
	if _, err := fmt.Fprintln(os.Stdout, "forwarding-original-unit-referenced"); err != nil {
		t.Fatal("reference readiness output failed")
	}
	joined := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = os.Stdin.Close(); close(joined) })
	defer func() {
		if !stop() {
			<-joined
		}
	}()
	var release [1]byte
	if _, err := io.ReadFull(os.Stdin, release[:]); err != nil || release[0] != 'r' || ctx.Err() != nil {
		t.Fatal("original reference release unavailable")
	}
	if err := control.Close(); err != nil {
		t.Fatal("original reference physical close failed")
	}
}

// This is a stopped-manager component, not an installed invocation. The parent
// retains the actual fixed unit with its own root connection while a separate
// ordinary test process independently observes it under the real Node principal.
func TestForwardingLoadedNativeOriginalRootAndNonrootConnections(t *testing.T) {
	path := os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING")
	if path == "" || !bytes.HasSuffix([]byte(path), []byte("/binding.json")) {
		t.Fatal("invalid environment: genuine private completed installation required")
	}
	if os.Getenv("ARDENTS_FORWARDING_LOADED_NATIVE_CHILD") == "1" {
		forwardingLoadedNativeChild(t)
		return
	}
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: original root component controller required")
	}
	var b struct{ UID, GID uint32 }
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &b) != nil || b.UID < 100 || b.UID > 999 || b.GID != b.UID {
		t.Fatal("actual native principal unavailable", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	control, err := OpenForwardingControl(ctx)
	if control != nil {
		defer control.Close()
	}
	if err != nil {
		t.Fatal("actual original fixed reference", err)
	}
	r := control.original.reference
	if r.client.transport.sourceJobs.Load() || !r.client.transport.forwardingJobs.Load() {
		t.Fatal("root Forwarding acquired another purpose's vocabulary")
	}
	observation, err := control.Observe()
	if err != nil || !Matches(observation.Unit, "Id", "s", "ardents-forwarding.service") || !Matches(observation.Unit, "ActiveState", "s", "inactive") || !Matches(observation.Service, "MainPID", "u", uint32(0)) {
		t.Fatal("actual original stopped observations differ", err)
	}
	identity, err := control.ManagerIdentity()
	if err != nil || identity == "" {
		t.Fatal("actual original root manager identity", err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, program, "-test.run", "^TestForwardingLoadedNativeOriginalRootAndNonrootConnections$", "-test.v", "-test.count=1", "-test.timeout=20s")
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "GOMAXPROCS=2", "ARDENTS_FORWARDING_INITIAL_BINDING=" + path,
		"ARDENTS_FORWARDING_LOADED_NATIVE_CHILD=1", "ARDENTS_FORWARDING_LOADED_NATIVE_UID=" + strconv.FormatUint(uint64(b.UID), 10),
		"ARDENTS_FORWARDING_LOADED_NATIVE_MANAGER=" + identity}
	command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: b.UID, Gid: b.GID, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	runtime.LockOSThread()
	output, childErr := command.CombinedOutput()
	runtime.UnlockOSThread()
	if childErr != nil {
		t.Fatalf("original non-root component did not join: %v\n%s", childErr, output)
	}
	t.Log(string(output))
	if _, err := control.Observe(); err != nil {
		t.Fatal("child observations changed root custody", err)
	}
	copy := *control
	if err := copy.Close(); err != nil {
		t.Fatal("actual root connection did not join", err)
	}
	if _, err := control.Observe(); err == nil {
		t.Fatal("root connection revived after copied Close")
	}
	if err := control.Close(); err != nil {
		t.Fatal("root retained Close changed", err)
	}
}

// ManagerIdentity exposes the original physical observation to this native
// test; runtime composition consumes the complete Observe result instead.
func (s *ForwardingRuntime) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func forwardingLoadedNativeChild(t *testing.T) {
	t.Helper()
	uid, err := strconv.ParseUint(os.Getenv("ARDENTS_FORWARDING_LOADED_NATIVE_UID"), 10, 32)
	if err != nil || uid < 100 || uid > 999 || uint64(os.Geteuid()) != uid || uint64(os.Getegid()) != uid {
		t.Fatal("invalid environment: actual private Node principal required", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	observer, err := OpenForwardingRuntime(ctx)
	if observer != nil {
		defer observer.Close()
	}
	if err != nil {
		t.Fatal("independent actual non-root manager observation", err)
	}
	r := observer.original.reference
	if r.client.transport.sourceJobs.Load() || r.client.transport.forwardingJobs.Load() {
		t.Fatal("read-only connection acquired job vocabulary")
	}
	identity, err := observer.ManagerIdentity()
	if err != nil || identity != os.Getenv("ARDENTS_FORWARDING_LOADED_NATIVE_MANAGER") {
		t.Fatal("independently observed manager differs", err)
	}
	if _, err := observer.Observe(); err != nil {
		t.Fatal("original read-only observation", err)
	}
	held, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	r.observe = func() { close(held); <-release }
	done := make(chan error, 1)
	closed := make(chan error, 1)
	observedConsumed, closeStarted, closeConsumed := false, false, false
	go func() { _, err := observer.Observe(); done <- err }()
	defer func() {
		unblock()
		_ = observer.Close()
		if !observedConsumed {
			<-done
		}
		if closeStarted && !closeConsumed {
			<-closed
		}
	}()
	select {
	case <-held:
	case err := <-done:
		observedConsumed = true
		t.Fatal("actual observation did not reach held original I/O", err)
	case <-time.After(5 * time.Second):
		t.Fatal("actual original read unavailable")
	}
	cancel()
	closeStarted = true
	go func() { closed <- observer.Close() }()
	if _, err := observer.Observe(); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation did not synchronously deny", err)
	}
	select {
	case err := <-closed:
		closeConsumed = true
		t.Fatal("physical Close escaped held original observation", err)
	default:
	}
	unblock()
	observedErr := <-done
	observedConsumed = true
	if !errors.Is(observedErr, context.Canceled) {
		t.Fatal("late observation lost original cancellation", observedErr)
	}
	closeErr := <-closed
	closeConsumed = true
	copy := *observer
	if !errors.Is(closeErr, context.Canceled) || copy.Close() != closeErr {
		t.Fatal("copied original Close result changed", closeErr)
	}
}
