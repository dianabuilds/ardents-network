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
