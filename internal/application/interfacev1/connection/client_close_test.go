package connection

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestClientCloseInterruptsBlockedWrite(t *testing.T) {
	path := shortClientSocketPath(t)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	peerReady := startBackpressuredPeer(t, listener)
	application, err := Dial(context.Background(), path, "ardents-target:v1:test")
	if err != nil {
		t.Fatal(err)
	}
	cleanupClient(t, application)

	payload := make([]byte, 8<<20)
	writeResult := make(chan writeCallResult, 1)
	go func() {
		written, writeErr := application.Write(payload)
		writeResult <- writeCallResult{written: written, err: writeErr}
	}()
	peer := awaitPeerSetup(t, peerReady)
	cleanupUnixConnection(t, peer)
	assertWriteRemainsBlocked(t, writeResult)

	closeResult := make(chan error, 1)
	go func() { closeResult <- application.Close() }()
	select {
	case closeErr := <-closeResult:
		if closeErr != nil {
			t.Fatalf("Close returned %v", closeErr)
		}
	case <-time.After(time.Second):
		_ = peer.Close()
		<-closeResult
		t.Fatal("Close waited for the blocked Write")
	}

	written := <-writeResult
	if written.err == nil || written.written >= len(payload) {
		t.Fatalf("blocked Write returned %d, %v", written.written, written.err)
	}
	outcome, open := <-application.Done()
	if !open || outcome.Class != LocalCancellation {
		t.Fatalf("Done returned %+v, open=%v", outcome, open)
	}
	if _, open := <-application.Done(); open {
		t.Fatal("Done published more than one terminal outcome")
	}
	if err := application.Close(); err != nil {
		t.Fatalf("repeated Close returned %v", err)
	}
}

func TestClientContextCancellationInterruptsBlockedWrite(t *testing.T) {
	path := shortClientSocketPath(t)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	peerReady := startBackpressuredPeer(t, listener)
	ctx, cancel := context.WithCancel(context.Background())
	application, err := Dial(ctx, path, "ardents-target:v1:test")
	if err != nil {
		t.Fatal(err)
	}
	cleanupClient(t, application)

	payload := make([]byte, 8<<20)
	writeResult := make(chan writeCallResult, 1)
	go func() {
		written, writeErr := application.Write(payload)
		writeResult <- writeCallResult{written: written, err: writeErr}
	}()
	peer := awaitPeerSetup(t, peerReady)
	cleanupUnixConnection(t, peer)
	assertWriteRemainsBlocked(t, writeResult)
	cancel()
	select {
	case written := <-writeResult:
		if written.err == nil || written.written >= len(payload) {
			t.Fatalf("canceled Write returned %d, %v", written.written, written.err)
		}
	case <-time.After(time.Second):
		_ = peer.Close()
		t.Fatal("context cancellation did not interrupt the blocked Write")
	}
	closeResult := make(chan error, 1)
	go func() { closeResult <- application.Close() }()
	select {
	case err := <-closeResult:
		if err != nil {
			t.Fatalf("Close after cancellation returned %v", err)
		}
	case <-time.After(time.Second):
		_ = peer.Close()
		t.Fatal("context cancellation did not join Client cleanup")
	}
	outcome, open := <-application.Done()
	if !open || outcome.Class != LocalCancellation {
		t.Fatalf("cancellation outcome = %+v, open=%v", outcome, open)
	}
	if written, err := application.Write([]byte("late")); written != 0 || !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Write after cancellation = %d, %v", written, err)
	}
	if err := application.CloseInput(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("CloseInput after cancellation = %v", err)
	}
}

func TestClientConcurrentCloseInterruptsWriteAndCloseInput(t *testing.T) {
	path := shortClientSocketPath(t)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	peerReady := startBackpressuredPeer(t, listener)
	application, err := Dial(context.Background(), path, "ardents-target:v1:test")
	if err != nil {
		t.Fatal(err)
	}
	cleanupClient(t, application)

	writeResult := make(chan writeCallResult, 1)
	go func() {
		written, writeErr := application.Write(make([]byte, 8<<20))
		writeResult <- writeCallResult{written: written, err: writeErr}
	}()
	peer := awaitPeerSetup(t, peerReady)
	cleanupUnixConnection(t, peer)
	assertWriteRemainsBlocked(t, writeResult)

	closeInputStarted := make(chan struct{})
	closeInputResult := make(chan error, 1)
	go func() {
		close(closeInputStarted)
		closeInputResult <- application.CloseInput()
	}()
	<-closeInputStarted
	select {
	case err := <-closeInputResult:
		t.Fatalf("CloseInput completed while Write remained blocked: %v", err)
	default:
	}

	const closeCalls = 8
	closeResults := make(chan error, closeCalls)
	for range closeCalls {
		go func() { closeResults <- application.Close() }()
	}
	deadline := time.After(time.Second)
	for range closeCalls {
		select {
		case closeErr := <-closeResults:
			if closeErr != nil {
				t.Fatalf("concurrent Close returned %v", closeErr)
			}
		case <-deadline:
			_ = peer.Close()
			t.Fatal("concurrent Close did not join blocked output operations")
		}
	}
	if written := <-writeResult; written.err == nil {
		t.Fatalf("blocked Write returned %d without an error", written.written)
	}
	if err := <-closeInputResult; !errors.Is(err, net.ErrClosed) {
		t.Fatalf("blocked CloseInput = %v", err)
	}
	outcome, open := <-application.Done()
	if !open || outcome.Class != LocalCancellation {
		t.Fatalf("Done returned %+v, open=%v", outcome, open)
	}
	if _, open := <-application.Done(); open {
		t.Fatal("concurrent abort published more than one outcome")
	}
}

func TestClientVerifiedRemoteOutcomePrecedesLaterClose(t *testing.T) {
	path := shortClientSocketPath(t)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	peerReady := startConnectedPeer(t, listener)
	application, err := Dial(context.Background(), path, "ardents-target:v1:test")
	if err != nil {
		t.Fatal(err)
	}
	cleanupClient(t, application)
	peer := awaitPeerSetup(t, peerReady)
	cleanupUnixConnection(t, peer)
	want := Outcome{Class: CleanClose, Reason: "remote terminal verified"}
	if err := writeTerminal(peer, want); err != nil {
		t.Fatal(err)
	}
	if got := <-application.Done(); got != want {
		t.Fatalf("Done = %+v, want %+v", got, want)
	}
	if err := application.Close(); err != nil {
		t.Fatal(err)
	}
	if _, open := <-application.Done(); open {
		t.Fatal("later Close replaced the verified remote outcome")
	}
}

func TestClientVerifiedRemoteOutcomeCompletesDuringClose(t *testing.T) {
	path := shortClientSocketPath(t)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	peerReady := startConnectedPeer(t, listener)
	want := Outcome{Class: CleanClose, Reason: "remote terminal verified during Close"}
	application, transport := dialGatedClient(t, path, "ardents-target:v1:test", terminalFrameSize(want))
	cleanupClient(t, application)
	peer := awaitPeerSetup(t, peerReady)
	cleanupUnixConnection(t, peer)

	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(transport.releaseRead) }) })
	if err := writeTerminal(peer, want); err != nil {
		t.Fatal(err)
	}
	select {
	case <-transport.readReady:
	case <-time.After(time.Second):
		t.Fatal("receiver did not obtain the complete remote terminal frame")
	}
	closeResult := make(chan error, 1)
	go func() { closeResult <- application.Close() }()
	select {
	case <-transport.closeStarted:
	case <-time.After(time.Second):
		t.Fatal("Client Close did not close the owned transport")
	}
	releaseOnce.Do(func() { close(transport.releaseRead) })
	if err := <-closeResult; err != nil {
		t.Fatal(err)
	}
	if got := <-application.Done(); got != want {
		t.Fatalf("Done = %+v, want verified remote %+v", got, want)
	}
	if _, open := <-application.Done(); open {
		t.Fatal("Close published a second terminal outcome")
	}
}

func TestClientWriteAndCloseInputPreserveFrameOrder(t *testing.T) {
	path := shortClientSocketPath(t)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	received := make(chan captureResult, 1)
	peerDone := make(chan struct{})
	go acceptCapturingPeer(listener, received, peerDone)
	joinPeer(t, peerDone)
	cleanupUnixListener(t, listener)
	application, err := Dial(context.Background(), path, "ardents-target:v1:test")
	if err != nil {
		t.Fatal(err)
	}
	cleanupClient(t, application)
	payload := bytes.Repeat([]byte("ordered"), maximumFrame)
	start := make(chan struct{})
	writeResult := make(chan writeCallResult, 1)
	closeInputResult := make(chan error, 1)
	go func() {
		<-start
		written, writeErr := application.Write(payload)
		writeResult <- writeCallResult{written: written, err: writeErr}
	}()
	go func() {
		<-start
		closeInputResult <- application.CloseInput()
	}()
	close(start)
	written, closeInputErr := <-writeResult, <-closeInputResult
	if closeInputErr != nil {
		t.Fatalf("CloseInput returned %v", closeInputErr)
	}
	got := awaitCapture(t, received)
	if written.err == nil {
		if written.written != len(payload) || !bytes.Equal(got, payload) {
			t.Fatalf("Write won ordering with written=%d received=%d", written.written, len(got))
		}
	} else if written.written != 0 || !errors.Is(written.err, net.ErrClosed) || len(got) != 0 {
		t.Fatalf("CloseInput won ordering with Write=%d, %v received=%d", written.written, written.err, len(got))
	}
	if err := application.Close(); err != nil {
		t.Fatal(err)
	}
}

type writeCallResult struct {
	written int
	err     error
}

func shortClientSocketPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(os.TempDir(), fmt.Sprintf("aai-close-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove Unix socket: %v", err)
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("Unix socket residue: %v", err)
		}
	})
	return path
}

func awaitPeerSetup(t *testing.T, ready <-chan peerSetupResult) *net.UnixConn {
	t.Helper()
	select {
	case result := <-ready:
		if result.err != nil || result.connection == nil {
			t.Fatalf("non-reading peer setup: %v", result.err)
		}
		return result.connection
	case <-time.After(time.Second):
		t.Fatal("non-reading peer setup did not complete")
		return nil
	}
}

func awaitCapture(t *testing.T, received <-chan captureResult) []byte {
	t.Helper()
	select {
	case result := <-received:
		if result.err != nil {
			t.Fatalf("capturing peer: %v", result.err)
		}
		return result.data
	case <-time.After(time.Second):
		t.Fatal("capturing peer did not complete")
		return nil
	}
}

func cleanupUnixConnection(t *testing.T, connection *net.UnixConn) {
	t.Helper()
	t.Cleanup(func() {
		if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close Unix peer: %v", err)
		}
	})
}

func cleanupUnixListener(t *testing.T, listener *net.UnixListener) {
	t.Helper()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close Unix listener: %v", err)
		}
	})
}

func cleanupClient(t *testing.T, application Client) {
	t.Helper()
	t.Cleanup(func() {
		if err := application.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close Application Client: %v", err)
		}
	})
}

func joinPeer(t *testing.T, done <-chan struct{}) {
	t.Helper()
	t.Cleanup(func() {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("Unix peer goroutine did not stop")
		}
	})
}

type peerSetupResult struct {
	connection *net.UnixConn
	err        error
}

type captureResult struct {
	data []byte
	err  error
}

func startBackpressuredPeer(t *testing.T, listener *net.UnixListener) <-chan peerSetupResult {
	t.Helper()
	return startPeer(t, listener, acceptNonReadingPeer)
}

func startConnectedPeer(t *testing.T, listener *net.UnixListener) <-chan peerSetupResult {
	t.Helper()
	return startPeer(t, listener, acceptConnectedPeer)
}

func startPeer(t *testing.T, listener *net.UnixListener, run func(*net.UnixListener, chan<- peerSetupResult)) <-chan peerSetupResult {
	t.Helper()
	ready := make(chan peerSetupResult, 1)
	go run(listener, ready)
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close Unix listener: %v", err)
		}
		for result := range ready {
			if result.err != nil {
				t.Errorf("abandoned Unix peer setup: %v", result.err)
			}
			if result.connection != nil {
				if err := result.connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
					t.Errorf("close abandoned Unix peer: %v", err)
				}
			}
		}
	})
	return ready
}

func assertWriteRemainsBlocked(t *testing.T, result <-chan writeCallResult) {
	t.Helper()
	select {
	case completed := <-result:
		t.Fatalf("Write completed before cancellation: %d, %v", completed.written, completed.err)
	default:
	}
}
