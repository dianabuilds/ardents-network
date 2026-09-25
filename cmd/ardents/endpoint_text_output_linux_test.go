//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/timeline"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

func TestHeadlessTextRefreshFailureEventExposesOnlyFixedCategory(t *testing.T) {
	output := &headlessTextBufferedOutput{}
	at := time.Date(2026, time.September, 25, 9, 30, 0, 0, time.UTC)
	event := endpointapi.TextParticipantEvent{At: at, Kind: "publication-refresh-failed", NetworkID: [32]byte{1}, Failure: "rotation"}
	if err := writeHeadlessTextEvent(t.Context(), output, event); err != nil {
		t.Fatal(err)
	}
	var observed struct {
		Schema  string    `json:"schema"`
		Kind    string    `json:"kind"`
		At      time.Time `json:"at"`
		Failure string    `json:"failure"`
	}
	if err := json.Unmarshal(output.Bytes(), &observed); err != nil || observed.Schema != "ardents-headless-runtime-event-v1" || observed.Kind != "headless-runtime-publication-refresh-failed" || !observed.At.Equal(at) || observed.Failure != "rotation" {
		t.Fatalf("refresh event = %#v / %v", observed, err)
	}
}

func TestHeadlessTextFatalEventKeepsWrappedFailureOutOfTimeline(t *testing.T) {
	at := time.Date(2026, time.September, 25, 9, 30, 0, 0, time.UTC)
	private := errors.New("private worker path and peer address")
	for _, test := range []struct {
		name, phase string
		ready       bool
	}{{"startup", "startup", false}, {"running", "running", true}} {
		t.Run(test.name, func(t *testing.T) {
			output := &headlessTextBufferedOutput{}
			if err := reportHeadlessTextFailure(t.Context(), output, [32]byte{1}, func() time.Time { return at }, test.ready, false, private); !errors.Is(err, private) {
				t.Fatalf("original failure lost: %v", err)
			}
			if bytes.Contains(output.Bytes(), []byte(private.Error())) {
				t.Fatalf("private failure entered event: %q", output.Bytes())
			}
			var projected bytes.Buffer
			err := timeline.Project(t.Context(), io.NopCloser(bytes.NewReader(output.Bytes())), &projected)
			if err != nil || !bytes.Contains(projected.Bytes(), []byte("headless-runtime-failed\t-\t\""+test.phase+"\"")) {
				t.Fatalf("fatal event timeline = %q, err=%v", projected.String(), err)
			}
		})
	}
	for _, test := range []struct {
		name         string
		ctx          context.Context
		outputFailed bool
	}{{"event output failed", t.Context(), true}, {"canceled", canceledHeadlessTextContext(), false}} {
		t.Run(test.name, func(t *testing.T) {
			output := &headlessTextBufferedOutput{}
			if err := reportHeadlessTextFailure(test.ctx, output, [32]byte{1}, func() time.Time { return at }, false, test.outputFailed, private); !errors.Is(err, private) || output.Len() != 0 {
				t.Fatalf("unexpected failure event: output=%q err=%v", output.Bytes(), err)
			}
		})
	}
}

func canceledHeadlessTextContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

type headlessTextBufferedOutput struct{ bytes.Buffer }

func (output *headlessTextBufferedOutput) SetWriteDeadline(time.Time) error { return nil }

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
