package process

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/trace"
	"strings"
	"testing"
	"time"
)

func privateSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ardents-debug-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "process.sock")
}
func clientFor(path string) *http.Client {
	return &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}}}
}
func TestLiveRuntimeAndProfileThenCleanup(t *testing.T) {
	socket := privateSocket(t)
	var responseErr error
	err := Run(context.Background(), socket, func(context.Context) error {
		client := clientFor(socket)
		defer client.CloseIdleConnections()
		for _, route := range []string{"/runtime", "/profile/goroutine", "/profile/heap"} {
			r, err := client.Get("http://diagnostic" + route)
			if err != nil {
				return err
			}
			body, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				return err
			}
			if r.StatusCode != 200 || len(body) == 0 {
				return errors.New("missing live profile")
			}
			if route == "/runtime" {
				var fields map[string]any
				if err := json.Unmarshal(body, &fields); err != nil {
					return err
				}
				if _, ok := fields["goroutines"]; !ok {
					return errors.New("runtime observation missing")
				}
			}
		}
		responseErr = errors.New("retained work error")
		return responseErr
	})
	if !errors.Is(err, responseErr) {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket survived close: %v", err)
	}
}
func TestUnsafeDirectoryAndExistingSocketRefuseBeforeWork(t *testing.T) {
	socket := privateSocket(t)
	if err := os.Chmod(filepath.Dir(socket), 0755); err != nil {
		t.Fatal(err)
	}
	called := false
	work := func(context.Context) error { called = true; return nil }
	if err := Run(context.Background(), socket, work); err == nil || called {
		t.Fatal("unsafe directory admitted")
	}
	os.Chmod(filepath.Dir(socket), 0700)
	if err := os.WriteFile(socket, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), socket, work); err == nil || called {
		t.Fatal("existing socket admitted")
	}
	body, err := os.ReadFile(socket)
	if err != nil || string(body) != "keep" {
		t.Fatal("existing file modified")
	}
}
func TestClosePreservesReplacementSocket(t *testing.T) {
	socket := privateSocket(t)
	err := Run(context.Background(), socket, func(context.Context) error {
		if err := os.Remove(socket); err != nil {
			return err
		}
		other, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
		if err != nil {
			return err
		}
		other.SetUnlinkOnClose(false)
		defer other.Close()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(socket); err != nil {
		t.Fatalf("replacement removed: %v", err)
	}
}
func TestTimedProfileRefusesInvalidBound(t *testing.T) {
	socket := privateSocket(t)
	err := Run(context.Background(), socket, func(context.Context) error {
		client := clientFor(socket)
		defer client.CloseIdleConnections()
		for _, path := range []string{"/profile/cpu?seconds=0", "/trace?seconds=31", "/profile/not-real"} {
			r, err := client.Get("http://diagnostic" + path)
			if err != nil {
				return err
			}
			r.Body.Close()
			if r.StatusCode == 200 {
				return errors.New("invalid profile admitted")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestCancellationJoinsActiveTrace(t *testing.T) {
	socket := privateSocket(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, socket, func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() })
	}()
	<-started
	client := clientFor(socket)
	defer client.CloseIdleConnections()
	capture := make(chan error, 1)
	go func() {
		r, err := client.Get("http://diagnostic/trace?seconds=30")
		if err == nil {
			_, err = io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}
		capture <- err
	}()
	deadline := time.Now().Add(time.Second)
	for !trace.IsEnabled() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !trace.IsEnabled() {
		t.Fatal("capture never became active")
	}

	busy, err := client.Get("http://diagnostic/profile/heap")
	if err != nil {
		t.Fatal(err)
	}
	busy.Body.Close()
	if busy.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("concurrent profile status %d", busy.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run cancellation: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("active trace prevented shutdown")
	}
	select {
	case <-capture:
	case <-time.After(4 * time.Second):
		t.Fatal("capture not joined")
	}
}
func TestLimitedProfileWriterReportsOverflow(t *testing.T) {
	writer := &limitedWriter{output: io.Discard, remaining: 2}
	if _, err := writer.Write([]byte(strings.Repeat("x", 3))); err == nil {
		t.Fatal("overflow lost")
	}
}

func TestPrivateListenerRefusesExcessClientAndRecoversSlot(t *testing.T) {
	socket := privateSocket(t)
	raw, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	listener := privateListener{Listener: raw, slots: make(chan struct{}, 1)}
	first, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	accepted, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer accepted.Close()
	done := make(chan net.Conn, 1)
	go func() { conn, _ := listener.Accept(); done <- conn }()
	excess, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	excess.SetReadDeadline(time.Now().Add(time.Second))
	var body [1]byte
	if _, err := excess.Read(body[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("excess client was not refused: %v", err)
	}
	excess.Close()
	accepted.Close()
	next, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	select {
	case conn := <-done:
		if conn == nil {
			t.Fatal("listener stopped")
		}
		conn.Close()
	case <-time.After(time.Second):
		t.Fatal("slot did not recover")
	}
}

func TestProfileUnlinkedBeforeSensitiveWriteAndRemovalFailureRetained(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("profile cleanup denial requires an unprivileged Linux process")
	}
	parent := filepath.Dir(privateSocket(t))
	var retained error
	file, err := openProfileFile(parent, func(err error) { retained = err })
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := os.Lstat(file.Name()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("profile remains linked before write")
	}
	if _, err := file.Write([]byte("sensitive")); err != nil {
		t.Fatal(err)
	}
	denied, err := os.CreateTemp(parent, ".profile-")
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Close()
	if err := os.Chmod(parent, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0700) })
	failure := unlinkProfileFile(denied, func(err error) { retained = err })
	if !errors.Is(failure, os.ErrPermission) {
		t.Fatalf("unlink denial was not observed: %v", failure)
	}
	// This is an empty file; failed initial unlink never admits sensitive writes.
	if info, err := denied.Stat(); err != nil || info.Size() != 0 {
		t.Fatal("failed unlink admitted profile bytes")
	}
	if !errors.Is(retained, os.ErrPermission) {
		t.Fatal("cleanup denial was not retained")
	}
}

type refusalCloseFailureListener struct {
	net.Listener
	failure error
}

func (listener refusalCloseFailureListener) Accept() (net.Conn, error) {
	conn, err := listener.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return refusalCloseFailureConnection{Conn: conn, failure: listener.failure}, nil
}

type refusalCloseFailureConnection struct {
	net.Conn
	failure error
}

func (conn refusalCloseFailureConnection) Close() error {
	return errors.Join(conn.Conn.Close(), conn.failure)
}

func TestPrivateListenerRetainsRefusalCleanupFailure(t *testing.T) {
	socket := privateSocket(t)
	raw, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	failure := errors.New("refused connection cleanup failed")
	retained := make(chan error, 1)
	slots := make(chan struct{}, 1)
	slots <- struct{}{}
	listener := privateListener{Listener: refusalCloseFailureListener{Listener: raw, failure: failure}, slots: slots,
		retainCleanup: func(err error) { retained <- err }}
	done := make(chan error, 1)
	go func() { _, err := listener.Accept(); done <- err }()
	peer, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var body [1]byte
	if _, err := peer.Read(body[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("excess client not refused: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("listener did not join: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("listener did not join")
	}
	select {
	case err := <-retained:
		if !errors.Is(err, failure) {
			t.Fatalf("refusal cleanup failure lost: %v", err)
		}
	default:
		t.Fatal("refusal cleanup failure not retained before listener joined")
	}
	if len(slots) != 1 {
		t.Fatal("refused connection changed an active slot")
	}
}

func TestPrivateConnectionRetainsUnexpectedCloseOnce(t *testing.T) {
	unexpected := errors.New("accepted connection cleanup failed")
	for _, scenario := range []struct {
		name     string
		failure  error
		retained int
	}{
		{"unexpected", unexpected, 1}, {"already-closed", net.ErrClosed, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			socket := privateSocket(t)
			raw, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			defer raw.Close()
			calls := 0
			var retained error
			slots := make(chan struct{}, 1)
			listener := privateListener{Listener: refusalCloseFailureListener{Listener: raw, failure: scenario.failure}, slots: slots,
				retainCleanup: func(err error) { calls++; retained = err }}
			peer, err := net.Dial("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			accepted, err := listener.Accept()
			if err != nil {
				t.Fatal(err)
			}
			results := make(chan error, 2)
			start := make(chan struct{})
			for range 2 {
				go func() { <-start; results <- accepted.Close() }()
			}
			close(start)
			for range 2 {
				select {
				case err := <-results:
					if !errors.Is(err, scenario.failure) {
						t.Fatalf("close result lost: %v", err)
					}
				case <-time.After(time.Second):
					t.Fatal("concurrent close did not join")
				}
			}
			if calls != scenario.retained {
				t.Fatalf("cleanup observations = %d, want %d", calls, scenario.retained)
			}
			if scenario.retained != 0 && !errors.Is(retained, unexpected) {
				t.Fatalf("unexpected cleanup lost: %v", retained)
			}
			if len(slots) != 0 {
				t.Fatal("accepted slot not released")
			}
			if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			var body [1]byte
			if _, err := peer.Read(body[:]); !errors.Is(err, io.EOF) {
				t.Fatalf("accepted connection not closed: %v", err)
			}
		})
	}
}

func TestLiveConnectionSnapshotIsReaderOnlyAndJoins(t *testing.T) {
	socket := privateSocket(t)
	err := Run(t.Context(), socket, func(ctx context.Context) error {
		client := clientFor(socket)
		defer client.CloseIdleConnections()
		read := func() (connectionSnapshot, error) {
			var snapshot connectionSnapshot
			response, err := client.Get("http://diagnostic/connection")
			if err != nil {
				return snapshot, err
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" {
				return snapshot, errors.New("snapshot response unavailable")
			}
			err = json.NewDecoder(response.Body).Decode(&snapshot)
			return snapshot, err
		}
		trace := ReaderTrace(ctx)
		ObserveConnection(ctx, ServiceAuthentication)(nil)
		snapshot, err := read()
		if err != nil {
			return err
		}
		if snapshot.State != "active" || len(snapshot.Records) != 0 {
			return errors.New("foreign process operation entered capture")
		}
		operation := trace.Bind(ctx)
		ended := ObserveConnection(operation, DocumentExchange)
		ended(nil)
		snapshot, err = read()
		if err != nil {
			return err
		}
		if snapshot.State != "active" || snapshot.Outcome != "" {
			return errors.New("Application exchange became joined")
		}
		trace.Finish(context.Canceled)
		snapshot, err = read()
		if err != nil {
			return err
		}
		if snapshot.State != "joined" || snapshot.Outcome != "canceled" || len(snapshot.Records) != 2 {
			return errors.New("joined snapshot missing")
		}
		if ReaderTrace(ctx) != nil {
			return errors.New("second Reader operation admitted")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
