//go:build linux

package main

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

func TestHeadlessTextEventCancellationJoinsBlockedPipe(t *testing.T) {
	testHeadlessTextCancellation(t, false)
}

func TestHeadlessTextEventCancellationJoinsJournalSocket(t *testing.T) {
	testHeadlessTextCancellation(t, true)
}

func testHeadlessTextCancellation(t *testing.T, socket bool) {
	reader, inherited, err := os.Pipe()
	if socket && err == nil {
		reader.Close()
		inherited.Close()
		var pair [2]int
		pair, err = syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
		if err == nil {
			reader = os.NewFile(uintptr(pair[0]), "journal-reader")
			inherited = os.NewFile(uintptr(pair[1]), "journal-writer")
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer inherited.Close()
	// Fd intentionally exposes the inherited blocking description before the
	// runtime reopens it. The runtime must not alter this owner's status flags.
	fd := inherited.Fd()
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
	if errno != 0 {
		t.Fatal(errno)
	}
	output, err := openHeadlessTextOutput(inherited)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	after, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
	if errno != 0 || flags != after {
		t.Fatalf("inherited output flags changed: %v", errno)
	}
	// Fill the actual pipe without a consumer, then prove an event cannot
	// complete until its context interrupts the owned pollable descriptor.
	if err := output.SetWriteDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	block := make([]byte, 4096)
	for {
		_, err := output.Write(block)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	entered := make(chan struct{})
	observed := &headlessTextObservedWrite{headlessTextEventOutput: output, entered: entered}
	go func() { done <- writeHeadlessTextEvent(ctx, observed, endpointapi.TextParticipantEvent{Kind: "ready"}) }()
	select {
	case err := <-done:
		t.Fatalf("event unexpectedly completed while pipe was full: %v", err)
	case <-entered:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("event did not reach actual output write")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled event = %v", err)
		}
	case <-time.After(2 * time.Second):
		output.Close()
		<-done
		t.Fatal("event did not join cancellation")
	}
	after, _, errno = syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
	if errno != 0 || flags != after {
		t.Fatalf("cancellation changed inherited output flags: %v", errno)
	}
}

func TestHeadlessTextOutputRejectsRegularFile(t *testing.T) {
	inherited, err := os.CreateTemp(t.TempDir(), "events")
	if err != nil {
		t.Fatal(err)
	}
	defer inherited.Close()
	output, err := openHeadlessTextOutput(inherited)
	if err == nil {
		output.Close()
		t.Fatal("uninterruptible regular output accepted")
	}
	if _, err := inherited.WriteString("still owned"); err != nil {
		t.Fatalf("rejection closed inherited output: %v", err)
	}
}

// The barrier observes entry after context checks and before the real pipe
// write, so cancellation must interrupt that write even if it races entry.
type headlessTextObservedWrite struct {
	headlessTextEventOutput
	entered chan struct{}
}

func (output *headlessTextObservedWrite) Write(value []byte) (int, error) {
	close(output.entered)
	return output.headlessTextEventOutput.Write(value)
}
