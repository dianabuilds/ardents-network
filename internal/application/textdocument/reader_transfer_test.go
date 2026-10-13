//go:build linux

package textdocument_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

// These transfer fixtures release completion immediately. They exercise the
// same retained reader contract used by the real Execution consumer.
func awaitTransferredReader(ctx context.Context, worker io.ReadWriteCloser, incoming <-chan connection.Stream) ([]byte, error) {
	ready := make(chan []byte, 1)
	release := make(chan struct{})
	close(release)
	return textdocument.AwaitRetainedWorkerConnection(ctx, worker, incoming, ready, release)
}

// The outcome fixture isolates Application lifetime only. It supplies no
// authenticated Connection or qualified Execution authority.
func TestRetainedReaderResultKeepsOriginalStreamUntilRelease(t *testing.T) {
	worker, done := startTextWorker(t, textdocument.ReaderWorker, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	stream := &responseStream{input: bytes.NewReader([]byte("ARDTXT01\x00\x00\x00\x00\x02ok")), done: make(chan connection.Outcome, 1)}
	stream.done <- connection.Outcome{Class: connection.CleanClose}
	close(stream.done)
	incoming := make(chan connection.Stream, 1)
	incoming <- stream
	ready, release := make(chan []byte, 1), make(chan struct{})
	completed := make(chan error, 1)
	go func() {
		_, err := textdocument.AwaitRetainedWorkerConnection(ctx, worker, incoming, ready, release)
		completed <- err
	}()
	waited := false
	defer func() {
		cancel()
		if !waited {
			<-completed
		}
		<-done
	}()
	select {
	case body := <-ready:
		if string(body) != "ok" || stream.closed {
			t.Fatal("private Reader result prematurely closed its original stream")
		}
	case <-ctx.Done():
		t.Fatal("retained result was not prepared", ctx.Err())
	}
	select {
	case <-done:
		t.Fatal("original worker joined before its RESULT release")
	default:
	}
	close(release)
	err := <-completed
	waited = true
	if err != nil || !stream.closed || !stream.inputClosed || stream.request.Len() != 512 {
		t.Fatal("release lost original stream completion", err)
	}
}

// These fixtures isolate Text's sole reader and transfer, not Service authority
// or qualified Execution. Genuine installed/Connection evidence is separate.
func TestInitializedReaderWaitsForSingleStreamWithSameProtocolReader(t *testing.T) {
	worker, done := startTextWorker(t, textdocument.ReaderWorker, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	stream := &responseStream{input: bytes.NewReader([]byte("ARDTXT01\x00\x00\x00\x00\x02ok")), done: make(chan connection.Outcome, 1)}
	stream.done <- connection.Outcome{Class: connection.CleanClose}
	close(stream.done)
	incoming := make(chan connection.Stream)
	result := make(chan struct {
		body []byte
		err  error
	}, 1)
	go func() {
		body, err := awaitTransferredReader(ctx, worker, incoming)
		result <- struct {
			body []byte
			err  error
		}{body, err}
	}()
	select {
	case incoming <- stream:
	case <-ctx.Done():
		t.Fatal("initialized reader failed to accept its one stream")
	}
	completed := <-result
	if completed.err != nil || string(completed.body) != "ok" || !stream.closed || !stream.inputClosed || stream.request.Len() != 512 {
		t.Fatal("single-reader handoff lost request/result or join", completed.err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("original Text worker did not join")
	}
}

func TestWaitingReaderJoinsWorkerOnMissingTransferAndOriginalCancellation(t *testing.T) {
	for _, closedInput := range []bool{false, true} {
		worker, done := startTextWorker(t, textdocument.ReaderWorker, nil)
		ctx, cancel := context.WithCancel(t.Context())
		incoming := make(chan connection.Stream)
		if closedInput {
			close(incoming)
		} else {
			cancel()
		}
		body, err := awaitTransferredReader(ctx, worker, incoming)
		cancel()
		if body != nil || err == nil {
			t.Fatal("missing original transfer produced text")
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("waiting protocol reader retained its worker after refusal")
		}
	}
}

func TestWaitingReaderRefusesUnsolicitedWorkerOutputBeforeStream(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	written := make(chan error, 1)
	go func() { _, err := remote.Write([]byte{0xff, 0, 0, 0, 1, 0, 0, 0, 0}); written <- err }()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	body, err := awaitTransferredReader(ctx, local, make(chan connection.Stream))
	if body != nil || err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("unsolicited frame did not synchronously refuse before transfer", err)
	}
	if err := <-written; err != nil && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}
