//go:build linux

package node

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestWriteEventBackpressureDeadline(t *testing.T) {
	for _, kind := range []string{"pipe", "socket"} {
		t.Run(kind, func(t *testing.T) {
			output, _ := eventStream(t, kind)
			fillEventStream(t, output)
			ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
			defer cancel()
			n, err := writeEvent(ctx, output, []byte("lifecycle-ready\n"))
			if n != 0 || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("saturated stream: n=%d err=%v", n, err)
			}
		})
	}
}

func TestWriteEventBackpressureCancellation(t *testing.T) {
	output, _ := eventStream(t, "pipe")
	fillEventStream(t, output)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	done := make(chan struct{})
	var n int
	var err error
	go func() {
		defer close(done)
		n, err = writeEvent(ctx, output, []byte("lifecycle-ready\n"))
	}()
	t.Cleanup(func() { cancel(); <-done })
	cancel()
	<-done
	if n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled saturated stream: n=%d err=%v", n, err)
	}
}

func TestWriteEventBackpressureResumesExactEvent(t *testing.T) {
	for _, kind := range []string{"pipe", "socket"} {
		t.Run(kind, func(t *testing.T) {
			output, peer := eventStream(t, kind)
			prefix := fillEventStream(t, output)
			// This is the emitter's maximum line length, below Linux PIPE_BUF.
			want := append(bytes.Repeat([]byte("e"), 2047), '\n')
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			done := make(chan struct{})
			var n int
			var writeErr error
			go func() {
				defer close(done)
				n, writeErr = writeEvent(ctx, output, want)
			}()
			t.Cleanup(func() { cancel(); <-done })
			// Retain saturation across several retry ticks, then release the peer.
			// The separate deadline case proves the real EAGAIN path without a reader.
			timer := time.NewTimer(20 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-done:
				t.Fatalf("writer completed before drain: n=%d err=%v", n, writeErr)
			case <-timer.C:
			}
			readDone := make(chan struct{})
			var got []byte
			var readErr error
			go func() {
				defer close(readDone)
				got, readErr = io.ReadAll(peer)
			}()
			t.Cleanup(func() {
				_ = output.Close()
				_ = peer.Close()
				<-readDone
			})
			<-done
			if err := output.Close(); err != nil {
				t.Fatal(err)
			}
			<-readDone
			if n != len(want) || writeErr != nil || readErr != nil {
				t.Fatalf("resumed event: n=%d write=%v read=%v", n, writeErr, readErr)
			}
			if !bytes.Equal(got, append(prefix, want...)) {
				t.Fatalf("resumed stream differs: got %d bytes, want %d", len(got), len(prefix)+len(want))
			}
		})
	}
}

func TestWriteEventPartialOffsetSurvivesBackpressure(t *testing.T) {
	output, peer := eventStream(t, "socket")
	// A mechanism fixture larger than the socket buffer forces a positive partial
	// write before EAGAIN. EventEmitter separately enforces its 2048-byte bound.
	want := bytes.Repeat([]byte("0123456789abcdef"), 4096)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	n, err := writeEvent(ctx, output, want)
	if n <= 0 || n >= len(want) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("partial saturated write: n=%d err=%v", n, err)
	}
	readDone := make(chan struct{})
	var got []byte
	var readErr error
	go func() {
		defer close(readDone)
		got, readErr = io.ReadAll(peer)
	}()
	t.Cleanup(func() {
		_ = output.Close()
		_ = peer.Close()
		<-readDone
	})
	resume, stop := context.WithTimeout(t.Context(), time.Second)
	defer stop()
	remaining, err := writeEvent(resume, output, want[n:])
	if err != nil || n+remaining != len(want) {
		t.Fatalf("resume partial offset: first=%d remaining=%d err=%v", n, remaining, err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	<-readDone
	if readErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("partial offset: first=%d, received=%d, read=%v", n, len(got), readErr)
	}
}

func TestWriteEventClosedPeerRetainsWriteError(t *testing.T) {
	for _, kind := range []string{"pipe", "socket"} {
		t.Run(kind, func(t *testing.T) {
			output, peer := eventStream(t, kind)
			if err := peer.Close(); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if n, err := writeEvent(ctx, output, []byte("lifecycle-ready\n")); n != 0 || !errors.Is(err, syscall.EPIPE) {
				t.Fatalf("closed peer: n=%d err=%v", n, err)
			}
		})
	}
}

func TestEventEmitterBackpressureAndDeadlineRequirement(t *testing.T) {
	output, _ := eventStream(t, "pipe")
	fillEventStream(t, output)
	emit := EventEmitter(output)
	event := Event{Schema: eventSchema, Kind: "lifecycle", State: "READY"}
	if err := emit(t.Context(), event); err == nil {
		t.Fatal("event without deadline accepted")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := emit(ctx, event); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("saturated emitter: %v", err)
	}
}

func eventStream(t *testing.T, kind string) (*os.File, *os.File) {
	t.Helper()
	var output, peer *os.File
	if kind == "pipe" {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		output, peer = writer, reader
	} else {
		pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
		if err != nil {
			t.Fatal(err)
		}
		output = os.NewFile(uintptr(pair[0]), "event-output")
		peer = os.NewFile(uintptr(pair[1]), "event-peer")
	}
	t.Cleanup(func() { _ = output.Close(); _ = peer.Close() })
	if kind == "socket" {
		if err := syscall.SetsockoptInt(int(output.Fd()), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4096); err != nil {
			t.Fatal(err)
		}
	}
	return output, peer
}

func fillEventStream(t *testing.T, output *os.File) []byte {
	t.Helper()
	fd := int(output.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	chunk := bytes.Repeat([]byte("p"), 2048)
	var prefix []byte
	for len(prefix) <= 1<<20 {
		n, err := syscall.Write(fd, chunk)
		if n > 0 {
			prefix = append(prefix, chunk[:n]...)
		}
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			return prefix
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("event stream did not saturate within fixture bound")
	return nil
}

func TestWriteEventAcceptsSystemdJournalSocket(t *testing.T) {
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	output := os.NewFile(uintptr(pair[0]), "journal-output")
	peer := os.NewFile(uintptr(pair[1]), "journal-peer")
	t.Cleanup(func() {
		_ = output.Close()
		_ = peer.Close()
	})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	want := []byte("lifecycle-ready\n")
	if n, err := writeEvent(ctx, output, want); err != nil || n != len(want) {
		t.Fatalf("write journal event: n=%d err=%v", n, err)
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(peer, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("journal event = %q, want %q", got, want)
	}
}
