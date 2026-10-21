//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// Failure fixtures isolate command ordering, never successful State authority.
type failedSourceLifetime struct {
	waitErr, closeErr error
	closed            bool
}

type blockedSourceClose struct {
	failedSourceLifetime
	entered, release chan struct{}
}

func (owner *blockedSourceClose) Close() error {
	close(owner.entered)
	<-owner.release
	return owner.failedSourceLifetime.Close()
}

type sourceNotice struct{ observed chan struct{} }

func (notice sourceNotice) Write(raw []byte) (int, error) {
	if bytes.Contains(raw, []byte("completion-unavailable")) {
		close(notice.observed)
	}
	return len(raw), nil
}

func TestSourceUnavailableJoinReportsWithoutReturningOriginalCustody(t *testing.T) {
	operation, cleanup := errors.New("original refusal"), errors.New("late physical close")
	owner := &blockedSourceClose{failedSourceLifetime: failedSourceLifetime{closeErr: cleanup}, entered: make(chan struct{}), release: make(chan struct{})}
	notice := sourceNotice{make(chan struct{})}
	admissionCtx, deny := context.WithCancel(t.Context())
	defer deny()
	joined := make(chan error, 1)
	go func() {
		joined <- superviseSource(t.Context(), owner, func() error { return operation }, deny, io.Discard, notice, nil)
	}()
	<-owner.entered
	if admissionCtx.Err() != context.Canceled {
		close(owner.release)
		<-joined
		t.Fatal("physical join began before synchronous admission denial")
	}
	<-notice.observed
	select {
	case <-joined:
		t.Fatal("notice or timeout substituted for original join")
	default:
	}
	close(owner.release)
	if err := <-joined; !errors.Is(err, operation) || !errors.Is(err, cleanup) {
		t.Fatal("late join erased original failures", err)
	}
}

func (f *failedSourceLifetime) Current() (state.Snapshot, error) { return state.Snapshot{}, nil }
func (f *failedSourceLifetime) Wait(ctx context.Context) error {
	if f.waitErr != nil {
		return f.waitErr
	}
	<-ctx.Done()
	return nil
}
func (f *failedSourceLifetime) Close() error { f.closed = true; return f.closeErr }

type failedSourceOutput struct{ err error }

func (f failedSourceOutput) Write([]byte) (int, error) { return 0, f.err }

func TestSourceSupervisionRetainsOperationAndCleanupFailures(t *testing.T) {
	operation, cleanup := errors.New("operation"), errors.New("independent cleanup")
	for _, mode := range []string{"output", "wait", "protection"} {
		t.Run(mode, func(t *testing.T) {
			owner := &failedSourceLifetime{closeErr: cleanup}
			var out io.Writer = io.Discard
			check := func() error { return nil }
			switch mode {
			case "output":
				out = failedSourceOutput{operation}
			case "wait":
				owner.waitErr = operation
			case "protection":
				check = func() error { return operation }
			}
			err := superviseSource(t.Context(), owner, check, func() {}, out, io.Discard, nil)
			if !owner.closed || !errors.Is(err, operation) || !errors.Is(err, cleanup) {
				t.Fatal("original users or independent cause lost", owner.closed, err)
			}
		})
	}
}

func TestSourceCallerCancellationDuringBlockedReadyOutputRetainsCaller(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	fd := int(writer.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	for {
		_, err := syscall.Write(fd, make([]byte, 4096))
		if errors.Is(err, syscall.EAGAIN) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, deny := context.WithCancel(ctx)
	defer deny()
	checked := make(chan struct{})
	owner := &failedSourceLifetime{}
	joined := make(chan error, 1)
	go func() {
		joined <- superviseSource(ctx, owner, func() error {
			select {
			case <-checked:
			default:
				close(checked)
			}
			return nil
		}, deny, writer, io.Discard, nil)
	}()
	<-checked
	cancel()
	if err := <-joined; err != nil || !owner.closed {
		t.Fatal("original caller cancellation became an output/background failure", err)
	}
}

type sourceFailureWriter func([]byte) (int, error)

func (writer sourceFailureWriter) Write(raw []byte) (int, error) { return writer(raw) }

func TestSourceOutputFailureRetainsOriginalLatchedProcessFailure(t *testing.T) {
	output, process := errors.New("original output failure"), errors.New("original process failure already retained")
	latched := false
	check := func() error {
		if latched {
			return process
		}
		return nil
	}
	owner := &failedSourceLifetime{}
	err := superviseSource(t.Context(), owner, check, func() {}, sourceFailureWriter(func([]byte) (int, error) { latched = true; return 0, output }), io.Discard, nil)
	if !errors.Is(err, output) || !errors.Is(err, process) || !owner.closed {
		t.Fatal("an original owner failure was dropped", err)
	}
}

func TestSourceInputLossAfterHealthyObservationRetainsInputCategory(t *testing.T) {
	// This fixture isolates first-cause projection, not successful State or
	// native process authority. The third observation proves initial readiness
	// was consumed before cancellation; its held late result is deliberately nil.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var checks atomic.Int32
	var lost atomic.Bool
	entered, release := make(chan struct{}), make(chan struct{})
	check := func() error {
		if checks.Add(1) == 3 {
			close(entered)
			<-release
			return nil
		}
		if lost.Load() {
			return errSourceInput
		}
		return nil
	}
	owner := &failedSourceLifetime{}
	joined := make(chan error, 1)
	go func() { joined <- superviseSource(ctx, owner, check, func() {}, io.Discard, io.Discard, nil) }()
	<-entered
	lost.Store(true)
	cancel()
	close(release)
	err := <-joined
	var terminal *sourceTerminal
	if !errors.Is(err, errSourceInput) || !errors.As(err, &terminal) || terminal.firstOwner != "input" || !owner.closed {
		t.Fatal("last healthy process observation replaced original input refusal", err)
	}
}
